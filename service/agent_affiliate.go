package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// affiliateMaxDepth 分销链的上溯上限，与代理树的存储护栏同量级，纯防御性。
const affiliateMaxDepth = model.AgentMaxDepth

// affiliateCut 一次分润里某个分销节点应得的部分。
type affiliateCut struct {
	AgentId int
	Amount  int
}

// splitAffiliateRebate 从某一级代理的差价里切出分销返佣。
//
// 逐级抽佣，和批发链同一套逻辑：推荐人 E 从代理 A 的差价里拿一份，
// 推荐 E 的 E1 再从 E 的那份里拿一份。每一级只跟自己的直接上下级结算，
// 因此 E1 永远不需要知道终端客户是谁。
//
// 返回各分销节点应得的金额，以及扣除返佣后代理自己剩下的金额。
// 三者之和恒等于传入的 margin，对账等式不因分销而破。
func splitAffiliateRebate(agentId, margin, consumerUserId int) ([]affiliateCut, int) {
	if margin <= 0 || consumerUserId <= 0 {
		return nil, margin
	}
	consumer, err := model.GetUserById(consumerUserId, false)
	if err != nil || consumer.InviterId <= 0 {
		return nil, margin
	}

	cuts := make([]affiliateCut, 0, 2)
	remaining := margin
	// pool 是当前这一级分销节点手里的钱，它的上线从中再切一刀。
	pool := 0
	inviterUserId := consumer.InviterId
	var current *model.Agent

	for depth := 0; depth < affiliateMaxDepth && inviterUserId > 0; depth++ {
		node, err := model.GetAgentByOwnerUserId(inviterUserId)
		if err != nil || node.Type != model.AgentTypeAffiliate ||
			node.Status != model.AgentStatusActive || node.RebateRatePercent <= 0 {
			break
		}
		// 分销节点必须与出钱的那一级代理同属一条线，否则就是跨代理拿钱。
		if depth == 0 && node.ParentAgentId != agentId {
			break
		}
		if depth > 0 && current != nil && node.ParentAgentId != current.ParentAgentId {
			break
		}

		source := remaining
		if depth > 0 {
			source = pool
		}
		amount := common.QuotaFromFloat(float64(source) * node.RebateRatePercent / 100)
		if amount <= 0 {
			break
		}
		if depth == 0 {
			remaining -= amount
		} else {
			// 上线的这一份从下线手里出，下线净得相应减少。
			cuts[len(cuts)-1].Amount -= amount
		}
		cuts = append(cuts, affiliateCut{AgentId: node.Id, Amount: amount})
		pool = amount

		current = node
		owner, err := model.GetUserById(node.OwnerUserId, false)
		if err != nil || owner.InviterId <= 0 {
			break
		}
		inviterUserId = owner.InviterId
	}

	// 净额为 0 的层不必写行
	filtered := cuts[:0]
	for _, cut := range cuts {
		if cut.Amount > 0 {
			filtered = append(filtered, cut)
		}
	}
	return filtered, remaining
}

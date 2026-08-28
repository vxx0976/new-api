package service

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var (
	ErrAgentRateInvalid    = errors.New("倍率必须大于 0")
	ErrAgentGroupRequired  = errors.New("分组不能为空")
	ErrAgentSellBelowCost  = errors.New("售价不能低于自己的进货价")
	ErrAgentCostBelowOwn   = errors.New("给下级的进货价不能低于自己的进货价")
	ErrAgentChildNotMine   = errors.New("只能给自己的直接下级设价")
	ErrAgentCostNotSet     = errors.New("尚未配置进货价，请联系上级")
	ErrAgentNotPlatformTop = errors.New("该代理不是平台直属代理")
)

// EffectiveCostRate 返回某代理在某分组下实际生效的进货倍率。
// 自己没配就沿链路继承上级的价（即该级不赚差价）。
func EffectiveCostRate(agent *model.Agent, group string) (float64, bool) {
	if agent == nil || group == "" {
		return 0, false
	}
	nodes, ok := buildAgentCostChain(agent, group)
	if !ok {
		return 0, false
	}
	return nodes[len(nodes)-1].CostRate, true
}

// SetOwnSellRate 代理设定自己对终端用户的售价。定价权属于代理本人，上级不得代设。
func SetOwnSellRate(operatorUserId int, group string, rate float64) error {
	if group == "" {
		return ErrAgentGroupRequired
	}
	if rate <= 0 {
		return ErrAgentRateInvalid
	}
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return err
	}
	if agent.Status != model.AgentStatusActive {
		return ErrAgentInactive
	}
	cost, ok := EffectiveCostRate(agent, group)
	if !ok {
		return ErrAgentCostNotSet
	}
	if rate < cost {
		return fmt.Errorf("%w（当前进货价 %.6f）", ErrAgentSellBelowCost, cost)
	}

	old := ""
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		existing, err := model.GetAgentGroupSellForUpdate(tx, agent.Id, group)
		if err != nil {
			return err
		}
		if existing != nil {
			old = fmt.Sprintf("%.6f", existing.SellRate)
		}
		if err := model.UpsertAgentGroupSell(tx, agent.Id, group, rate); err != nil {
			return err
		}
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: agent.Id, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionSetSell, TargetType: "group", TargetId: group,
			OldValue: old, NewValue: fmt.Sprintf("%.6f", rate),
		})
	})
	if err != nil {
		return err
	}
	InvalidateAgentPricingCache()
	return nil
}

// SetChildCostRate 上级给自己的直接下级设进货价。
//
// 降价立即生效；抬价一律延迟到 CostRaiseDelayHours 之后，并保留原价继续计费，
// 好让下级有时间调整自己的售价，不至于毫无预警地开始倒贴。
func SetChildCostRate(operatorUserId, childAgentId int, group string, rate float64) error {
	parent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return err
	}
	if parent.Status != model.AgentStatusActive {
		return ErrAgentInactive
	}
	child, err := model.GetAgentById(childAgentId)
	if err != nil || child.ParentAgentId != parent.Id {
		return ErrAgentChildNotMine
	}
	parentCost, ok := EffectiveCostRate(parent, group)
	if !ok {
		return ErrAgentCostNotSet
	}
	if rate < parentCost {
		return fmt.Errorf("%w（当前进货价 %.6f）", ErrAgentCostBelowOwn, parentCost)
	}
	return writeAgentCostRate(operatorUserId, child, group, rate, true)
}

// ListChildCostRates 上级查看自己给某个直接下级设的进货价。
// 只回进货价，不回下级的售价——那是下级的利润率，上级无权知晓。
func ListChildCostRates(operatorUserId, childAgentId int) ([]*model.AgentGroupCost, error) {
	parent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return nil, err
	}
	child, err := model.GetAgentById(childAgentId)
	if err != nil || child.ParentAgentId != parent.Id {
		return nil, ErrAgentChildNotMine
	}
	costs, err := model.GetAgentGroupCosts(child.Id)
	if err != nil {
		return nil, err
	}
	rows := make([]*model.AgentGroupCost, 0, len(costs))
	for _, cost := range costs {
		rows = append(rows, cost)
	}
	return rows, nil
}

// SetPlatformAgentCostRate 管理员给平台直属代理设进货价。
func SetPlatformAgentCostRate(operatorUserId, agentId int, group string, rate float64) error {
	agent, err := model.GetAgentById(agentId)
	if err != nil {
		return ErrAgentNotFound
	}
	if agent.ParentAgentId != 0 {
		return ErrAgentNotPlatformTop
	}
	return writeAgentCostRate(operatorUserId, agent, group, rate, true)
}

func writeAgentCostRate(operatorUserId int, target *model.Agent, group string, rate float64, delayRaise bool) error {
	if group == "" {
		return ErrAgentGroupRequired
	}
	if rate <= 0 {
		return ErrAgentRateInvalid
	}

	var (
		old            string
		scheduledAt    int64
		appliedNow     = true
		delaySeconds   = int64(operation_setting.GetAgentCostRaiseDelayHours()) * 3600
		effectiveAtVal int64
	)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		existing, err := model.GetAgentGroupCostForUpdate(tx, target.Id, group)
		if err != nil {
			return err
		}
		if existing != nil {
			old = fmt.Sprintf("%.6f", existing.CostRate)
			if delayRaise && rate > existing.CostRate {
				appliedNow = false
				effectiveAtVal = common.GetTimestamp() + delaySeconds
				scheduledAt = effectiveAtVal
				return model.SetAgentGroupCostPending(tx, target.Id, group, rate, effectiveAtVal)
			}
		}
		return model.UpsertAgentGroupCost(tx, target.Id, group, rate)
	})
	if err != nil {
		return err
	}

	action := model.AgentAuditActionSetChildCost
	newValue := fmt.Sprintf("%.6f", rate)
	if !appliedNow {
		newValue = fmt.Sprintf("%.6f (生效于 %d)", rate, scheduledAt)
	}
	_ = model.RecordAgentAudit(nil, &model.AgentAuditLog{
		AgentId: target.Id, OperatorUserId: operatorUserId,
		Action: action, TargetType: "group", TargetId: group,
		OldValue: old, NewValue: newValue,
	})
	if appliedNow {
		InvalidateAgentPricingCache()
	}
	return nil
}

// ApplyDueAgentCostRates 让到期的抬价生效，由定时任务驱动。
//
// 生效后若下级的售价低于新进货价（延迟窗口内没调价），把售价顶到成本线：
// 三种选择里——让下级持续倒贴、直接断掉下级的终端客户、把售价顶到成本——
// 只有最后一种既不让人亏钱也不断服，代价是终端价随上游涨价而上浮，属正常商业传导。
// 每一次自动上调都写审计并打日志，不静默发生。
func ApplyDueAgentCostRates() (int, error) {
	due, err := model.ListDueAgentGroupCosts(common.GetTimestamp())
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, row := range due {
		newRate := row.PendingRate
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			if err := model.ApplyAgentGroupCostPending(tx, row.Id, newRate); err != nil {
				return err
			}
			sell, err := model.GetAgentGroupSellForUpdate(tx, row.AgentId, row.Group)
			if err != nil {
				return err
			}
			if sell == nil || sell.SellRate >= newRate {
				return nil
			}
			if err := model.UpsertAgentGroupSell(tx, row.AgentId, row.Group, newRate); err != nil {
				return err
			}
			logger.LogWarn(nil, fmt.Sprintf(
				"代理 %d 分组 %s 售价 %.6f 低于新进货价 %.6f，已自动上调至成本线",
				row.AgentId, row.Group, sell.SellRate, newRate))
			return model.RecordAgentAudit(tx, &model.AgentAuditLog{
				AgentId: row.AgentId, Action: model.AgentAuditActionSetSell,
				TargetType: "group", TargetId: row.Group,
				OldValue: fmt.Sprintf("%.6f", sell.SellRate),
				NewValue: fmt.Sprintf("%.6f", newRate),
				Note:     "进货价上调导致倒挂，售价自动顶到成本线",
			})
		})
		if err != nil {
			logger.LogError(nil, fmt.Sprintf("应用代理调价失败 id=%d: %v", row.Id, err))
			continue
		}
		applied++
	}
	if applied > 0 {
		InvalidateAgentPricingCache()
	}
	return applied, nil
}

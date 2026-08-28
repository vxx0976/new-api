package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	agentSettleTickInterval = 1 * time.Minute
	agentSettleBatchSize    = 500
)

var (
	agentSettleOnce    sync.Once
	agentSettleRunning atomic.Bool
)

// RecordAgentConsumeEarnings 按链路逐级计提一笔消费的分润。
//
// 基数用实扣额度反推：终端实付 = base × PaidRate，故第 i 级应得 = quota × margin_i / PaidRate。
// 平台那一份不写行，靠「实扣额度减去各级分润」隐式留存——整数截断的残差因此自动归平台，
// 对账等式恒不超发。
func RecordAgentConsumeEarnings(relayInfo *relaycommon.RelayInfo, quota int) {
	if relayInfo == nil || quota <= 0 {
		return
	}
	chain := relayInfo.AgentPricing
	if chain == nil || !chain.Applies || chain.PaidRate <= 0 {
		return
	}

	windowStart := currentAgentEarningsWindow()
	for _, margin := range chain.LevelMargins() {
		if margin.CostRate <= 0 {
			// 差价为 0 的层直接跳过：写一条金额为 0 的行没有意义，
			// 还会让「金额必须为正」这类约束在计费路径上炸开、回滚整笔计费。
			continue
		}
		amount := common.QuotaFromFloat(float64(quota) * margin.CostRate / chain.PaidRate)
		if amount <= 0 {
			continue
		}
		// 分销返佣从这一级代理的差价里切走，切完剩下的才是代理自己的，
		// 因此「代理净得 + 各分销净得」恒等于原差价，对账等式不因分销而破。
		cuts, agentShare := splitAffiliateRebate(margin.AgentId, amount, relayInfo.UserId)
		for _, cut := range cuts {
			accumulateAgentEarning(cut.AgentId, relayInfo.UserId, cut.Amount, windowStart,
				model.AgentEarningSourceAffiliateRebate, "aff")
		}
		accumulateAgentEarning(margin.AgentId, relayInfo.UserId, agentShare, windowStart,
			model.AgentEarningSourceTierMarkup, "tier")
	}
}

func accumulateAgentEarning(agentId, fromUserId, amount int, windowStart int64, source, keyPrefix string) {
	if amount <= 0 {
		return
	}
	row := &model.AgentEarningsOutbox{
		AgentId:        agentId,
		FromUserId:     fromUserId,
		Amount:         amount,
		Source:         source,
		RefType:        "consume_window",
		RefId:          fmt.Sprintf("%d:%d", fromUserId, windowStart),
		IdempotencyKey: fmt.Sprintf("%s:%d:%d:%d", keyPrefix, agentId, fromUserId, windowStart),
	}
	if err := model.AccumulateAgentEarnings(row); err != nil {
		logger.LogError(nil, fmt.Sprintf("代理分润计提失败 agent=%d: %v", agentId, err))
	}
}

func currentAgentEarningsWindow() int64 {
	windowSeconds := int64(operation_setting.GetAgentEarningsWindowHours()) * 3600
	now := common.GetTimestamp()
	return now - now%windowSeconds
}

// StartAgentEarningsSettleTask 周期性把聚合分润入账到代理的分润钱包。
func StartAgentEarningsSettleTask() {
	agentSettleOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(),
				fmt.Sprintf("agent earnings settle task started: tick=%s", agentSettleTickInterval))
			ticker := time.NewTicker(agentSettleTickInterval)
			defer ticker.Stop()
			SettleAgentEarningsOnce()
			for range ticker.C {
				SettleAgentEarningsOnce()
			}
		})
	})
}

// SettleAgentEarningsOnce 跑一轮结算，返回入账的额度与行数。
func SettleAgentEarningsOnce() (int, int) {
	if !agentSettleRunning.CompareAndSwap(false, true) {
		return 0, 0
	}
	defer agentSettleRunning.Store(false)

	rows, err := model.ListUncreditedAgentEarnings(agentSettleBatchSize)
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("读取待入账分润失败: %v", err))
		return 0, 0
	}

	// 顺带把到期的调价落地，两件事都是低频的定时维护，共用一个 tick。
	if applied, err := ApplyDueAgentCostRates(); err != nil {
		logger.LogError(nil, fmt.Sprintf("应用到期调价失败: %v", err))
	} else if applied > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("已生效代理调价 %d 条", applied))
	}

	owners := map[int]int{}
	totalQuota, totalRows := 0, 0
	for _, row := range rows {
		ownerUserId, ok := owners[row.AgentId]
		if !ok {
			agent, err := model.GetAgentById(row.AgentId)
			if err != nil {
				logger.LogError(nil, fmt.Sprintf("分润入账找不到代理 agent=%d: %v", row.AgentId, err))
				continue
			}
			ownerUserId = agent.OwnerUserId
			owners[row.AgentId] = ownerUserId
		}
		credited, err := model.CreditAgentEarnings(row, ownerUserId)
		if err != nil {
			logger.LogError(nil, fmt.Sprintf("分润入账失败 outbox=%d: %v", row.Id, err))
			continue
		}
		if credited > 0 {
			totalQuota += credited
			totalRows++
		}
	}
	return totalQuota, totalRows
}

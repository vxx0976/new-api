package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func costRate(t *testing.T, agentId int, group string) *model.AgentGroupCost {
	t.Helper()
	row, err := model.GetAgentGroupCostForUpdate(nil, agentId, group)
	require.NoError(t, err)
	require.NotNil(t, row)
	return row
}

func sellRate(t *testing.T, agentId int, group string) float64 {
	t.Helper()
	row, err := model.GetAgentGroupSellForUpdate(nil, agentId, group)
	require.NoError(t, err)
	require.NotNil(t, row)
	return row.SellRate
}

func TestSetOwnSellRateRejectsBelowCost(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.8})

	assert.ErrorIs(t, SetOwnSellRate(owners[0].Id, "default", 0.7), ErrAgentSellBelowCost,
		"售价低于自己的进货价就是每笔倒贴")
	assert.ErrorIs(t, SetOwnSellRate(owners[0].Id, "default", 0), ErrAgentRateInvalid)

	require.NoError(t, SetOwnSellRate(owners[0].Id, "default", 0.8), "等于成本可以，零毛利是代理自己的选择")
	require.NoError(t, SetOwnSellRate(owners[0].Id, "default", 1.2))
	assert.InDelta(t, 1.2, sellRate(t, agents[0].Id, "default"), 1e-9)
}

func TestSetChildCostRateRejectsBelowOwnCost(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})

	assert.ErrorIs(t, SetChildCostRate(owners[0].Id, agents[1].Id, "default", 0.5),
		ErrAgentCostBelowOwn, "低于自己的进货价批发给下级就是自己贴钱")

	// 不是我的直接下级
	other := seedAgentUser(t, "other-owner", 0)
	otherAgent := seedAgentFor(t, other, nil, model.AgentTypeReseller)
	assert.ErrorIs(t, SetChildCostRate(owners[0].Id, otherAgent.Id, "default", 0.9),
		ErrAgentChildNotMine)
}

// 降价对下级有利，立即生效。
func TestSetChildCostRateAppliesPriceCutImmediately(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})

	require.NoError(t, SetChildCostRate(owners[0].Id, agents[1].Id, "default", 0.7))
	row := costRate(t, agents[1].Id, "default")
	assert.InDelta(t, 0.7, row.CostRate, 1e-9)
	assert.Zero(t, row.PendingEffectiveAt, "降价不需要延迟窗口")
}

// 抬价必须延迟生效并保留原价计费，否则下级毫无预警地开始倒贴。
func TestSetChildCostRateDefersPriceRaise(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.0)
	endUser := seedAgentUser(t, "raise-user", agents[1].Id)

	require.NoError(t, SetChildCostRate(owners[0].Id, agents[1].Id, "default", 0.95))

	row := costRate(t, agents[1].Id, "default")
	assert.InDelta(t, 0.8, row.CostRate, 1e-9, "抬价期间仍按原价计费")
	assert.InDelta(t, 0.95, row.PendingRate, 1e-9)
	expected := common.GetTimestamp() + int64(operation_setting.GetAgentCostRaiseDelayHours())*3600
	assert.InDelta(t, expected, row.PendingEffectiveAt, 5)

	InvalidateAgentPricingCache()
	chain := ResolveAgentPricing(endUser.Id, "default")
	require.True(t, chain.Applies)
	assert.InDelta(t, 0.8, chain.Ancestors[1].CostRate, 1e-9, "延迟窗口内计费用的仍是旧价")
}

// 到期生效后若下级来不及调价而倒挂，把售价顶到成本线：
// 既不让下级持续亏钱，也不直接断掉下级的终端客户。
func TestApplyDueAgentCostRatesLiftsSellToCostOnInversion(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 0.9)

	require.NoError(t, SetChildCostRate(owners[0].Id, agents[1].Id, "default", 1.1))
	// 把生效时间提前到现在
	require.NoError(t, model.DB.Model(&model.AgentGroupCost{}).
		Where("agent_id = ? AND group_name = ?", agents[1].Id, "default").
		Update("pending_effective_at", common.GetTimestamp()-1).Error)

	applied, err := ApplyDueAgentCostRates()
	require.NoError(t, err)
	assert.Equal(t, 1, applied)

	row := costRate(t, agents[1].Id, "default")
	assert.InDelta(t, 1.1, row.CostRate, 1e-9)
	assert.Zero(t, row.PendingEffectiveAt)
	assert.InDelta(t, 1.1, sellRate(t, agents[1].Id, "default"), 1e-9,
		"倒挂时售价被顶到成本线，不让下级继续亏着跑")

	var audits []model.AgentAuditLog
	require.NoError(t, model.DB.Where("agent_id = ? AND action = ?",
		agents[1].Id, model.AgentAuditActionSetSell).Find(&audits).Error)
	require.NotEmpty(t, audits, "自动上调售价必须留痕，不能静默发生")
}

// 售价本来就高于新成本时不动它，代理的毛利是自己的事。
func TestApplyDueAgentCostRatesKeepsHealthySellRate(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.5)

	require.NoError(t, SetChildCostRate(owners[0].Id, agents[1].Id, "default", 1.0))
	require.NoError(t, model.DB.Model(&model.AgentGroupCost{}).
		Where("agent_id = ? AND group_name = ?", agents[1].Id, "default").
		Update("pending_effective_at", common.GetTimestamp()-1).Error)

	applied, err := ApplyDueAgentCostRates()
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.InDelta(t, 1.5, sellRate(t, agents[1].Id, "default"), 1e-9)
}

func TestEffectiveCostRateInheritsFromParent(t *testing.T) {
	resetAgentMoney(t)
	rootOwner := seedAgentUser(t, "eff-root", 0)
	root := seedAgentFor(t, rootOwner, nil, model.AgentTypeReseller)
	setAgentCost(t, root.Id, "default", 0.6)
	midOwner := seedAgentUser(t, "eff-mid", root.Id)
	mid := seedAgentFor(t, midOwner, root, model.AgentTypeReseller)

	rate, ok := EffectiveCostRate(mid, "default")
	require.True(t, ok)
	assert.InDelta(t, 0.6, rate, 1e-9, "没配进货价的层继承上级的价")

	// 继承来的成本同样是售价的下限
	assert.ErrorIs(t, SetOwnSellRate(midOwner.Id, "default", 0.5), ErrAgentSellBelowCost)
	require.NoError(t, SetOwnSellRate(midOwner.Id, "default", 0.9))
}

func TestSetChildCostRateRequiresActiveParent(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	require.NoError(t, model.DB.Model(&model.Agent{}).Where("id = ?", agents[0].Id).
		Update("status", model.AgentStatusFrozen).Error)

	assert.ErrorIs(t, SetChildCostRate(owners[0].Id, agents[1].Id, "default", 0.9), ErrAgentInactive)
	assert.ErrorIs(t, SetOwnSellRate(owners[0].Id, "default", 1.0), ErrAgentInactive)
}

// 上级只能看到自己给下级设的进货价，看不到下级的售价。
func TestListChildCostRatesHidesChildSellRate(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.4)

	rows, err := ListChildCostRates(owners[0].Id, agents[1].Id)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.InDelta(t, 0.8, rows[0].CostRate, 1e-9)

	_, err = ListChildCostRates(owners[1].Id, agents[0].Id)
	assert.ErrorIs(t, err, ErrAgentChildNotMine, "不能反向查看上级的定价")
}

package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedAffiliate 在某个代理下开一个分销节点，并把它挂成 consumer 的推荐人。
func seedAffiliate(t *testing.T, name string, parent *model.Agent, ratePercent float64, inviterUserId int) (*model.Agent, *model.User) {
	t.Helper()
	owner := seedAgentUser(t, name, parent.Id)
	if inviterUserId > 0 {
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", owner.Id).
			Update("inviter_id", inviterUserId).Error)
		owner.InviterId = inviterUserId
	}
	agent := seedAgentFor(t, owner, parent, model.AgentTypeAffiliate)
	require.NoError(t, model.DB.Model(&model.Agent{}).Where("id = ?", agent.Id).
		Update("rebate_rate_percent", ratePercent).Error)
	agent.RebateRatePercent = ratePercent
	return agent, owner
}

// 分销返佣从代理的差价里切走：代理净得 + 分销净得 == 原差价，对账等式不因分销而破。
func TestAffiliateRebateSplitsAgentMarginWithoutBreakingReconciliation(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)

	affiliate, affiliateOwner := seedAffiliate(t, "aff-1", agents[0], 20, 0)
	endUser := seedAgentUser(t, "aff-customer", agents[0].Id)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", endUser.Id).
		Update("inviter_id", affiliateOwner.Id).Error)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()

	agentShare := earningQuota(t, agents[0].Id)
	affiliateShare := earningQuota(t, affiliate.Id)

	assert.Equal(t, 100_000, affiliateShare, "分销拿代理差价的 20%")
	assert.Equal(t, 400_000, agentShare, "代理拿剩下的 80%")
	assert.Equal(t, 500_000, agentShare+affiliateShare, "两者之和恒等于原差价")
}

// 逐级抽佣：推荐人的推荐人从推荐人那份里再切一刀，永远不需要知道终端客户是谁。
func TestAffiliateChainSplitsLevelByLevel(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)

	upper, upperOwner := seedAffiliate(t, "aff-upper", agents[0], 50, 0)
	lower, lowerOwner := seedAffiliate(t, "aff-lower", agents[0], 20, upperOwner.Id)

	endUser := seedAgentUser(t, "chain-customer", agents[0].Id)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", endUser.Id).
		Update("inviter_id", lowerOwner.Id).Error)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()

	// 差价 500000 → 下线分销拿 20% = 100000，其上线再从中拿 50% = 50000
	assert.Equal(t, 50_000, earningQuota(t, lower.Id))
	assert.Equal(t, 50_000, earningQuota(t, upper.Id))
	assert.Equal(t, 400_000, earningQuota(t, agents[0].Id))
	assert.Equal(t, 500_000,
		earningQuota(t, lower.Id)+earningQuota(t, upper.Id)+earningQuota(t, agents[0].Id))
}

func TestAffiliateRebateSkippedWhenRateZeroOrFrozen(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)

	affiliate, affiliateOwner := seedAffiliate(t, "aff-zero", agents[0], 0, 0)
	endUser := seedAgentUser(t, "zero-rate-customer", agents[0].Id)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", endUser.Id).
		Update("inviter_id", affiliateOwner.Id).Error)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()

	assert.Zero(t, earningQuota(t, affiliate.Id), "比例默认 0 就不返佣")
	assert.Equal(t, 500_000, earningQuota(t, agents[0].Id))
}

// 分销节点必须与出钱的那一级代理同属一条线，不能跨代理拿钱。
func TestAffiliateRebateRejectsCrossAgentInviter(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)

	otherOwner := seedAgentUser(t, "other-root", 0)
	other := seedAgentFor(t, otherOwner, nil, model.AgentTypeReseller)
	foreign, foreignOwner := seedAffiliate(t, "foreign-aff", other, 50, 0)

	endUser := seedAgentUser(t, "cross-customer", agents[0].Id)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", endUser.Id).
		Update("inviter_id", foreignOwner.Id).Error)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()

	assert.Zero(t, earningQuota(t, foreign.Id), "别家代理的分销不能从这条线拿钱")
	assert.Equal(t, 500_000, earningQuota(t, agents[0].Id))
}

func TestFreezeAgentSubtreeStopsPricingAndEarnings(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.0)
	endUser := seedAgentUser(t, "freeze-customer", agents[1].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)

	affected, err := FreezeAgentSubtree(1, agents[0].Id, "跑路")
	require.NoError(t, err)
	assert.Equal(t, 2, affected, "冻结必须覆盖整棵子树")

	for _, agent := range agents {
		reloaded, err := model.GetAgentById(agent.Id)
		require.NoError(t, err)
		assert.Equal(t, model.AgentStatusFrozen, reloaded.Status)
	}
	assert.False(t, ResolveAgentPricing(endUser.Id, "default").Applies,
		"冻结后不再按该链路计价")

	_, err = FreezeAgentSubtree(1, agents[0].Id, "重复")
	assert.ErrorIs(t, err, ErrAgentAlreadyFrozen)

	restored, err := UnfreezeAgentSubtree(1, agents[0].Id, "恢复")
	require.NoError(t, err)
	assert.Equal(t, 2, restored)
	assert.True(t, ResolveAgentPricing(endUser.Id, "default").Applies)
}

// 上提接管是显式操作，会重算被搬子树的 path 与层级。
func TestPromoteFrozenAgentChildrenReparentsSubtree(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6, 0.8, 0.9})
	root, mid, leaf := agents[0], agents[1], agents[2]

	_, err := PromoteFrozenAgentChildren(1, mid.Id, "接管")
	assert.ErrorIs(t, err, ErrAgentPromoteNotFrozen, "没冻结就不能接管")

	_, err = FreezeAgentSubtree(1, mid.Id, "跑路")
	require.NoError(t, err)

	promoted, err := PromoteFrozenAgentChildren(1, mid.Id, "接管")
	require.NoError(t, err)
	assert.Equal(t, 1, promoted)

	reloaded, err := model.GetAgentById(leaf.Id)
	require.NoError(t, err)
	assert.Equal(t, root.Id, reloaded.ParentAgentId, "下级被上提到被冻结代理的上级名下")
	assert.Equal(t, 1, reloaded.Level)
	assert.Equal(t, model.BuildAgentPath(root), reloaded.AgentPath)

	children, err := model.ListDirectChildAgents(root.Id)
	require.NoError(t, err)
	require.Len(t, children, 2, "接管后原下下级成为直接下级")

	var audits []model.AgentAuditLog
	require.NoError(t, model.DB.Where("action = ?", model.AgentAuditActionPromote).Find(&audits).Error)
	assert.NotEmpty(t, audits, "可见性边界的变更必须留痕")
}

// 可见性边界的总闸：任何一条代理侧读接口都不得泄露下下级。
func TestAgentReadPathsNeverExposeGrandchildren(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8, 0.9})
	root, mid, leaf := agents[0], agents[1], agents[2]
	setAgentSell(t, leaf.Id, "default", 1.6)
	setAgentSell(t, mid.Id, "default", 1.2)

	_, children, err := ListChildAgents(owners[0].Id)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, mid.Id, children[0].Id)

	// 下级的定价接口只回我给他的进货价，回不出他自己的售价
	costs, err := ListChildCostRates(owners[0].Id, mid.Id)
	require.NoError(t, err)
	for _, cost := range costs {
		assert.InDelta(t, 0.8, cost.CostRate, 1e-9, "只回我给他的进货价")
		assert.Less(t, cost.CostRate, 1.2, "不得把下级的售价当成进货价回出去")
	}

	// 隔代的任何操作都必须被拒
	_, err = ListChildCostRates(owners[0].Id, leaf.Id)
	assert.ErrorIs(t, err, ErrAgentChildNotMine, "上级不能直接操作下下级")
	assert.ErrorIs(t, SetChildCostRate(owners[0].Id, leaf.Id, "default", 1.0), ErrAgentChildNotMine)

	// 反向也不行：下级不能反查上级
	_, err = ListChildCostRates(owners[2].Id, mid.Id)
	assert.ErrorIs(t, err, ErrAgentChildNotMine)

	_ = root
}

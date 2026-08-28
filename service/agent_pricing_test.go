package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetAgentPricing(t *testing.T) {
	t.Helper()
	// 代理体系默认关闭，用例必须显式打开总开关，否则定价解析会直接短路。
	prevEnabled := operation_setting.GetAgentSetting().Enabled
	operation_setting.GetAgentSetting().Enabled = true
	t.Cleanup(func() { operation_setting.GetAgentSetting().Enabled = prevEnabled })
	clear := func() {
		model.DB.Exec("DELETE FROM agents")
		model.DB.Exec("DELETE FROM agent_group_costs")
		model.DB.Exec("DELETE FROM agent_group_sells")
		model.DB.Exec("DELETE FROM users")
		InvalidateAgentPricingCache()
	}
	clear()
	t.Cleanup(clear)
}

func setAgentCost(t *testing.T, agentId int, group string, rate float64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.AgentGroupCost{AgentId: agentId, Group: group, CostRate: rate}).Error)
	InvalidateAgentPricingCache()
}

func setAgentSell(t *testing.T, agentId int, group string, rate float64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.AgentGroupSell{AgentId: agentId, Group: group, SellRate: rate}).Error)
	InvalidateAgentPricingCache()
}

// buildAgentLadder 建一条 depth 级的代理链，第 i 级进货价为 costs[i]。
// 返回各级代理与各级 owner 用户。
func buildAgentLadder(t *testing.T, group string, costs []float64) ([]*model.Agent, []*model.User) {
	t.Helper()
	agents := make([]*model.Agent, 0, len(costs))
	owners := make([]*model.User, 0, len(costs))
	var parent *model.Agent
	parentAgentId := 0
	for i, cost := range costs {
		owner := seedAgentUser(t, fmt.Sprintf("ladder-owner-%d", i), parentAgentId)
		agent := seedAgentFor(t, owner, parent, model.AgentTypeReseller)
		setAgentCost(t, agent.Id, group, cost)
		agents = append(agents, agent)
		owners = append(owners, owner)
		parent = agent
		parentAgentId = agent.Id
	}
	return agents, owners
}

// 代理拿货就是按自己的进货价用，末级不赚自己的钱。
func TestResolveAgentPricingSelfUseAtCost(t *testing.T) {
	resetAgentPricing(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.0)

	chain := ResolveAgentPricing(owners[1].Id, "default")
	require.True(t, chain.Applies)
	assert.True(t, chain.SelfUse)
	assert.InDelta(t, 0.8, chain.PaidRate, 1e-9, "代理自用按自己的进货价，不是自己的对外售价")

	margins := chain.LevelMargins()
	require.Len(t, margins, 2)
	assert.InDelta(t, 0.2, margins[0].CostRate, 1e-9, "上级仍然赚这一层的差价")
	assert.InDelta(t, 0.0, margins[1].CostRate, 1e-9, "自己不赚自己的钱")
}

func TestResolveAgentPricingEndUserPaysSellRate(t *testing.T) {
	resetAgentPricing(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.05)
	endUser := seedAgentUser(t, "end-user", agents[1].Id)

	chain := ResolveAgentPricing(endUser.Id, "default")
	require.True(t, chain.Applies)
	assert.False(t, chain.SelfUse)
	assert.InDelta(t, 1.05, chain.PaidRate, 1e-9)
	assert.Equal(t, agents[1].Id, chain.LeafAgentId)
}

// 对账等式是整个体系的正确性核心：平台收 + 各级差价 == 终端实付，且与层数无关。
func TestAgentPricingChainReconcilesAtEveryDepth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		costs []float64
		sell  float64
	}{
		{"depth1", []float64{0.6}, 1.0},
		{"depth2", []float64{0.6, 0.8}, 1.0},
		{"depth5", []float64{0.50, 0.60, 0.70, 0.82, 0.91}, 1.13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAgentPricing(t)
			agents, _ := buildAgentLadder(t, "default", tc.costs)
			leaf := agents[len(agents)-1]
			setAgentSell(t, leaf.Id, "default", tc.sell)
			endUser := seedAgentUser(t, "reconcile-user", leaf.Id)

			chain := ResolveAgentPricing(endUser.Id, "default")
			require.True(t, chain.Applies)

			total := chain.PlatformRate()
			for _, margin := range chain.LevelMargins() {
				assert.GreaterOrEqual(t, margin.CostRate, 0.0, "任何一级都不得出现负差价")
				total += margin.CostRate
			}
			assert.InDelta(t, chain.PaidRate, total, 1e-9,
				"平台收 + 各级差价必须严格等于终端实付")
			assert.InDelta(t, tc.costs[0], chain.PlatformRate(), 1e-9, "平台收的是根节点的进货价")
		})
	}
}

// 代理没给这个分组定价就不能替他猜一个价，退回主站逻辑且不分润。
func TestAgentPricingFallsBackWhenSellMissing(t *testing.T) {
	resetAgentPricing(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6})
	endUser := seedAgentUser(t, "no-sell-user", agents[0].Id)

	chain := ResolveAgentPricing(endUser.Id, "default")
	assert.False(t, chain.Applies)

	rate, ok := AgentGroupRatio(endUser.Id, "default")
	assert.False(t, ok)
	assert.Zero(t, rate)
}

// 某一级没配进货价时继承上一级的价，即该级不赚差价——绝不凭空造出利润。
func TestAgentPricingInheritsMissingCostWithoutInventingMargin(t *testing.T) {
	resetAgentPricing(t)
	rootOwner := seedAgentUser(t, "inherit-root", 0)
	root := seedAgentFor(t, rootOwner, nil, model.AgentTypeReseller)
	setAgentCost(t, root.Id, "default", 0.6)

	midOwner := seedAgentUser(t, "inherit-mid", root.Id)
	mid := seedAgentFor(t, midOwner, root, model.AgentTypeReseller)
	// 故意不给 mid 配进货价
	setAgentSell(t, mid.Id, "default", 0.9)
	endUser := seedAgentUser(t, "inherit-user", mid.Id)

	chain := ResolveAgentPricing(endUser.Id, "default")
	require.True(t, chain.Applies)
	require.Len(t, chain.Ancestors, 2)
	assert.InDelta(t, 0.6, chain.Ancestors[1].CostRate, 1e-9, "未配价的层继承上级进货价")

	margins := chain.LevelMargins()
	assert.InDelta(t, 0.0, margins[0].CostRate, 1e-9, "root 不因为下级没配价就凭空多赚")
	assert.InDelta(t, 0.3, margins[1].CostRate, 1e-9)
}

// 根节点没配进货价，整条链路无法定价，必须退回主站逻辑而不是按 0 计费。
func TestAgentPricingRequiresRootCost(t *testing.T) {
	resetAgentPricing(t)
	rootOwner := seedAgentUser(t, "no-root-cost", 0)
	root := seedAgentFor(t, rootOwner, nil, model.AgentTypeReseller)
	setAgentSell(t, root.Id, "default", 1.0)
	endUser := seedAgentUser(t, "no-root-cost-user", root.Id)

	chain := ResolveAgentPricing(endUser.Id, "default")
	assert.False(t, chain.Applies, "链路不完整时不得按缺省 0 倍率放行")
}

func TestAgentPricingSkipsFrozenAgent(t *testing.T) {
	resetAgentPricing(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	endUser := seedAgentUser(t, "frozen-agent-user", agents[0].Id)

	require.True(t, ResolveAgentPricing(endUser.Id, "default").Applies)

	require.NoError(t, model.DB.Model(agents[0]).Update("status", model.AgentStatusFrozen).Error)
	InvalidateAgentPricingCache()
	assert.False(t, ResolveAgentPricing(endUser.Id, "default").Applies,
		"冻结的代理不再计价，也不再计提分润")
}

// 分销型代理没有定价权，其拥有者仍是上级的普通客户，按上级售价付费。
func TestAffiliateOwnerPaysParentSellRate(t *testing.T) {
	resetAgentPricing(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6})
	setAgentSell(t, agents[0].Id, "default", 1.0)

	affiliateOwner := seedAgentUser(t, "affiliate-owner", agents[0].Id)
	affiliate := seedAgentFor(t, affiliateOwner, agents[0], model.AgentTypeAffiliate)
	setAgentCost(t, affiliate.Id, "default", 0.6)

	chain := ResolveAgentPricing(affiliateOwner.Id, "default")
	require.True(t, chain.Applies)
	assert.False(t, chain.SelfUse, "分销型拥有者不享进货价")
	assert.InDelta(t, 1.0, chain.PaidRate, 1e-9)
	assert.Equal(t, agents[0].Id, chain.LeafAgentId)
}

// 预扣费与结算必须取同一份快照，否则中途改价会让差额退款算错。
func TestEnsureAgentPricingSnapshotSurvivesPriceChange(t *testing.T) {
	resetAgentPricing(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	endUser := seedAgentUser(t, "snapshot-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	first := EnsureAgentPricing(info)
	require.True(t, first.Applies)
	assert.InDelta(t, 1.0, first.PaidRate, 1e-9)

	require.NoError(t, model.DB.Model(&model.AgentGroupSell{}).
		Where("agent_id = ? AND group_name = ?", agents[0].Id, "default").
		Update("sell_rate", 2.0).Error)
	InvalidateAgentPricingCache()

	again := EnsureAgentPricing(info)
	assert.InDelta(t, 1.0, again.PaidRate, 1e-9,
		"同一次请求内必须复用快照，改价不得中途生效")
	assert.InDelta(t, 2.0, ResolveAgentPricing(endUser.Id, "default").PaidRate, 1e-9,
		"下一次请求才按新价")
}

// auto 分组重试会改 UsingGroup，快照必须跟着重解析，否则会拿旧分组的价结算。
func TestEnsureAgentPricingReresolvesOnGroupChange(t *testing.T) {
	resetAgentPricing(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6})
	setAgentCost(t, agents[0].Id, "vip", 0.5)
	setAgentSell(t, agents[0].Id, "default", 1.0)
	setAgentSell(t, agents[0].Id, "vip", 1.4)
	endUser := seedAgentUser(t, "group-switch-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	assert.InDelta(t, 1.0, EnsureAgentPricing(info).PaidRate, 1e-9)

	info.UsingGroup = "vip"
	assert.InDelta(t, 1.4, EnsureAgentPricing(info).PaidRate, 1e-9)
	assert.InDelta(t, 0.5, EnsureAgentPricing(info).PlatformRate(), 1e-9)
}

func TestPlatformDirectUserKeepsOriginalPricing(t *testing.T) {
	resetAgentPricing(t)
	user := seedAgentUser(t, "platform-user", 0)
	chain := ResolveAgentPricing(user.Id, "default")
	assert.False(t, chain.Applies, "不属于任何代理的用户必须走原有全局分组倍率")
}

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupAgentTables(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM agents")
		DB.Exec("DELETE FROM agent_domains")
		DB.Exec("DELETE FROM agent_group_costs")
		DB.Exec("DELETE FROM agent_group_sells")
		DB.Exec("DELETE FROM agent_earnings_outbox")
		DB.Exec("DELETE FROM agent_ledger")
	})
}

// 建一条 平台 → root → mid → leaf 的三级链路。
func seedAgentChain(t *testing.T) (root, mid, leaf *Agent) {
	t.Helper()

	root = &Agent{OwnerUserId: 9001, ParentAgentId: 0, AgentPath: "/", Level: 0,
		Type: AgentTypeReseller, Name: "root", Status: AgentStatusActive, PricingVersion: 1}
	require.NoError(t, DB.Create(root).Error)

	mid = &Agent{OwnerUserId: 9002, ParentAgentId: root.Id, AgentPath: BuildAgentPath(root), Level: 1,
		Type: AgentTypeReseller, Name: "mid", Status: AgentStatusActive, PricingVersion: 1}
	require.NoError(t, DB.Create(mid).Error)

	leaf = &Agent{OwnerUserId: 9003, ParentAgentId: mid.Id, AgentPath: BuildAgentPath(mid), Level: 2,
		Type: AgentTypeReseller, Name: "leaf", Status: AgentStatusActive, PricingVersion: 1}
	require.NoError(t, DB.Create(leaf).Error)

	return root, mid, leaf
}

func TestAgentPathEncoding(t *testing.T) {
	root := &Agent{Id: 7, AgentPath: "/"}
	mid := &Agent{Id: 23, AgentPath: BuildAgentPath(root)}

	assert.Equal(t, "/", BuildAgentPath(nil), "平台直属代理的 path 应为根")
	assert.Equal(t, "/7/", mid.AgentPath)
	assert.Equal(t, "/7/23/", BuildAgentPath(mid))

	assert.Nil(t, ParseAgentPath("/"))
	assert.Equal(t, []int{7}, ParseAgentPath("/7/"))
	assert.Equal(t, []int{7, 23}, ParseAgentPath("/7/23/"), "祖先顺序必须是根在前、父在后")
}

func TestAgentPathContainsGuardsAgainstPrefixCollision(t *testing.T) {
	// "/7/" 不能因为 "/17/" 或 "/71/" 含有字符 7 就误判成祖先。
	assert.True(t, AgentPathContains("/7/23/", 7))
	assert.True(t, AgentPathContains("/7/23/", 23))
	assert.False(t, AgentPathContains("/17/", 7))
	assert.False(t, AgentPathContains("/71/", 7))
	assert.False(t, AgentPathContains("/7/23/", 2))
	assert.False(t, AgentPathContains("/", 7))
}

func TestValidateAgentPlacementRejectsCycle(t *testing.T) {
	mid := &Agent{Id: 23, AgentPath: "/7/"}

	require.NoError(t, ValidateAgentPlacement(0, mid), "新建挂在链路下应通过")
	require.NoError(t, ValidateAgentPlacement(99, mid), "无关代理挂在链路下应通过")

	// 把 root 挂到自己的后代 mid 下面就形成了环，链路上溯会死循环并无限计提分润。
	assert.ErrorIs(t, ValidateAgentPlacement(7, mid), ErrAgentPathCycle)
	assert.ErrorIs(t, ValidateAgentPlacement(23, mid), ErrAgentPathCycle)
}

func TestValidateAgentPlacementEnforcesDepthGuard(t *testing.T) {
	deepest := &Agent{Id: AgentMaxDepth, AgentPath: "/"}
	for i := 1; i < AgentMaxDepth; i++ {
		deepest.AgentPath += "1" + string(rune('0'+i%10)) + "/"
	}
	require.Len(t, ParseAgentPath(deepest.AgentPath), AgentMaxDepth-1)

	require.NoError(t, ValidateAgentPlacement(0, &Agent{Id: 1, AgentPath: "/"}))
	assert.ErrorIs(t, ValidateAgentPlacement(0, deepest), ErrAgentPathTooDeep,
		"超出 agent_path 存储上限必须拒绝，否则路径被静默截断，链路上溯会挂到错误的祖先")
}

// 可见性边界：直接子集与子树查询必须是两个不同的结果，
// 上级只能拿到直接下级，拿不到下下级。
func TestAgentDirectChildrenExcludeGrandchildren(t *testing.T) {
	cleanupAgentTables(t)
	root, mid, leaf := seedAgentChain(t)

	children, err := ListDirectChildAgents(root.Id)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, mid.Id, children[0].Id)
	for _, child := range children {
		assert.NotEqual(t, leaf.Id, child.Id, "上级不得在直接下级里看到下下级")
	}

	subtree, err := ListSubtreeAgentsAdminOnly(root)
	require.NoError(t, err)
	assert.Len(t, subtree, 3, "子树查询（仅管理员）应覆盖 root/mid/leaf")
}

// 改价的缓存失效必须覆盖整棵子树：只失效被改的那一个代理，
// 下级会继续按旧成本计费，算出来的差价是错的。
func TestBumpSubtreePricingVersionCascades(t *testing.T) {
	cleanupAgentTables(t)
	root, mid, leaf := seedAgentChain(t)

	sibling := &Agent{OwnerUserId: 9004, ParentAgentId: 0, AgentPath: "/", Level: 0,
		Type: AgentTypeReseller, Name: "sibling", Status: AgentStatusActive, PricingVersion: 1}
	require.NoError(t, DB.Create(sibling).Error)

	require.NoError(t, BumpSubtreePricingVersion(nil, root))

	reload := func(id int) int64 {
		agent, err := GetAgentById(id)
		require.NoError(t, err)
		return agent.PricingVersion
	}
	assert.EqualValues(t, 2, reload(root.Id))
	assert.EqualValues(t, 2, reload(mid.Id))
	assert.EqualValues(t, 2, reload(leaf.Id), "下下级也必须失效，它的进货价来自被改的那一层")
	assert.EqualValues(t, 1, reload(sibling.Id), "无关子树不应被波及")
}

func TestAgentGroupPricingUniquePerAgentAndGroup(t *testing.T) {
	cleanupAgentTables(t)
	root, mid, _ := seedAgentChain(t)

	require.NoError(t, DB.Create(&AgentGroupCost{AgentId: mid.Id, Group: "default", CostRate: 0.8}).Error)
	require.Error(t, DB.Create(&AgentGroupCost{AgentId: mid.Id, Group: "default", CostRate: 0.9}).Error,
		"同一代理同一分组只能有一个进货价，否则计费取哪一行是不确定的")
	require.NoError(t, DB.Create(&AgentGroupCost{AgentId: mid.Id, Group: "vip", CostRate: 0.7}).Error)
	require.NoError(t, DB.Create(&AgentGroupCost{AgentId: root.Id, Group: "default", CostRate: 0.6}).Error)

	require.NoError(t, DB.Create(&AgentGroupSell{AgentId: mid.Id, Group: "default", SellRate: 1.0}).Error)
	require.Error(t, DB.Create(&AgentGroupSell{AgentId: mid.Id, Group: "default", SellRate: 1.2}).Error)

	costs, err := GetAgentGroupCostsByAgentIds([]int{root.Id, mid.Id}, "default")
	require.NoError(t, err)
	require.Len(t, costs, 2, "链路展开必须一次取回各级进货价")
	assert.InDelta(t, 0.6, costs[root.Id].CostRate, 1e-9)
	assert.InDelta(t, 0.8, costs[mid.Id].CostRate, 1e-9)
}

// 支付回调与消费结算都可能重放，幂等键唯一是唯一一道防线。
func TestAgentEarningsIdempotencyKeyIsUnique(t *testing.T) {
	cleanupAgentTables(t)
	_, mid, _ := seedAgentChain(t)

	row := &AgentEarningsOutbox{AgentId: mid.Id, FromUserId: 42, Amount: 1200,
		Source: AgentEarningSourceTierMarkup, RefType: "consume_window", RefId: "42:20260808",
		IdempotencyKey: "tier:consume_window:42:20260808:" + "2"}
	require.NoError(t, DB.Create(row).Error)

	replay := *row
	replay.Id = 0
	require.Error(t, DB.Create(&replay).Error, "同一幂等键重放必须被唯一索引挡住")

	ledger := &AgentLedger{AgentId: mid.Id, OwnerUserId: mid.OwnerUserId, Direction: AgentLedgerDirectionCredit,
		Amount: 1200, Source: AgentEarningSourceTierMarkup, IdempotencyKey: "ledger:1"}
	require.NoError(t, DB.Create(ledger).Error)
	dup := *ledger
	dup.Id = 0
	require.Error(t, DB.Create(&dup).Error)
}

func TestGetVerifiedAgentDomainIgnoresUnverified(t *testing.T) {
	cleanupAgentTables(t)
	root, _, _ := seedAgentChain(t)

	require.NoError(t, DB.Create(&AgentDomain{AgentId: root.Id, Domain: "pending.example.com"}).Error)
	require.NoError(t, DB.Create(&AgentDomain{AgentId: root.Id, Domain: "live.example.com",
		Verified: true, VerifiedAt: 1}).Error)

	_, err := GetVerifiedAgentDomain("pending.example.com")
	assert.Error(t, err, "未验证的域名不得生效，否则任何人都能绑别人的域名")

	got, err := GetVerifiedAgentDomain("live.example.com")
	require.NoError(t, err)
	assert.Equal(t, root.Id, got.AgentId)
}

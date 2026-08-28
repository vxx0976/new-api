package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetAgentMoney(t *testing.T) {
	t.Helper()
	// 代理体系默认关闭，用例必须显式打开总开关，否则定价解析会直接短路。
	prevEnabled := operation_setting.GetAgentSetting().Enabled
	operation_setting.GetAgentSetting().Enabled = true
	t.Cleanup(func() { operation_setting.GetAgentSetting().Enabled = prevEnabled })
	clear := func() {
		model.DB.Exec("DELETE FROM agents")
		model.DB.Exec("DELETE FROM agent_group_costs")
		model.DB.Exec("DELETE FROM agent_group_sells")
		model.DB.Exec("DELETE FROM agent_earnings_outbox")
		model.DB.Exec("DELETE FROM agent_ledger")
		model.DB.Exec("DELETE FROM agent_withdraw_requests")
		model.DB.Exec("DELETE FROM agent_audit_logs")
		model.DB.Exec("DELETE FROM users")
		InvalidateAgentPricingCache()
	}
	clear()
	t.Cleanup(clear)
}

func earningQuota(t *testing.T, agentId int) int {
	t.Helper()
	agent, err := model.GetAgentById(agentId)
	require.NoError(t, err)
	return agent.EarningQuota
}

// 对账等式的落地版：一笔消费扣的钱，等于各级已入账分润加上平台留存，一分不多一分不少。
func TestAgentEarningsReconcileAgainstChargedQuota(t *testing.T) {
	for _, tc := range []struct {
		name  string
		costs []float64
		sell  float64
		quota int
	}{
		{"depth1", []float64{0.6}, 1.0, 1_000_000},
		{"depth2", []float64{0.6, 0.8}, 1.0, 999_983},
		{"depth5", []float64{0.50, 0.60, 0.70, 0.82, 0.91}, 1.13, 777_777},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAgentMoney(t)
			agents, _ := buildAgentLadder(t, "default", tc.costs)
			leaf := agents[len(agents)-1]
			setAgentSell(t, leaf.Id, "default", tc.sell)
			endUser := seedAgentUser(t, "money-user", leaf.Id)

			info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
			require.True(t, EnsureAgentPricing(info).Applies)
			RecordAgentConsumeEarnings(info, tc.quota)

			credited, _ := SettleAgentEarningsOnce()

			totalEarned := 0
			for _, agent := range agents {
				totalEarned += earningQuota(t, agent.Id)
			}
			assert.Equal(t, credited, totalEarned)

			platformKeeps := tc.quota - totalEarned
			assert.GreaterOrEqual(t, platformKeeps, 0, "分润总额不得超过实扣额度")

			// 平台应得 ≈ quota × costs[0] / sell，整数截断的残差归平台，只多不少。
			expectedPlatform := float64(tc.quota) * tc.costs[0] / tc.sell
			assert.GreaterOrEqual(t, float64(platformKeeps), expectedPlatform-1)
			assert.LessOrEqual(t, float64(platformKeeps), expectedPlatform+float64(len(agents))+1,
				"残差不得超过每级 1 个额度单位的截断上限")
		})
	}
}

// 代理自用按进货价，末级差价为 0，不能给自己发钱。
func TestAgentSelfUseEarnsNothingForItself(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.0)

	info := &relaycommon.RelayInfo{UserId: owners[1].Id, UsingGroup: "default"}
	chain := EnsureAgentPricing(info)
	require.True(t, chain.Applies)
	require.True(t, chain.SelfUse)
	RecordAgentConsumeEarnings(info, 800_000)
	SettleAgentEarningsOnce()

	assert.Zero(t, earningQuota(t, agents[1].Id), "自己不赚自己的钱")
	assert.Positive(t, earningQuota(t, agents[0].Id), "上级该赚的那层照赚")
}

// 同一个时间窗内的多笔消费聚合成一行，避免深链路下每请求写多条。
func TestAgentEarningsAggregateWithinWindow(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	endUser := seedAgentUser(t, "agg-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	for i := 0; i < 5; i++ {
		RecordAgentConsumeEarnings(info, 100_000)
	}

	var rows int64
	require.NoError(t, model.DB.Model(&model.AgentEarningsOutbox{}).Count(&rows).Error)
	assert.EqualValues(t, 1, rows, "同窗口内应只有一行聚合记录")

	SettleAgentEarningsOnce()
	assert.Equal(t, 5*40_000, earningQuota(t, agents[0].Id))
}

// 窗口仍在累加时被结算过，后续增量必须还能补上，且已发的部分不能重复发。
func TestAgentEarningsCreditsIncrementallyWithoutDoublePay(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	endUser := seedAgentUser(t, "incr-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)

	RecordAgentConsumeEarnings(info, 200_000)
	SettleAgentEarningsOnce()
	assert.Equal(t, 100_000, earningQuota(t, agents[0].Id))

	// 重复结算不得再发一次
	SettleAgentEarningsOnce()
	assert.Equal(t, 100_000, earningQuota(t, agents[0].Id))

	// 同窗口继续消费，只补增量
	RecordAgentConsumeEarnings(info, 200_000)
	SettleAgentEarningsOnce()
	assert.Equal(t, 200_000, earningQuota(t, agents[0].Id))

	settled, err := model.SumAgentSettledEarnings(agents[0].Id)
	require.NoError(t, err)
	assert.Equal(t, 200_000, settled, "账本汇总必须等于钱包余额")
}

// 差价为 0 的层不写行：金额为 0 的记录没有意义，还会污染账本。
func TestAgentEarningsSkipsZeroMarginLevels(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6, 0.6})
	setAgentSell(t, agents[1].Id, "default", 1.0)
	endUser := seedAgentUser(t, "zero-margin-user", agents[1].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 500_000)

	var rows []*model.AgentEarningsOutbox
	require.NoError(t, model.DB.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, agents[1].Id, rows[0].AgentId, "中间层没有差价就不该有分润记录")
	assert.Positive(t, rows[0].Amount)
}

func TestAgentEarningsSkippedWhenPricingNotApplied(t *testing.T) {
	resetAgentMoney(t)
	user := seedAgentUser(t, "plain", 0)
	info := &relaycommon.RelayInfo{UserId: user.Id, UsingGroup: "default"}
	EnsureAgentPricing(info)
	RecordAgentConsumeEarnings(info, 1_000_000)

	var rows int64
	require.NoError(t, model.DB.Model(&model.AgentEarningsOutbox{}).Count(&rows).Error)
	assert.Zero(t, rows)
}

func TestAgentWithdrawFreezesQuotaAndRefundsOnReject(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	endUser := seedAgentUser(t, "withdraw-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()
	require.Equal(t, 500_000, earningQuota(t, agents[0].Id))

	_, err := CreateAgentWithdraw(owners[0].Id, 600_000, "bank")
	assert.ErrorIs(t, err, ErrAgentWithdrawInsufficient, "不得超提")

	req, err := CreateAgentWithdraw(owners[0].Id, 300_000, "bank")
	require.NoError(t, err)
	assert.Equal(t, 200_000, earningQuota(t, agents[0].Id),
		"申请即冻结额度，否则审核期间可以重复申请把余额提空")

	require.NoError(t, ReviewAgentWithdraw(1, req.Id, false, "信息不全", ""))
	assert.Equal(t, 500_000, earningQuota(t, agents[0].Id), "驳回必须退回冻结额度")

	require.ErrorIs(t, ReviewAgentWithdraw(1, req.Id, true, "", "ref"), ErrAgentWithdrawStateInvalid,
		"已终结的提现单不能再次审核")
}

func TestAgentWithdrawApprovalRecordsWithdrawnTotal(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	endUser := seedAgentUser(t, "withdraw-ok-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()

	req, err := CreateAgentWithdraw(owners[0].Id, 400_000, "bank")
	require.NoError(t, err)
	require.NoError(t, ReviewAgentWithdraw(1, req.Id, true, "", "TX-1"))

	agent, err := model.GetAgentById(agents[0].Id)
	require.NoError(t, err)
	assert.Equal(t, 100_000, agent.EarningQuota)
	assert.Equal(t, 400_000, agent.WithdrawnQuota)
	assert.Equal(t, 500_000, agent.HistoryEarningQuota, "累计收入不受提现影响")
}

// 分润钱包与消费余额必须完全隔离：提现动的是 EarningQuota，不是 users.quota。
func TestAgentWithdrawDoesNotTouchConsumerBalance(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.5})
	setAgentSell(t, agents[0].Id, "default", 1.0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", owners[0].Id).
		Update("quota", 123_456).Error)
	endUser := seedAgentUser(t, "isolation-user", agents[0].Id)

	info := &relaycommon.RelayInfo{UserId: endUser.Id, UsingGroup: "default"}
	require.True(t, EnsureAgentPricing(info).Applies)
	RecordAgentConsumeEarnings(info, 1_000_000)
	SettleAgentEarningsOnce()

	req, err := CreateAgentWithdraw(owners[0].Id, 500_000, "bank")
	require.NoError(t, err)
	require.NoError(t, ReviewAgentWithdraw(1, req.Id, true, "", "TX-2"))

	owner, err := model.GetUserById(owners[0].Id, false)
	require.NoError(t, err)
	assert.Equal(t, 123_456, owner.Quota, "提现不得动消费余额")
}

func TestAgentEarningsWindowKeyIsStable(t *testing.T) {
	first := currentAgentEarningsWindow()
	assert.Equal(t, first, currentAgentEarningsWindow())
	assert.Zero(t, first%3600, "窗口起点必须对齐到整点")
	assert.LessOrEqual(t, first, common.GetTimestamp())
}

// 分润必须挂在真实结算路径上，而不是只在被直接调用时才计提。
//
// 这条用例是为一个真实 bug 补的：最初 hook 挂在 PostConsumeQuota 上，但 chat completion
// 走的是 SettleBilling 里的 BillingSession 分支，压根不经过 PostConsumeQuota，
// 于是线上跑真实请求时一条分润都没计提。而当时所有用例都直接调 RecordAgentConsumeEarnings，
// 完全测不到接线本身。改成从 SettleBilling 进入，接线断了就会红。
func TestSettleBillingRecordsAgentEarnings(t *testing.T) {
	resetAgentMoney(t)
	agents, _ := buildAgentLadder(t, "default", []float64{0.6, 0.8})
	setAgentSell(t, agents[1].Id, "default", 1.0)
	endUser := seedAgentUser(t, "settle-hook-user", agents[1].Id)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", endUser.Id).
		Update("quota", 10_000_000).Error)

	info := &relaycommon.RelayInfo{
		UserId:       endUser.Id,
		UsingGroup:   "default",
		IsPlayground: true, // 跳过令牌扣减，本例只关心分润有没有被计提
	}
	require.True(t, EnsureAgentPricing(info).Applies)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, SettleBilling(ctx, info, 1_000_000))

	var rows []*model.AgentEarningsOutbox
	require.NoError(t, model.DB.Order("agent_id").Find(&rows).Error)
	require.Len(t, rows, 2, "两级代理都应被计提，一条都没有说明 hook 没挂在结算路径上")
	// 差价按倍率相减再截断，浮点残差归平台，所以允许比理论值少 1 个单位，但绝不能多。
	assert.InDelta(t, 200_000, rows[0].Amount, 1, "上级差价 0.8−0.6")
	assert.InDelta(t, 200_000, rows[1].Amount, 1, "末级差价 1.0−0.8")

	SettleAgentEarningsOnce()
	totalEarned := earningQuota(t, agents[0].Id) + earningQuota(t, agents[1].Id)
	assert.InDelta(t, 400_000, totalEarned, 2)
	assert.LessOrEqual(t, totalEarned, 1_000_000-600_000,
		"两级分润之和不得超过实扣额度减去平台应收的部分")
}

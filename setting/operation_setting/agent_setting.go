package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// AgentSetting 多级代理体系配置。
type AgentSetting struct {
	Enabled bool `json:"enabled"` // 总开关，默认关

	// CostRaiseDelayHours 上级抬高下级进货价的生效延迟。
	// 抬价必须给下级留出调价窗口，否则下级会在毫无预警的情况下开始倒贴。
	CostRaiseDelayHours int `json:"cost_raise_delay_hours"`

	// EarningsWindowHours 消费分润的聚合窗口。窗口内同一 (代理, 客户) 只累加一行，
	// 否则深链路下每请求写多条会造成严重写放大。
	EarningsWindowHours int `json:"earnings_window_hours"`

	// MinWithdrawQuota 单次提现的最小额度，0 表示不限制。
	MinWithdrawQuota int `json:"min_withdraw_quota"`
}

var agentSetting = AgentSetting{
	Enabled:             false,
	CostRaiseDelayHours: 72,
	EarningsWindowHours: 1,
	MinWithdrawQuota:    0,
}

func init() {
	config.GlobalConfig.Register("agent_setting", &agentSetting)
}

func GetAgentSetting() *AgentSetting {
	return &agentSetting
}

// GetAgentCostRaiseDelayHours 返回抬价生效延迟，配置异常时回落到默认的 72 小时。
func GetAgentCostRaiseDelayHours() int {
	if agentSetting.CostRaiseDelayHours <= 0 {
		return 72
	}
	return agentSetting.CostRaiseDelayHours
}

// GetAgentEarningsWindowHours 返回分润聚合窗口小时数，最小 1 小时。
func GetAgentEarningsWindowHours() int {
	if agentSetting.EarningsWindowHours <= 0 {
		return 1
	}
	return agentSetting.EarningsWindowHours
}

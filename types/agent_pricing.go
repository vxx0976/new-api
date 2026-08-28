package types

// AgentChainNode 代理链路上某一级及其进货倍率。
type AgentChainNode struct {
	AgentId  int
	CostRate float64
}

// AgentPricingChain 一次请求内只解析一次的代理定价链路快照。
//
// 必须是快照而不是「用到再查」：预扣费与结算之间若各查一次，中间代理改价会让两次
// 取到不同的倍率，差额退款直接算错。解析结果挂在 RelayInfo 上，全生命周期复用。
//
// 类型放在 types 而非 service，是因为 RelayInfo 所在的 relay/common 被 service 依赖，
// 反向引用会成环。
//
// 对账等式（base 为未乘倍率的基准额度）：
//
//	平台收   = Ancestors[0].CostRate × base
//	第 i 级赚 = (Ancestors[i+1].CostRate − Ancestors[i].CostRate) × base
//	末级赚   = (PaidRate − Ancestors[last].CostRate) × base
//	三者之和 = PaidRate × base = 用户实付，与层数无关
type AgentPricingChain struct {
	Applies     bool // false 表示该用户不参与代理定价，走原有全局分组倍率
	Group       string
	LeafAgentId int
	SelfUse     bool // 代理拥有者自用：按自己的进货价，末级利润为 0
	PaidRate    float64
	Ancestors   []AgentChainNode // 根→叶
}

// PlatformRate 返回平台在这条链路上收取的倍率。
func (c *AgentPricingChain) PlatformRate() float64 {
	if c == nil || !c.Applies || len(c.Ancestors) == 0 {
		return 0
	}
	return c.Ancestors[0].CostRate
}

// LevelMargins 返回各级代理的差价倍率，与 Ancestors 一一对应。
// 末级用实付价减自己的进货价；其余各级用下一级的进货价减自己的进货价。
func (c *AgentPricingChain) LevelMargins() []AgentChainNode {
	if c == nil || !c.Applies || len(c.Ancestors) == 0 {
		return nil
	}
	margins := make([]AgentChainNode, 0, len(c.Ancestors))
	for i, node := range c.Ancestors {
		next := c.PaidRate
		if i+1 < len(c.Ancestors) {
			next = c.Ancestors[i+1].CostRate
		}
		margins = append(margins, AgentChainNode{AgentId: node.AgentId, CostRate: next - node.CostRate})
	}
	return margins
}

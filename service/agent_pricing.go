package service

import (
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

// agentPricingTTLSeconds 限定改价在多实例部署下的最大生效延迟。
const agentPricingTTLSeconds int64 = 30

type agentPricingCacheEntry struct {
	chain     types.AgentPricingChain
	expiresAt int64
}

var (
	agentPricingCacheMu sync.RWMutex
	agentPricingCache   = map[string]agentPricingCacheEntry{}
)

// InvalidateAgentPricingCache 在任何改价、建树、冻结之后整体清空定价缓存。
//
// 这里刻意不做「只失效被改代理的子树」的精确失效：逐级链路下一个代理的进货价
// 变动会改变其整棵子树每一级的差价，精确失效要反查子树、极易漏，而漏了就是算错钱。
// 改价是低频管理操作，全清更简单也更安全。多实例部署下其他实例最多按 TTL 滞后。
func InvalidateAgentPricingCache() {
	agentPricingCacheMu.Lock()
	agentPricingCache = map[string]agentPricingCacheEntry{}
	agentPricingCacheMu.Unlock()
}

// ResolveAgentPricing 解析某用户在某分组下的代理定价链路。
// 不参与代理体系、链路不完整、或末级未配价时返回 Applies=false，调用方走原有逻辑。
func ResolveAgentPricing(userId int, group string) types.AgentPricingChain {
	if userId <= 0 || group == "" {
		return types.AgentPricingChain{}
	}
	// 总开关关闭时整套体系不生效：定价退回主站原有分组倍率，分润也不再计提
	// （分润基数来自这里返回的链路，Applies=false 就没有任何一级会被计提）。
	if !operation_setting.GetAgentSetting().Enabled {
		return types.AgentPricingChain{Group: group}
	}
	key := fmt.Sprintf("%d:%s", userId, group)
	now := common.GetTimestamp()

	agentPricingCacheMu.RLock()
	entry, ok := agentPricingCache[key]
	agentPricingCacheMu.RUnlock()
	if ok && entry.expiresAt > now {
		return entry.chain
	}

	chain := resolveAgentPricingFromDB(userId, group)
	chain.Group = group
	agentPricingCacheMu.Lock()
	agentPricingCache[key] = agentPricingCacheEntry{chain: chain, expiresAt: now + agentPricingTTLSeconds}
	agentPricingCacheMu.Unlock()
	return chain
}

func resolveAgentPricingFromDB(userId int, group string) types.AgentPricingChain {
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return types.AgentPricingChain{}
	}

	leaf, selfUse := resolvePricingLeaf(user)
	if leaf == nil || leaf.Status != model.AgentStatusActive {
		return types.AgentPricingChain{}
	}

	ancestors, ok := buildAgentCostChain(leaf, group)
	if !ok {
		return types.AgentPricingChain{}
	}

	chain := types.AgentPricingChain{
		LeafAgentId: leaf.Id,
		SelfUse:     selfUse,
		Ancestors:   ancestors,
	}
	if selfUse {
		// 代理拿货就是按自己的进货价用，末级不赚自己的钱。
		chain.Applies = true
		chain.PaidRate = ancestors[len(ancestors)-1].CostRate
		return chain
	}

	sells, err := model.GetAgentGroupSells(leaf.Id)
	if err != nil {
		return types.AgentPricingChain{}
	}
	sell, ok := sells[group]
	if !ok {
		// 代理没给这个分组定价就不能替他猜一个价，退回主站原有逻辑且不分润。
		return types.AgentPricingChain{}
	}
	chain.Applies = true
	chain.PaidRate = sell.SellRate
	return chain
}

// resolvePricingLeaf 判定这个用户按谁的价计费：
// 经销型代理的拥有者按自己的进货价，其余人按其归属代理的售价。
//
// 分销型代理没有域名也没有定价权，其拥有者仍是上级的普通客户，按上级售价付费。
func resolvePricingLeaf(user *model.User) (*model.Agent, bool) {
	owned, err := model.GetAgentByOwnerUserId(user.Id)
	if err == nil && owned != nil &&
		owned.Type == model.AgentTypeReseller &&
		owned.ParentAgentId == user.ParentAgentId {
		return owned, true
	}
	if user.ParentAgentId == 0 {
		return nil, false
	}
	parent, err := model.GetAgentById(user.ParentAgentId)
	if err != nil {
		return nil, false
	}
	return parent, false
}

// buildAgentCostChain 展开根→叶各级的进货倍率。
//
// 某一级没配进货价时继承上一级的价，即该级不赚差价——绝不凭空造出利润。
// 根节点没配价则整条链路无法定价，返回 false 由调用方退回主站逻辑。
func buildAgentCostChain(leaf *model.Agent, group string) ([]types.AgentChainNode, bool) {
	ids := append(model.ParseAgentPath(leaf.AgentPath), leaf.Id)
	costs, err := model.GetAgentGroupCostsByAgentIds(ids, group)
	if err != nil {
		return nil, false
	}

	nodes := make([]types.AgentChainNode, 0, len(ids))
	for i, id := range ids {
		rate := 0.0
		if cost, ok := costs[id]; ok {
			rate = cost.CostRate
		} else if i > 0 {
			rate = nodes[i-1].CostRate
		} else {
			return nil, false
		}
		if rate <= 0 {
			return nil, false
		}
		nodes = append(nodes, types.AgentChainNode{AgentId: id, CostRate: rate})
	}
	return nodes, true
}

// AgentGroupRatio 返回该用户在某分组下应当生效的绝对倍率。
// 第二个返回值为 false 时调用方必须走原有的全局分组倍率逻辑。
func AgentGroupRatio(userId int, group string) (float64, bool) {
	chain := ResolveAgentPricing(userId, group)
	if !chain.Applies {
		return 0, false
	}
	return chain.PaidRate, true
}

// EnsureAgentPricing 取本次请求的链路快照，没有就解析一次并挂到 RelayInfo 上。
//
// 预扣费、结算、任务计费都必须经由这里拿倍率，谁先到谁负责解析。
// UsingGroup 在 auto 分组重试时会变，快照记录了解析时所用的分组，
// 分组变了才重新解析——否则重试后会拿着旧分组的价结算。
func EnsureAgentPricing(relayInfo *relaycommon.RelayInfo) *types.AgentPricingChain {
	if relayInfo == nil {
		return nil
	}
	if relayInfo.AgentPricing != nil && relayInfo.AgentPricing.Group == relayInfo.UsingGroup {
		return relayInfo.AgentPricing
	}
	chain := ResolveAgentPricing(relayInfo.UserId, relayInfo.UsingGroup)
	relayInfo.AgentPricing = &chain
	return relayInfo.AgentPricing
}

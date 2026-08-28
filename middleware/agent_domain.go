package middleware

import (
	"net"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const agentDomainCacheTTLSeconds int64 = 60

type agentDomainCacheEntry struct {
	agentId   int // 0 表示该域名不属于任何代理，负缓存
	expiresAt int64
}

var (
	agentDomainCacheMu sync.RWMutex
	agentDomainCache   = map[string]agentDomainCacheEntry{}
)

// InvalidateAgentDomainCache 在绑定、解绑、验证域名之后清空解析缓存。
func InvalidateAgentDomainCache() {
	agentDomainCacheMu.Lock()
	agentDomainCache = map[string]agentDomainCacheEntry{}
	agentDomainCacheMu.Unlock()
}

// AgentDomainResolver 按 Host 解析白标域名，命中则把代理 id 注入 context。
//
// 未命中同样写入缓存（负缓存）：否则拿任意不存在的域名刷接口就能把每个请求都打到数据库。
func AgentDomainResolver() func(c *gin.Context) {
	return func(c *gin.Context) {
		host := normalizeHost(c.Request.Host)
		if host != "" {
			if agentId := resolveAgentIdByHost(host); agentId > 0 {
				common.SetContextKey(c, constant.ContextKeyAgentId, agentId)
			}
		}
		c.Next()
	}
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}

func resolveAgentIdByHost(host string) int {
	now := common.GetTimestamp()
	agentDomainCacheMu.RLock()
	entry, ok := agentDomainCache[host]
	agentDomainCacheMu.RUnlock()
	if ok && entry.expiresAt > now {
		return entry.agentId
	}

	agentId := 0
	if row, err := model.GetVerifiedAgentDomain(host); err == nil && row != nil {
		agentId = row.AgentId
	}
	agentDomainCacheMu.Lock()
	agentDomainCache[host] = agentDomainCacheEntry{agentId: agentId, expiresAt: now + agentDomainCacheTTLSeconds}
	agentDomainCacheMu.Unlock()
	return agentId
}

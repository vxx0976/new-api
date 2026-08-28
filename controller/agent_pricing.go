package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type agentRateRequest struct {
	Group string  `json:"group"`
	Rate  float64 `json:"rate"`
}

type agentAdminRateRequest struct {
	AgentId int     `json:"agent_id"`
	Group   string  `json:"group"`
	Rate    float64 `json:"rate"`
}

type agentPricingRow struct {
	Group              string  `json:"group"`
	CostRate           float64 `json:"cost_rate"`
	SellRate           float64 `json:"sell_rate"`
	PendingCostRate    float64 `json:"pending_cost_rate"`
	PendingEffectiveAt int64   `json:"pending_effective_at"`
}

// agentChildPricingRow 上级看下级的定价。
// 只含「我给他的进货价」，不含他的售价——看得到下级售价就能反推其利润率，
// 直接影响议价，这正是逐级模型要隔离的东西。
type agentChildPricingRow struct {
	Group              string  `json:"group"`
	CostRate           float64 `json:"cost_rate"`
	PendingCostRate    float64 `json:"pending_cost_rate"`
	PendingEffectiveAt int64   `json:"pending_effective_at"`
}

// GetSelfAgentPricing 返回自己的进货价与售价。
func GetSelfAgentPricing(c *gin.Context) {
	agent, err := service.GetOwnedAgent(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	costs, err := model.GetAgentGroupCosts(agent.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sells, err := model.GetAgentGroupSells(agent.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rows := make([]agentPricingRow, 0, len(costs))
	seen := map[string]bool{}
	for group, cost := range costs {
		seen[group] = true
		row := agentPricingRow{
			Group: group, CostRate: cost.CostRate,
			PendingCostRate: cost.PendingRate, PendingEffectiveAt: cost.PendingEffectiveAt,
		}
		if sell, ok := sells[group]; ok {
			row.SellRate = sell.SellRate
		}
		rows = append(rows, row)
	}
	for group, sell := range sells {
		if seen[group] {
			continue
		}
		// 自己没配进货价时按链路继承上级的价，这里展示实际生效值而不是空。
		effective, _ := service.EffectiveCostRate(agent, group)
		rows = append(rows, agentPricingRow{Group: group, CostRate: effective, SellRate: sell.SellRate})
	}
	common.ApiSuccess(c, rows)
}

// SetSelfAgentSellRate 代理设自己对终端用户的售价。
func SetSelfAgentSellRate(c *gin.Context) {
	var req agentRateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	if err := service.SetOwnSellRate(c.GetInt("id"), req.Group, req.Rate); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// GetChildAgentPricing 上级查看自己给某个直接下级设的进货价。
func GetChildAgentPricing(c *gin.Context) {
	childId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	rows, err := service.ListChildCostRates(c.GetInt("id"), childId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]agentChildPricingRow, 0, len(rows))
	for _, row := range rows {
		views = append(views, agentChildPricingRow{
			Group: row.Group, CostRate: row.CostRate,
			PendingCostRate: row.PendingRate, PendingEffectiveAt: row.PendingEffectiveAt,
		})
	}
	common.ApiSuccess(c, views)
}

// SetChildAgentCostRate 上级给直接下级设进货价。抬价延迟生效。
func SetChildAgentCostRate(c *gin.Context) {
	childId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	var req agentRateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	if err := service.SetChildCostRate(c.GetInt("id"), childId, req.Group, req.Rate); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// SetPlatformAgentCostRate 管理员给平台直属代理设进货价。
func SetPlatformAgentCostRate(c *gin.Context) {
	var req agentAdminRateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	if err := service.SetPlatformAgentCostRate(c.GetInt("id"), req.AgentId, req.Group, req.Rate); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

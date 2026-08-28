package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// agentSelfView 代理看自己：全量。
type agentSelfView struct {
	Id                  int     `json:"id"`
	ParentAgentId       int     `json:"parent_agent_id"`
	Level               int     `json:"level"`
	Type                string  `json:"type"`
	Name                string  `json:"name"`
	Status              int     `json:"status"`
	EarningQuota        int     `json:"earning_quota"`
	HistoryEarningQuota int     `json:"history_earning_quota"`
	WithdrawnQuota      int     `json:"withdrawn_quota"`
	RebateRatePercent   float64 `json:"rebate_rate_percent"`
	CreatedAt           int64   `json:"created_at"`
}

// agentChildView 上级看直接下级。
//
// 这个结构体是可见性边界的物理载体，缺失的字段是刻意缺的：
//   - 不含下级的售价：上级能看到下级的售价就能反推其利润率，直接影响议价；
//   - 不含下级的下级数量：那是下下级的存在性，逐级模型里上级无权知晓；
//   - 不含下级的分润余额与 agent_path：同上，属于下级自己的经营数据。
//
// 别为了省事直接序列化 model.Agent —— 那会把上面三类字段一次性全泄露出去。
type agentChildView struct {
	Id                int     `json:"id"`
	OwnerUserId       int     `json:"owner_user_id"`
	OwnerUsername     string  `json:"owner_username"`
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	Status            int     `json:"status"`
	RebateRatePercent float64 `json:"rebate_rate_percent"` // 我给他设的返佣比例，仅分销型有意义
	CreatedAt         int64   `json:"created_at"`
}

type createAgentRequest struct {
	UserId int    `json:"user_id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
}

// GetSelfAgent 返回当前用户拥有的代理。非代理用户返回 is_agent=false，不报错，
// 前端据此决定是否展示代理入口。
func GetSelfAgent(c *gin.Context) {
	agent, err := service.GetOwnedAgent(c.GetInt("id"))
	if err == service.ErrAgentNotFound {
		common.ApiSuccess(c, gin.H{"is_agent": false})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"is_agent": true, "agent": toAgentSelfView(agent)})
}

// GetChildAgents 返回当前代理的直接下级。
func GetChildAgents(c *gin.Context) {
	_, children, err := service.ListChildAgents(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views, err := toAgentChildViews(children)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, views)
}

// CreateSubAgent 上级代理开通自己的直接下级。
func CreateSubAgent(c *gin.Context) {
	var req createAgentRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	agent, err := service.CreateSubAgent(service.CreateAgentInput{
		OperatorUserId: c.GetInt("id"),
		TargetUserId:   req.UserId,
		Type:           req.Type,
		Name:           req.Name,
		Ip:             c.ClientIP(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAgentSelfView(agent))
}

// CreatePlatformAgent 管理员开通平台直属代理。
func CreatePlatformAgent(c *gin.Context) {
	var req createAgentRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	agent, err := service.CreatePlatformAgent(service.CreateAgentInput{
		OperatorUserId: c.GetInt("id"),
		TargetUserId:   req.UserId,
		Type:           req.Type,
		Name:           req.Name,
		Ip:             c.ClientIP(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toAgentSelfView(agent))
}

func toAgentSelfView(agent *model.Agent) agentSelfView {
	return agentSelfView{
		Id:                  agent.Id,
		ParentAgentId:       agent.ParentAgentId,
		Level:               agent.Level,
		Type:                agent.Type,
		Name:                agent.Name,
		Status:              agent.Status,
		EarningQuota:        agent.EarningQuota,
		HistoryEarningQuota: agent.HistoryEarningQuota,
		WithdrawnQuota:      agent.WithdrawnQuota,
		RebateRatePercent:   agent.RebateRatePercent,
		CreatedAt:           agent.CreatedAt,
	}
}

func toAgentChildViews(children []*model.Agent) ([]agentChildView, error) {
	views := make([]agentChildView, 0, len(children))
	if len(children) == 0 {
		return views, nil
	}
	ownerIds := make([]int, 0, len(children))
	for _, child := range children {
		ownerIds = append(ownerIds, child.OwnerUserId)
	}
	usernames, err := model.GetAgentOwnerUsernames(ownerIds)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		views = append(views, agentChildView{
			Id:                child.Id,
			OwnerUserId:       child.OwnerUserId,
			OwnerUsername:     usernames[child.OwnerUserId],
			Name:              child.Name,
			Type:              child.Type,
			Status:            child.Status,
			RebateRatePercent: child.RebateRatePercent,
			CreatedAt:         child.CreatedAt,
		})
	}
	return views, nil
}

type agentLifecycleRequest struct {
	AgentId int    `json:"agent_id"`
	Note    string `json:"note"`
}

// FreezeAgent 管理员冻结代理及其整棵子树。
func FreezeAgent(c *gin.Context) {
	var req agentLifecycleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	affected, err := service.FreezeAgentSubtree(c.GetInt("id"), req.AgentId, req.Note)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"affected": affected})
}

// UnfreezeAgent 管理员解冻代理及其整棵子树。
func UnfreezeAgent(c *gin.Context) {
	var req agentLifecycleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	affected, err := service.UnfreezeAgentSubtree(c.GetInt("id"), req.AgentId, req.Note)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"affected": affected})
}

// PromoteAgentChildren 管理员把被冻结代理的下级上提到其上级名下。
// 这会让上级看到原本不可见的下下级，属于显式接管，不是冻结的副作用。
func PromoteAgentChildren(c *gin.Context) {
	var req agentLifecycleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	promoted, err := service.PromoteFrozenAgentChildren(c.GetInt("id"), req.AgentId, req.Note)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"promoted": promoted})
}

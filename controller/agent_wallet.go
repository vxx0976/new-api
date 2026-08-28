package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type agentWithdrawRequest struct {
	Quota     int    `json:"quota"`
	PayeeInfo string `json:"payee_info"`
}

type agentWithdrawReviewRequest struct {
	Approve    bool   `json:"approve"`
	Note       string `json:"note"`
	PaymentRef string `json:"payment_ref"`
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 100 {
		size = 20
	}
	return (page - 1) * size, size
}

// GetAgentLedger 返回自己分润钱包的资金流水。
func GetAgentLedger(c *gin.Context) {
	agent, err := service.GetOwnedAgent(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	offset, limit := pageParams(c)
	rows, err := model.ListAgentLedger(agent.Id, offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// GetAgentWithdraws 返回自己的提现记录。
func GetAgentWithdraws(c *gin.Context) {
	agent, err := service.GetOwnedAgent(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	offset, limit := pageParams(c)
	rows, err := model.ListAgentWithdraws(agent.Id, offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// CreateAgentWithdraw 代理发起提现。上级无权干预。
func CreateAgentWithdraw(c *gin.Context) {
	var req agentWithdrawRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	withdraw, err := service.CreateAgentWithdraw(c.GetInt("id"), req.Quota, req.PayeeInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, withdraw)
}

// GetPendingAgentWithdraws 管理员查看待审核提现单。
func GetPendingAgentWithdraws(c *gin.Context) {
	offset, limit := pageParams(c)
	rows, err := model.ListPendingAgentWithdraws(offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// ReviewAgentWithdraw 管理员审核提现单，驳回时退回冻结额度。
func ReviewAgentWithdraw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	var req agentWithdrawReviewRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	if err := service.ReviewAgentWithdraw(c.GetInt("id"), id, req.Approve, req.Note, req.PaymentRef); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

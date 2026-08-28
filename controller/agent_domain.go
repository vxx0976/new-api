package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type bindAgentDomainRequest struct {
	Domain string `json:"domain"`
}

type agentDomainBrandingRequest struct {
	SiteName       string `json:"site_name"`
	SiteLogo       string `json:"site_logo"`
	BrandColor     string `json:"brand_color"`
	CustomCss      string `json:"custom_css"`
	HomeContent    string `json:"home_content"`
	SeoTitle       string `json:"seo_title"`
	SeoDescription string `json:"seo_description"`
	SeoKeywords    string `json:"seo_keywords"`
}

// GetAgentDomains 返回当前代理名下的白标域名。
func GetAgentDomains(c *gin.Context) {
	rows, err := service.ListOwnAgentDomains(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// BindAgentDomain 绑定白标域名，返回需要写入 DNS 的 TXT 令牌。
func BindAgentDomain(c *gin.Context) {
	var req bindAgentDomainRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	row, err := service.BindAgentDomain(c.GetInt("id"), req.Domain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":           row.Id,
		"domain":       row.Domain,
		"verify_token": row.VerifyToken,
		"verified":     row.Verified,
	})
}

// VerifyAgentDomain 触发域名归属校验。
func VerifyAgentDomain(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	if err := service.VerifyAgentDomain(c.GetInt("id"), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// UpdateAgentDomainBranding 更新白标品牌信息。
func UpdateAgentDomainBranding(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	var req agentDomainBrandingRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	patch := &model.AgentDomain{
		SiteName: req.SiteName, SiteLogo: req.SiteLogo, BrandColor: req.BrandColor,
		CustomCss: req.CustomCss, HomeContent: req.HomeContent,
		SeoTitle: req.SeoTitle, SeoDescription: req.SeoDescription, SeoKeywords: req.SeoKeywords,
	}
	if err := service.UpdateAgentDomainBranding(c.GetInt("id"), id, patch); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// UnbindAgentDomain 解绑白标域名。
func UnbindAgentDomain(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "请求参数不合法")
		return
	}
	if err := service.UnbindAgentDomain(c.GetInt("id"), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

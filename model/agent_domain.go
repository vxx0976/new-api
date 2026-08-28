package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// AgentDomain 代理的白标域名与站点品牌。代理不需要部署，只需把域名解析过来并完成校验。
//
// 未通过校验的域名不生效，否则任何人都能绑别人的域名。
// 本表不做软删除：解绑即物理删除，域名可立即被重新绑定；审计留痕在 agent_audit_logs。
type AgentDomain struct {
	Id      int    `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId int    `json:"agent_id" gorm:"not null;index"`
	Domain  string `json:"domain" gorm:"type:varchar(255);not null;uniqueIndex"`

	SiteName    string `json:"site_name" gorm:"type:varchar(100)"`
	SiteLogo    string `json:"site_logo" gorm:"type:text"`
	BrandColor  string `json:"brand_color" gorm:"type:varchar(20)"`
	CustomCss   string `json:"custom_css" gorm:"type:text"`
	HomeContent string `json:"home_content" gorm:"type:text"`

	SeoTitle       string `json:"seo_title" gorm:"type:varchar(255)"`
	SeoDescription string `json:"seo_description" gorm:"type:text"`
	SeoKeywords    string `json:"seo_keywords" gorm:"type:varchar(500)"`

	VerifyToken string `json:"verify_token" gorm:"type:varchar(64)"`
	Verified    bool   `json:"verified" gorm:"index"`
	VerifiedAt  int64  `json:"verified_at" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (AgentDomain) TableName() string {
	return "agent_domains"
}

// GetVerifiedAgentDomain 按 Host 查已验证的白标域名，用于域名路由与注册归属。
func GetVerifiedAgentDomain(domain string) (*AgentDomain, error) {
	if domain == "" {
		return nil, errors.New("域名为空")
	}
	var row AgentDomain
	if err := DB.First(&row, "domain = ? AND verified = ?", domain, true).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListAgentDomains 取某代理名下的全部域名。
func ListAgentDomains(agentId int) ([]*AgentDomain, error) {
	var rows []*AgentDomain
	err := DB.Where("agent_id = ?", agentId).Order("id ASC").Find(&rows).Error
	return rows, err
}

// GetAgentDomainByName 按域名查绑定记录（含未验证的），不存在返回 (nil, nil)。
func GetAgentDomainByName(tx *gorm.DB, domain string) (*AgentDomain, error) {
	if tx == nil {
		tx = DB
	}
	var row AgentDomain
	err := tx.First(&row, "domain = ?", domain).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func GetAgentDomainById(id int) (*AgentDomain, error) {
	var row AgentDomain
	if err := DB.First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkAgentDomainVerified 标记域名归属校验通过，此后该域名才会生效。
func MarkAgentDomainVerified(id int) error {
	return DB.Model(&AgentDomain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"verified":    true,
		"verified_at": common.GetTimestamp(),
	}).Error
}

// UpdateAgentDomainBranding 更新白标品牌字段，不触碰归属与校验状态。
func UpdateAgentDomainBranding(id int, patch *AgentDomain) error {
	if patch == nil {
		return errors.New("品牌信息为空")
	}
	return DB.Model(&AgentDomain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"site_name":       patch.SiteName,
		"site_logo":       patch.SiteLogo,
		"brand_color":     patch.BrandColor,
		"custom_css":      patch.CustomCss,
		"home_content":    patch.HomeContent,
		"seo_title":       patch.SeoTitle,
		"seo_description": patch.SeoDescription,
		"seo_keywords":    patch.SeoKeywords,
	}).Error
}

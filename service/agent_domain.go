package service

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrAgentDomainInvalid  = errors.New("域名格式不合法")
	ErrAgentDomainTaken    = errors.New("该域名已被绑定")
	ErrAgentDomainNotMine  = errors.New("该域名不属于当前代理")
	ErrAgentDomainVerify   = errors.New("域名归属校验未通过")
	ErrAgentDomainNotAllow = errors.New("分销型代理没有独立站点，无法绑定域名")
)

// NormalizeAgentDomain 统一域名大小写与末尾点，并做基本格式校验。
func NormalizeAgentDomain(domain string) (string, error) {
	d := strings.TrimSpace(strings.ToLower(domain))
	d = strings.TrimSuffix(d, ".")
	if h, _, err := net.SplitHostPort(d); err == nil {
		d = h
	}
	if d == "" || len(d) > 255 || !strings.Contains(d, ".") ||
		strings.ContainsAny(d, " /\\:?#@") {
		return "", ErrAgentDomainInvalid
	}
	return d, nil
}

// BindAgentDomain 代理绑定一个白标域名。绑定后未验证不生效。
func BindAgentDomain(operatorUserId int, domain string) (*model.AgentDomain, error) {
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return nil, err
	}
	if agent.Status != model.AgentStatusActive {
		return nil, ErrAgentInactive
	}
	// 分销型代理不建站，给它绑域名没有意义，也会让归属判定出现两个来源。
	if agent.Type != model.AgentTypeReseller {
		return nil, ErrAgentDomainNotAllow
	}
	normalized, err := NormalizeAgentDomain(domain)
	if err != nil {
		return nil, err
	}

	row := &model.AgentDomain{
		AgentId:     agent.Id,
		Domain:      normalized,
		VerifyToken: "newapi-agent-verify=" + common.GetRandomString(24),
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		existing, err := model.GetAgentDomainByName(tx, normalized)
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrAgentDomainTaken
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: agent.Id, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionBindDomain, TargetType: "domain", TargetId: normalized,
		})
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// VerifyAgentDomain 校验域名归属：要求该域名的 TXT 记录里出现绑定时下发的令牌。
//
// 不校验就生效等于任何代理都能绑别人的域名，把别人的流量接到自己的站点上。
func VerifyAgentDomain(operatorUserId, domainId int) error {
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return err
	}
	row, err := model.GetAgentDomainById(domainId)
	if err != nil || row.AgentId != agent.Id {
		return ErrAgentDomainNotMine
	}
	if row.Verified {
		return nil
	}
	if !lookupDomainToken(row.Domain, row.VerifyToken) {
		return fmt.Errorf("%w：请先为 %s 添加 TXT 记录 %s", ErrAgentDomainVerify, row.Domain, row.VerifyToken)
	}
	if err := model.MarkAgentDomainVerified(row.Id); err != nil {
		return err
	}
	InvalidateAgentDomainCacheHook()
	return nil
}

func lookupDomainToken(domain, token string) bool {
	records, err := net.LookupTXT(domain)
	if err != nil {
		return false
	}
	for _, record := range records {
		if strings.TrimSpace(record) == token {
			return true
		}
	}
	return false
}

// UpdateAgentDomainBranding 更新白标品牌信息。
func UpdateAgentDomainBranding(operatorUserId, domainId int, patch *model.AgentDomain) error {
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return err
	}
	row, err := model.GetAgentDomainById(domainId)
	if err != nil || row.AgentId != agent.Id {
		return ErrAgentDomainNotMine
	}
	if err := model.UpdateAgentDomainBranding(row.Id, patch); err != nil {
		return err
	}
	InvalidateAgentDomainCacheHook()
	return nil
}

// UnbindAgentDomain 解绑域名。物理删除，审计留在 agent_audit_logs。
func UnbindAgentDomain(operatorUserId, domainId int) error {
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return err
	}
	row, err := model.GetAgentDomainById(domainId)
	if err != nil || row.AgentId != agent.Id {
		return ErrAgentDomainNotMine
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.AgentDomain{}, "id = ?", row.Id).Error; err != nil {
			return err
		}
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: agent.Id, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionUnbindDomain, TargetType: "domain", TargetId: row.Domain,
		})
	})
	if err != nil {
		return err
	}
	InvalidateAgentDomainCacheHook()
	return nil
}

// InvalidateAgentDomainCacheHook 由 middleware 注册，避免 service 反向依赖 middleware。
var InvalidateAgentDomainCacheHook = func() {}

// AgentSiteBranding 某个白标域名对外呈现的站点信息。
type AgentSiteBranding struct {
	AgentId        int    `json:"agent_id"`
	SiteName       string `json:"site_name"`
	SiteLogo       string `json:"site_logo"`
	BrandColor     string `json:"brand_color"`
	CustomCss      string `json:"custom_css"`
	HomeContent    string `json:"home_content"`
	SeoTitle       string `json:"seo_title"`
	SeoDescription string `json:"seo_description"`
	SeoKeywords    string `json:"seo_keywords"`
}

// GetAgentSiteBranding 取某代理的站点品牌，供 /api/status 按域名返回。
// 代理绑了多个域名时取当前 Host 命中的那个。
func GetAgentSiteBranding(agentId int, host string) (*AgentSiteBranding, bool) {
	if agentId <= 0 {
		return nil, false
	}
	normalized, err := NormalizeAgentDomain(host)
	if err != nil {
		return nil, false
	}
	row, err := model.GetVerifiedAgentDomain(normalized)
	if err != nil || row.AgentId != agentId {
		return nil, false
	}
	return &AgentSiteBranding{
		AgentId:        row.AgentId,
		SiteName:       row.SiteName,
		SiteLogo:       row.SiteLogo,
		BrandColor:     row.BrandColor,
		CustomCss:      row.CustomCss,
		HomeContent:    row.HomeContent,
		SeoTitle:       row.SeoTitle,
		SeoDescription: row.SeoDescription,
		SeoKeywords:    row.SeoKeywords,
	}, true
}

// ListOwnAgentDomains 返回当前代理名下的域名。
func ListOwnAgentDomains(operatorUserId int) ([]*model.AgentDomain, error) {
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return nil, err
	}
	return model.ListAgentDomains(agent.Id)
}

// AgentIdString 便于审计写入。
func AgentIdString(id int) string { return strconv.Itoa(id) }

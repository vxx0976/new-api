package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAgentDomain(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"Example.COM", "example.com", true},
		{" api.example.com. ", "api.example.com", true},
		{"api.example.com:8443", "api.example.com", true},
		{"localhost", "", false},
		{"", "", false},
		{"http://example.com", "", false},
		{"exa mple.com", "", false},
	} {
		got, err := NormalizeAgentDomain(tc.in)
		if !tc.ok {
			assert.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got)
	}
}

func TestBindAgentDomainRejectsDuplicateAndAffiliate(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6})

	row, err := BindAgentDomain(owners[0].Id, "Shop.Example.com")
	require.NoError(t, err)
	assert.Equal(t, "shop.example.com", row.Domain)
	assert.False(t, row.Verified, "刚绑定的域名必须是未验证状态")
	assert.NotEmpty(t, row.VerifyToken)

	// 同域名不能被两个代理绑走
	otherOwner := seedAgentUser(t, "other-domain-owner", agents[0].Id)
	other := seedAgentFor(t, otherOwner, agents[0], model.AgentTypeReseller)
	_, err = BindAgentDomain(otherOwner.Id, "shop.example.com")
	assert.ErrorIs(t, err, ErrAgentDomainTaken)

	// 分销型代理不建站
	affOwner := seedAgentUser(t, "aff-domain-owner", agents[0].Id)
	seedAgentFor(t, affOwner, agents[0], model.AgentTypeAffiliate)
	_, err = BindAgentDomain(affOwner.Id, "aff.example.com")
	assert.ErrorIs(t, err, ErrAgentDomainNotAllow)

	_ = other
}

// 未通过归属校验的域名不得生效，否则任何代理都能绑别人的域名把流量接走。
func TestUnverifiedDomainDoesNotResolve(t *testing.T) {
	resetAgentMoney(t)
	_, owners := buildAgentLadder(t, "default", []float64{0.6})

	row, err := BindAgentDomain(owners[0].Id, "pending.example.com")
	require.NoError(t, err)

	_, err = model.GetVerifiedAgentDomain("pending.example.com")
	assert.Error(t, err)

	require.NoError(t, model.MarkAgentDomainVerified(row.Id))
	got, err := model.GetVerifiedAgentDomain("pending.example.com")
	require.NoError(t, err)
	assert.Equal(t, row.AgentId, got.AgentId)
}

func TestAgentDomainOperationsRejectForeignDomain(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6})
	row, err := BindAgentDomain(owners[0].Id, "mine.example.com")
	require.NoError(t, err)

	intruderUser := seedAgentUser(t, "intruder", agents[0].Id)
	intruder := seedAgentFor(t, intruderUser, agents[0], model.AgentTypeReseller)

	assert.ErrorIs(t, VerifyAgentDomain(intruderUser.Id, row.Id), ErrAgentDomainNotMine)
	assert.ErrorIs(t, UnbindAgentDomain(intruderUser.Id, row.Id), ErrAgentDomainNotMine)
	assert.ErrorIs(t, UpdateAgentDomainBranding(intruderUser.Id, row.Id,
		&model.AgentDomain{SiteName: "hijacked"}), ErrAgentDomainNotMine)

	_ = intruder
}

func TestAgentSiteBrandingOnlyForMatchingHost(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6})
	row, err := BindAgentDomain(owners[0].Id, "brand.example.com")
	require.NoError(t, err)
	require.NoError(t, model.MarkAgentDomainVerified(row.Id))
	require.NoError(t, UpdateAgentDomainBranding(owners[0].Id, row.Id, &model.AgentDomain{
		SiteName: "品牌站", SiteLogo: "logo.png", HomeContent: "hello",
	}))

	branding, ok := GetAgentSiteBranding(agents[0].Id, "brand.example.com:443")
	require.True(t, ok)
	assert.Equal(t, "品牌站", branding.SiteName)
	assert.Equal(t, "hello", branding.HomeContent)

	_, ok = GetAgentSiteBranding(agents[0].Id, "unknown.example.com")
	assert.False(t, ok, "Host 没命中就不能套用该代理的品牌")

	_, ok = GetAgentSiteBranding(agents[0].Id+999, "brand.example.com")
	assert.False(t, ok, "域名与代理必须匹配，不能只凭 Host 就返回别人的品牌")
}

func TestUnbindAgentDomainFreesItForRebinding(t *testing.T) {
	resetAgentMoney(t)
	agents, owners := buildAgentLadder(t, "default", []float64{0.6})
	row, err := BindAgentDomain(owners[0].Id, "reuse.example.com")
	require.NoError(t, err)
	require.NoError(t, UnbindAgentDomain(owners[0].Id, row.Id))

	nextOwner := seedAgentUser(t, "next-domain-owner", agents[0].Id)
	seedAgentFor(t, nextOwner, agents[0], model.AgentTypeReseller)
	_, err = BindAgentDomain(nextOwner.Id, "reuse.example.com")
	assert.NoError(t, err, "解绑后域名应能立即被重新绑定")

	var audits []model.AgentAuditLog
	require.NoError(t, model.DB.Where("action = ?", model.AgentAuditActionUnbindDomain).Find(&audits).Error)
	assert.NotEmpty(t, audits, "解绑必须留痕")
}

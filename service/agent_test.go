package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetAgentTables(t *testing.T) {
	t.Helper()
	model.DB.Exec("DELETE FROM agents")
	model.DB.Exec("DELETE FROM agent_audit_logs")
	model.DB.Exec("DELETE FROM users")
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM agents")
		model.DB.Exec("DELETE FROM agent_audit_logs")
		model.DB.Exec("DELETE FROM users")
	})
}

func seedAgentUser(t *testing.T, name string, parentAgentId int) *model.User {
	t.Helper()
	user := &model.User{
		Username:    name,
		Password:    "password",
		DisplayName: name,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-" + name,
		AuthVersion: 1,
		// 归属边：这个用户挂在哪个代理名下，决定了谁有资格把他开成下级代理。
		ParentAgentId: parentAgentId,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func seedAgentFor(t *testing.T, user *model.User, parent *model.Agent, agentType string) *model.Agent {
	t.Helper()
	agent := &model.Agent{
		OwnerUserId:    user.Id,
		AgentPath:      model.BuildAgentPath(parent),
		Type:           agentType,
		Name:           user.Username,
		Status:         model.AgentStatusActive,
		PricingVersion: 1,
	}
	if parent != nil {
		agent.ParentAgentId = parent.Id
		agent.Level = parent.Level + 1
	}
	require.NoError(t, model.DB.Create(agent).Error)
	return agent
}

func TestCreateSubAgentRequiresCallerToBeAgent(t *testing.T) {
	resetAgentTables(t)
	caller := seedAgentUser(t, "plain-user", 0)
	target := seedAgentUser(t, "target-user", 0)

	_, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: caller.Id, TargetUserId: target.Id,
		Type: model.AgentTypeReseller, Name: "downstream",
	})
	assert.ErrorIs(t, err, ErrAgentNotFound, "普通用户不能凭空开出下级代理")
}

// 开通下级必须以归属边为准：候选人得已经在我名下，
// 否则任何代理只要猜到一个用户 id 就能把陌生人挂到自己树上。
func TestCreateSubAgentRejectsUserOutsideCallerScope(t *testing.T) {
	resetAgentTables(t)
	ownerA := seedAgentUser(t, "owner-a", 0)
	agentA := seedAgentFor(t, ownerA, nil, model.AgentTypeReseller)

	ownerB := seedAgentUser(t, "owner-b", 0)
	agentB := seedAgentFor(t, ownerB, nil, model.AgentTypeReseller)
	underB := seedAgentUser(t, "under-b", agentB.Id)
	platformUser := seedAgentUser(t, "platform-user", 0)

	_, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: ownerA.Id, TargetUserId: underB.Id,
		Type: model.AgentTypeReseller, Name: "stolen",
	})
	assert.ErrorIs(t, err, ErrAgentTargetNotMine, "不能把别家代理的用户开成自己的下级")

	_, err = CreateSubAgent(CreateAgentInput{
		OperatorUserId: ownerA.Id, TargetUserId: platformUser.Id,
		Type: model.AgentTypeReseller, Name: "stolen",
	})
	assert.ErrorIs(t, err, ErrAgentTargetNotMine, "平台直属用户也不属于任何代理")

	_ = agentA
}

func TestCreateSubAgentRejectsDuplicateOwner(t *testing.T) {
	resetAgentTables(t)
	ownerA := seedAgentUser(t, "owner-a", 0)
	agentA := seedAgentFor(t, ownerA, nil, model.AgentTypeReseller)

	child := seedAgentUser(t, "child", agentA.Id)
	seedAgentFor(t, child, agentA, model.AgentTypeReseller)

	_, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: ownerA.Id, TargetUserId: child.Id,
		Type: model.AgentTypeReseller, Name: "again",
	})
	assert.ErrorIs(t, err, ErrAgentOwnerTaken, "一个用户只能拥有一个代理，否则链路上溯会分叉")
}

// 分销型节点没有域名也没有定价权，它开出的 reseller 下级将没有进货价基准。
func TestAffiliateAgentCannotOpenResellerChild(t *testing.T) {
	resetAgentTables(t)
	owner := seedAgentUser(t, "affiliate-owner", 0)
	agent := seedAgentFor(t, owner, nil, model.AgentTypeAffiliate)
	target := seedAgentUser(t, "affiliate-target", agent.Id)

	_, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: owner.Id, TargetUserId: target.Id,
		Type: model.AgentTypeReseller, Name: "bad-child",
	})
	assert.ErrorIs(t, err, ErrAgentTypeNotAllowed)

	created, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: owner.Id, TargetUserId: target.Id,
		Type: model.AgentTypeAffiliate, Name: "good-child",
	})
	require.NoError(t, err, "分销线可以继续带分销下级")
	assert.Equal(t, model.AgentTypeAffiliate, created.Type)
}

func TestFrozenAgentCannotOpenChild(t *testing.T) {
	resetAgentTables(t)
	owner := seedAgentUser(t, "frozen-owner", 0)
	agent := seedAgentFor(t, owner, nil, model.AgentTypeReseller)
	require.NoError(t, model.DB.Model(agent).Update("status", model.AgentStatusFrozen).Error)
	target := seedAgentUser(t, "frozen-target", agent.Id)

	_, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: owner.Id, TargetUserId: target.Id,
		Type: model.AgentTypeReseller, Name: "child",
	})
	assert.ErrorIs(t, err, ErrAgentInactive, "冻结中的代理不得继续扩张下级")
}

func TestCreateSubAgentBuildsPathAndWritesAudit(t *testing.T) {
	resetAgentTables(t)
	rootOwner := seedAgentUser(t, "root-owner", 0)
	root := seedAgentFor(t, rootOwner, nil, model.AgentTypeReseller)

	midUser := seedAgentUser(t, "mid-user", root.Id)
	mid, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: rootOwner.Id, TargetUserId: midUser.Id,
		Type: model.AgentTypeReseller, Name: "mid", Ip: "10.0.0.1",
	})
	require.NoError(t, err)
	assert.Equal(t, root.Id, mid.ParentAgentId)
	assert.Equal(t, root.Level+1, mid.Level)
	assert.Equal(t, fmt.Sprintf("/%d/", root.Id), mid.AgentPath)

	leafUser := seedAgentUser(t, "leaf-user", mid.Id)
	leaf, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: midUser.Id, TargetUserId: leafUser.Id,
		Type: model.AgentTypeReseller, Name: "leaf",
	})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("/%d/%d/", root.Id, mid.Id), leaf.AgentPath)
	assert.Equal(t, []int{root.Id, mid.Id}, model.ParseAgentPath(leaf.AgentPath))

	var audits []model.AgentAuditLog
	require.NoError(t, model.DB.Where("action = ?", model.AgentAuditActionCreate).Find(&audits).Error)
	require.Len(t, audits, 2, "每次开通下级都必须留痕")
	assert.Equal(t, rootOwner.Id, audits[0].OperatorUserId)
	assert.Equal(t, "10.0.0.1", audits[0].Ip)
}

func TestCreatePlatformAgentRequiresPlatformDirectUser(t *testing.T) {
	resetAgentTables(t)
	rootOwner := seedAgentUser(t, "root-owner", 0)
	root := seedAgentFor(t, rootOwner, nil, model.AgentTypeReseller)
	underRoot := seedAgentUser(t, "under-root", root.Id)
	platformUser := seedAgentUser(t, "platform-user", 0)

	_, err := CreatePlatformAgent(CreateAgentInput{
		OperatorUserId: 1, TargetUserId: underRoot.Id,
		Type: model.AgentTypeReseller, Name: "wrong",
	})
	assert.ErrorIs(t, err, ErrAgentTargetNotMine, "已归属某代理的用户不能被开成平台直属代理")

	agent, err := CreatePlatformAgent(CreateAgentInput{
		OperatorUserId: 1, TargetUserId: platformUser.Id,
		Type: model.AgentTypeReseller, Name: "platform-direct",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, agent.ParentAgentId)
	assert.Equal(t, 0, agent.Level)
	assert.Equal(t, "/", agent.AgentPath)
}

func TestCreateSubAgentRejectsDisabledOrPrivilegedTarget(t *testing.T) {
	resetAgentTables(t)
	owner := seedAgentUser(t, "owner", 0)
	agent := seedAgentFor(t, owner, nil, model.AgentTypeReseller)

	disabled := seedAgentUser(t, "disabled", agent.Id)
	require.NoError(t, model.DB.Model(disabled).Update("status", common.UserStatusDisabled).Error)
	admin := seedAgentUser(t, "admin-user", agent.Id)
	require.NoError(t, model.DB.Model(admin).Update("role", common.RoleAdminUser).Error)

	_, err := CreateSubAgent(CreateAgentInput{
		OperatorUserId: owner.Id, TargetUserId: disabled.Id,
		Type: model.AgentTypeReseller, Name: "child",
	})
	assert.ErrorIs(t, err, ErrAgentTargetInvalid)

	_, err = CreateSubAgent(CreateAgentInput{
		OperatorUserId: owner.Id, TargetUserId: admin.Id,
		Type: model.AgentTypeReseller, Name: "child",
	})
	assert.ErrorIs(t, err, ErrAgentTargetInvalid, "管理员不应被挂到某个代理的树下")
}

// 可见性边界：ListChildAgents 只返回直接下级，下下级不得出现。
func TestListChildAgentsExcludesGrandchildren(t *testing.T) {
	resetAgentTables(t)
	rootOwner := seedAgentUser(t, "root-owner", 0)
	root := seedAgentFor(t, rootOwner, nil, model.AgentTypeReseller)

	midUser := seedAgentUser(t, "mid-user", root.Id)
	mid := seedAgentFor(t, midUser, root, model.AgentTypeReseller)
	leafUser := seedAgentUser(t, "leaf-user", mid.Id)
	leaf := seedAgentFor(t, leafUser, mid, model.AgentTypeReseller)

	_, children, err := ListChildAgents(rootOwner.Id)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, mid.Id, children[0].Id)
	for _, child := range children {
		assert.NotEqual(t, leaf.Id, child.Id, "上级不得看到下下级")
	}

	_, midChildren, err := ListChildAgents(midUser.Id)
	require.NoError(t, err)
	require.Len(t, midChildren, 1)
	assert.Equal(t, leaf.Id, midChildren[0].Id, "中间层看得到自己的直接下级")
}

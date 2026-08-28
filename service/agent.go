package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrAgentNotFound       = errors.New("当前账号不是代理")
	ErrAgentInactive       = errors.New("代理已被冻结，无法进行该操作")
	ErrAgentOwnerTaken     = errors.New("该用户已经是代理")
	ErrAgentTargetNotMine  = errors.New("只能把自己名下的用户开通为下级代理")
	ErrAgentTargetInvalid  = errors.New("目标用户不可用")
	ErrAgentTypeInvalid    = errors.New("代理类型不合法")
	ErrAgentTypeNotAllowed = errors.New("分销型代理只能开通分销型下级")
	ErrAgentNameRequired   = errors.New("代理名称不能为空")
)

const agentNameMaxLen = 100

// GetOwnedAgent 返回该用户拥有的代理。非代理返回 ErrAgentNotFound。
func GetOwnedAgent(userId int) (*model.Agent, error) {
	agent, err := model.GetAgentByOwnerUserId(userId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// ListChildAgents 返回调用者的直接下级。
//
// 这里刻意只取直接子集：一旦按 agent_path 前缀取子树，上级就能看到下下级，
// 而逐级批发模型的整个责任边界就建立在「每一级只认识自己的直接上下级」上。
func ListChildAgents(operatorUserId int) (*model.Agent, []*model.Agent, error) {
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return nil, nil, err
	}
	children, err := model.ListDirectChildAgents(agent.Id)
	if err != nil {
		return nil, nil, err
	}
	return agent, children, nil
}

type CreateAgentInput struct {
	OperatorUserId int
	TargetUserId   int
	Type           string
	Name           string
	Ip             string
}

// CreateSubAgent 由上级代理的拥有者开通一个直接下级。
//
// 鉴权是所有权式的而非角色式的：只有上级代理本人能开自己的下级，管理员走
// CreatePlatformAgent 开平台直属代理。这样代理树的每一条边都有明确的责任人。
func CreateSubAgent(in CreateAgentInput) (*model.Agent, error) {
	parent, err := GetOwnedAgent(in.OperatorUserId)
	if err != nil {
		return nil, err
	}
	if parent.Status != model.AgentStatusActive {
		return nil, ErrAgentInactive
	}
	// 分销型代理没有域名也没有定价权，它开不出一个有定价权的下级——
	// 那个下级的进货价将无从计算。分销线只能继续分销。
	if parent.Type == model.AgentTypeAffiliate && in.Type != model.AgentTypeAffiliate {
		return nil, ErrAgentTypeNotAllowed
	}
	return createAgent(in, parent)
}

// CreatePlatformAgent 由管理员开通平台直属代理（ParentAgentId = 0）。
func CreatePlatformAgent(in CreateAgentInput) (*model.Agent, error) {
	return createAgent(in, nil)
}

func createAgent(in CreateAgentInput, parent *model.Agent) (*model.Agent, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrAgentNameRequired
	}
	if len([]rune(name)) > agentNameMaxLen {
		return nil, fmt.Errorf("代理名称不能超过 %d 个字符", agentNameMaxLen)
	}
	if !model.IsValidAgentType(in.Type) {
		return nil, ErrAgentTypeInvalid
	}
	if err := model.ValidateAgentPlacement(0, parent); err != nil {
		return nil, err
	}

	target, err := model.GetUserById(in.TargetUserId, false)
	if err != nil {
		return nil, ErrAgentTargetInvalid
	}
	if target.Status != common.UserStatusEnabled || target.Role != common.RoleCommonUser {
		return nil, ErrAgentTargetInvalid
	}
	// 归属边决定谁能把谁开成下级：候选人必须已经在我名下，
	// 否则任何代理都能凭一个用户 id 把陌生人挂到自己树上。
	expectedParentAgentId := 0
	if parent != nil {
		expectedParentAgentId = parent.Id
	}
	if target.ParentAgentId != expectedParentAgentId {
		return nil, ErrAgentTargetNotMine
	}

	agent := &model.Agent{
		OwnerUserId:    target.Id,
		AgentPath:      model.BuildAgentPath(parent),
		Type:           in.Type,
		Name:           name,
		Status:         model.AgentStatusActive,
		PricingVersion: 1,
	}
	if parent != nil {
		agent.ParentAgentId = parent.Id
		agent.Level = parent.Level + 1
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		taken, err := model.AgentOwnerTaken(tx, target.Id)
		if err != nil {
			return err
		}
		if taken {
			return ErrAgentOwnerTaken
		}
		if err := model.CreateAgentWithTx(tx, agent); err != nil {
			return err
		}
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId:        agent.Id,
			OperatorUserId: in.OperatorUserId,
			Action:         model.AgentAuditActionCreate,
			TargetType:     "user",
			TargetId:       fmt.Sprintf("%d", target.Id),
			NewValue:       fmt.Sprintf("type=%s parent_agent_id=%d level=%d", agent.Type, agent.ParentAgentId, agent.Level),
			Ip:             in.Ip,
		})
	})
	if err != nil {
		return nil, err
	}
	// 建树改变了链路结构，定价缓存必须失效，否则新代理的下级会在 TTL 内继续按旧链路计费。
	InvalidateAgentPricingCache()
	return agent, nil
}

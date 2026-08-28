package service

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrAgentPromoteNotFrozen = errors.New("只有被冻结的代理才能做上提接管")
	ErrAgentAlreadyFrozen    = errors.New("代理已处于冻结状态")
)

// FreezeAgentSubtree 冻结一个代理及其整棵子树。
//
// 冻结的语义是「停止服务与计提，保留全部数据」：新请求不再按该链路计价、
// 分润不再计提，余额和账本原样保留。不做删除，也不自动把下级上提——
// 上提会改变可见性边界（上级会突然看到原本的下下级），必须由人显式发起。
func FreezeAgentSubtree(operatorUserId, agentId int, note string) (int, error) {
	agent, err := model.GetAgentById(agentId)
	if err != nil {
		return 0, ErrAgentNotFound
	}
	if agent.Status == model.AgentStatusFrozen {
		return 0, ErrAgentAlreadyFrozen
	}
	affected := 0
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		n, err := model.SetAgentSubtreeStatus(tx, agent, model.AgentStatusFrozen)
		if err != nil {
			return err
		}
		affected = n
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: agent.Id, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionFreeze, TargetType: "agent",
			TargetId: strconv.Itoa(agent.Id),
			NewValue: fmt.Sprintf("frozen subtree count=%d", n),
			Note:     note,
		})
	})
	if err != nil {
		return 0, err
	}
	InvalidateAgentPricingCache()
	return affected, nil
}

// UnfreezeAgentSubtree 解冻一个代理及其整棵子树。
func UnfreezeAgentSubtree(operatorUserId, agentId int, note string) (int, error) {
	agent, err := model.GetAgentById(agentId)
	if err != nil {
		return 0, ErrAgentNotFound
	}
	affected := 0
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		n, err := model.SetAgentSubtreeStatus(tx, agent, model.AgentStatusActive)
		if err != nil {
			return err
		}
		affected = n
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: agent.Id, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionUnfreeze, TargetType: "agent",
			TargetId: strconv.Itoa(agent.Id),
			NewValue: fmt.Sprintf("active subtree count=%d", n), Note: note,
		})
	})
	if err != nil {
		return 0, err
	}
	InvalidateAgentPricingCache()
	return affected, nil
}

// PromoteFrozenAgentChildren 把被冻结代理的直接下级上提到它的上级名下。
//
// 这是接管操作，会让上级看到原本不可见的下下级——可见性边界的变更必须显式发起并留痕，
// 不能作为冻结的副作用自动发生。下级的进货价保持原值不动，避免接管瞬间价格跳变。
func PromoteFrozenAgentChildren(operatorUserId, agentId int, note string) (int, error) {
	frozen, err := model.GetAgentById(agentId)
	if err != nil {
		return 0, ErrAgentNotFound
	}
	if frozen.Status != model.AgentStatusFrozen {
		return 0, ErrAgentPromoteNotFrozen
	}
	children, err := model.ListDirectChildAgents(frozen.Id)
	if err != nil {
		return 0, err
	}
	if len(children) == 0 {
		return 0, nil
	}

	var newParent *model.Agent
	if frozen.ParentAgentId > 0 {
		newParent, err = model.GetAgentById(frozen.ParentAgentId)
		if err != nil {
			return 0, err
		}
	}

	promoted := 0
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		for _, child := range children {
			if err := model.ReparentAgentSubtree(tx, child, newParent); err != nil {
				return err
			}
			promoted++
			if err := model.RecordAgentAudit(tx, &model.AgentAuditLog{
				AgentId: child.Id, OperatorUserId: operatorUserId,
				Action: model.AgentAuditActionPromote, TargetType: "agent",
				TargetId: strconv.Itoa(child.Id),
				OldValue: fmt.Sprintf("parent=%d", frozen.Id),
				NewValue: fmt.Sprintf("parent=%d", frozen.ParentAgentId),
				Note:     note,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	InvalidateAgentPricingCache()
	return promoted, nil
}

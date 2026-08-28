package service

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var (
	ErrAgentWithdrawAmountInvalid = errors.New("提现额度必须大于 0")
	ErrAgentWithdrawTooSmall      = errors.New("低于最小提现额度")
	ErrAgentWithdrawInsufficient  = errors.New("可提现余额不足")
	ErrAgentWithdrawNotFound      = errors.New("提现单不存在")
	ErrAgentWithdrawStateInvalid  = errors.New("提现单当前状态不允许该操作")
)

// CreateAgentWithdraw 代理发起提现。
//
// 提现是代理与平台之间的事，上级无权审批下级的提现单——否则上级可以靠卡钱拿捏下级。
// 额度从 Agent.EarningQuota 扣，与 users.quota（消费余额）完全隔离。
func CreateAgentWithdraw(operatorUserId, quota int, payeeInfo string) (*model.AgentWithdrawRequest, error) {
	if quota <= 0 {
		return nil, ErrAgentWithdrawAmountInvalid
	}
	if min := operation_setting.GetAgentSetting().MinWithdrawQuota; min > 0 && quota < min {
		return nil, fmt.Errorf("%w（最小 %d）", ErrAgentWithdrawTooSmall, min)
	}
	agent, err := GetOwnedAgent(operatorUserId)
	if err != nil {
		return nil, err
	}
	if agent.Status != model.AgentStatusActive {
		return nil, ErrAgentInactive
	}

	req := &model.AgentWithdrawRequest{
		AgentId:     agent.Id,
		OwnerUserId: agent.OwnerUserId,
		Quota:       quota,
		Status:      model.AgentWithdrawStatusPending,
		PayeeInfo:   payeeInfo,
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 先锁住代理行再校验余额，否则并发提现能把同一笔钱申请两次。
		locked, err := model.LockAgentForUpdate(tx, agent.Id)
		if err != nil {
			return err
		}
		if locked.EarningQuota < quota {
			return ErrAgentWithdrawInsufficient
		}
		if err := tx.Create(req).Error; err != nil {
			return err
		}
		// 申请即冻结额度：不先扣的话，审核期间代理可以重复申请把余额提空。
		if err := model.AddAgentEarningQuota(tx, agent.Id, -quota); err != nil {
			return err
		}
		if err := tx.Create(&model.AgentLedger{
			AgentId: agent.Id, OwnerUserId: agent.OwnerUserId,
			Direction: model.AgentLedgerDirectionDebit, Amount: quota,
			BalanceAfter: locked.EarningQuota - quota,
			Source:       model.AgentLedgerSourceWithdraw,
			RefType:      "withdraw", RefId: strconv.Itoa(req.Id),
			IdempotencyKey: "withdraw:" + strconv.Itoa(req.Id),
		}).Error; err != nil {
			return err
		}
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: agent.Id, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionWithdrawAudit, TargetType: "withdraw",
			TargetId: strconv.Itoa(req.Id), NewValue: "pending " + strconv.Itoa(quota),
		})
	})
	if err != nil {
		return nil, err
	}
	return req, nil
}

// ReviewAgentWithdraw 管理员审核提现单。驳回时把冻结的额度退回代理钱包。
func ReviewAgentWithdraw(operatorUserId, withdrawId int, approve bool, note, paymentRef string) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		req, err := model.LockAgentWithdrawForUpdate(tx, withdrawId)
		if err != nil {
			return ErrAgentWithdrawNotFound
		}
		if req.Status != model.AgentWithdrawStatusPending {
			return ErrAgentWithdrawStateInvalid
		}

		updates := map[string]interface{}{
			"reviewer_user_id": operatorUserId,
			"review_note":      note,
			"reviewed_at":      common.GetTimestamp(),
		}
		if approve {
			updates["status"] = model.AgentWithdrawStatusPaid
			updates["paid_at"] = common.GetTimestamp()
			updates["payment_ref"] = paymentRef
		} else {
			updates["status"] = model.AgentWithdrawStatusRejected
			if err := model.AddAgentEarningQuota(tx, req.AgentId, req.Quota); err != nil {
				return err
			}
			locked, err := model.LockAgentForUpdate(tx, req.AgentId)
			if err != nil {
				return err
			}
			if err := tx.Create(&model.AgentLedger{
				AgentId: req.AgentId, OwnerUserId: req.OwnerUserId,
				Direction: model.AgentLedgerDirectionCredit, Amount: req.Quota,
				BalanceAfter: locked.EarningQuota,
				Source:       model.AgentLedgerSourceWithdrawRefund,
				RefType:      "withdraw", RefId: strconv.Itoa(req.Id),
				IdempotencyKey: "withdraw-refund:" + strconv.Itoa(req.Id),
			}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.AgentWithdrawRequest{}).
			Where("id = ?", req.Id).Updates(updates).Error; err != nil {
			return err
		}
		if approve {
			if err := model.AddAgentWithdrawnQuota(tx, req.AgentId, req.Quota); err != nil {
				return err
			}
		}
		return model.RecordAgentAudit(tx, &model.AgentAuditLog{
			AgentId: req.AgentId, OperatorUserId: operatorUserId,
			Action: model.AgentAuditActionWithdrawAudit, TargetType: "withdraw",
			TargetId: strconv.Itoa(req.Id),
			OldValue: "pending",
			NewValue: map[bool]string{true: "paid", false: "rejected"}[approve],
			Note:     note,
		})
	})
}

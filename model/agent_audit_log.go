package model

import (
	"errors"

	"gorm.io/gorm"
)

// 代理体系的高危操作。改价、冻结、上提接管都直接影响资金与可见性边界，必须留痕。
const (
	AgentAuditActionCreate        = "create"         // 开通下级
	AgentAuditActionSetChildCost  = "set_child_cost" // 给下级设进货价
	AgentAuditActionSetSell       = "set_sell"       // 设自己的售价
	AgentAuditActionSetRebateRate = "set_rebate"     // 给分销型下级设返佣比例
	AgentAuditActionFreeze        = "freeze"         // 冻结（含子树）
	AgentAuditActionUnfreeze      = "unfreeze"
	AgentAuditActionPromote       = "promote"       // 上提接管：把被冻结代理的下级挂到其上级名下
	AgentAuditActionBindDomain    = "bind_domain"   // 绑定白标域名
	AgentAuditActionUnbindDomain  = "unbind_domain" // 解绑白标域名
	AgentAuditActionWithdrawAudit = "withdraw_audit"
)

// AgentAuditLog 代理体系操作审计。
//
// 上提接管会让上级突然看到原本不可见的下下级，属于可见性边界的变更，
// 这类操作必须能被事后追溯到具体操作人。
type AgentAuditLog struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId        int    `json:"agent_id" gorm:"not null;index:idx_agent_audit_agent,priority:1"` // 被操作的代理
	OperatorUserId int    `json:"operator_user_id" gorm:"type:int;default:0;index"`
	Action         string `json:"action" gorm:"type:varchar(40);not null;index"`
	TargetType     string `json:"target_type" gorm:"type:varchar(40)"`
	TargetId       string `json:"target_id" gorm:"type:varchar(64)"`
	OldValue       string `json:"old_value" gorm:"type:text"`
	NewValue       string `json:"new_value" gorm:"type:text"`
	Ip             string `json:"ip" gorm:"type:varchar(64)"`
	Note           string `json:"note" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index:idx_agent_audit_agent,priority:2"`
}

func (AgentAuditLog) TableName() string {
	return "agent_audit_logs"
}

// RecordAgentAudit 写一条代理审计。必须与被审计的操作在同一事务内，
// 否则操作成功而审计丢失，事后无法追溯谁改了价、谁接管了谁。
func RecordAgentAudit(tx *gorm.DB, log *AgentAuditLog) error {
	if log == nil {
		return errors.New("审计记录为空")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Create(log).Error
}

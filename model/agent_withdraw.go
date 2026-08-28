package model

import "gorm.io/gorm"

// 提现单状态。提现是代理与平台之间的事，上级无权审批下级的提现单。
const (
	AgentWithdrawStatusPending  = 1 // 待审核
	AgentWithdrawStatusApproved = 2 // 已通过，待打款
	AgentWithdrawStatusPaid     = 3 // 已打款
	AgentWithdrawStatusRejected = 4 // 已驳回，额度已退回
)

// AgentWithdrawRequest 代理分润提现申请。
// 可提现金额来自 Agent.EarningQuota，与 users.quota（消费余额）互不相通。
type AgentWithdrawRequest struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId     int    `json:"agent_id" gorm:"not null;index:idx_agent_withdraw_agent,priority:1"`
	OwnerUserId int    `json:"owner_user_id" gorm:"not null;index"`
	Quota       int    `json:"quota" gorm:"type:int;not null"`
	Status      int    `json:"status" gorm:"type:int;default:1;index"`
	PayeeInfo   string `json:"payee_info" gorm:"type:text"` // 收款信息，按需脱敏后展示

	ReviewerUserId int    `json:"reviewer_user_id" gorm:"type:int;default:0"`
	ReviewNote     string `json:"review_note" gorm:"type:text"`
	ReviewedAt     int64  `json:"reviewed_at" gorm:"type:bigint;default:0"`
	PaidAt         int64  `json:"paid_at" gorm:"type:bigint;default:0"`
	PaymentRef     string `json:"payment_ref" gorm:"type:varchar(128)"`

	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at;index:idx_agent_withdraw_agent,priority:2"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (AgentWithdrawRequest) TableName() string {
	return "agent_withdraw_requests"
}

// LockAgentWithdrawForUpdate 加锁读取提现单，防止并发审核把同一单处理两次。
func LockAgentWithdrawForUpdate(tx *gorm.DB, id int) (*AgentWithdrawRequest, error) {
	if tx == nil {
		tx = DB
	}
	var req AgentWithdrawRequest
	if err := lockForUpdate(tx).First(&req, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

// ListAgentWithdraws 分页取某代理的提现单。
func ListAgentWithdraws(agentId, offset, limit int) ([]*AgentWithdrawRequest, error) {
	var rows []*AgentWithdrawRequest
	err := DB.Where("agent_id = ?", agentId).
		Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, err
}

// ListPendingAgentWithdraws 管理员侧的待审核提现单。
func ListPendingAgentWithdraws(offset, limit int) ([]*AgentWithdrawRequest, error) {
	var rows []*AgentWithdrawRequest
	err := DB.Where("status = ?", AgentWithdrawStatusPending).
		Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, err
}

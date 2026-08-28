package model

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 分润来源。
const (
	AgentEarningSourceTierMarkup      = "tier_markup"      // 逐级批发差价
	AgentEarningSourceAffiliateRebate = "affiliate_rebate" // 分销返佣
	AgentLedgerSourceWithdraw         = "withdraw"
	AgentLedgerSourceWithdrawRefund   = "withdraw_refund"
)

const (
	AgentLedgerDirectionCredit = "credit"
	AgentLedgerDirectionDebit  = "debit"
)

// AgentEarningsOutbox 消费分润的聚合入账队列。
//
// 一笔消费沿链路自下而上逐级计提，每级一行；差价为 0 的层直接跳过不写。
// 同一 (代理, 客户, 时间窗) 只有一行，靠 upsert 累加 Amount，避免深链路下每请求写多条
// 造成写放大。CreditedQuota 记录已入账部分，worker 每次只补差额——这样一个仍在累加的
// 窗口也能安全地被反复结算，不会重复发钱，也不会因为先结算过就漏掉后续增量。
//
// 金额为正由写入侧保证，不下沉为数据库 CHECK：计费路径上的约束失败会回滚整笔计费。
type AgentEarningsOutbox struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId        int    `json:"agent_id" gorm:"not null;index:idx_agent_outbox_pending,priority:2"`
	FromUserId     int    `json:"from_user_id" gorm:"type:int;default:0;index"` // 产生这笔分润的终端用户
	Amount         int    `json:"amount" gorm:"type:int;not null"`
	CreditedQuota  int    `json:"credited_quota" gorm:"type:int;default:0"`
	Source         string `json:"source" gorm:"type:varchar(40);not null"`
	RefType        string `json:"ref_type" gorm:"type:varchar(40)"`
	RefId          string `json:"ref_id" gorm:"type:varchar(64)"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index:idx_agent_outbox_pending,priority:1"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (AgentEarningsOutbox) TableName() string {
	return "agent_earnings_outbox"
}

// AgentLedger 代理分润钱包的资金流水，永久保留，是提现与对账的唯一依据。
// 记的是 Agent.EarningQuota 的变动，与 users.quota（消费余额）无关。
type AgentLedger struct {
	Id                 int    `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId            int    `json:"agent_id" gorm:"not null;index:idx_agent_ledger_agent,priority:1"`
	OwnerUserId        int    `json:"owner_user_id" gorm:"not null;index"`
	CounterpartyUserId int    `json:"counterparty_user_id" gorm:"type:int;default:0"`
	Direction          string `json:"direction" gorm:"type:varchar(10);not null"`
	Amount             int    `json:"amount" gorm:"type:int;not null"`
	BalanceAfter       int    `json:"balance_after" gorm:"type:int;default:0"`

	// 高频消费分润按 (代理, 客户, 时间窗) 聚合入账，否则深链路下每请求写多条会造成写放大。
	IsAggregated    bool `json:"is_aggregated"`
	AggregatedCount int  `json:"aggregated_count" gorm:"type:int;default:0"`

	Source         string `json:"source" gorm:"type:varchar(40);not null"`
	RefType        string `json:"ref_type" gorm:"type:varchar(40)"`
	RefId          string `json:"ref_id" gorm:"type:varchar(64)"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex"`
	Note           string `json:"note" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index:idx_agent_ledger_agent,priority:2"`
}

func (AgentLedger) TableName() string {
	return "agent_ledger"
}

// AccumulateAgentEarnings 把一笔分润累加进当前时间窗的聚合行。
func AccumulateAgentEarnings(row *AgentEarningsOutbox) error {
	if row == nil || row.Amount <= 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "idempotency_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"amount":     gorm.Expr("agent_earnings_outbox.amount + ?", row.Amount),
			"updated_at": common.GetTimestamp(),
		}),
	}).Create(row).Error
}

// ListUncreditedAgentEarnings 取还有未入账增量的聚合行，按 agent_id 升序，
// 让并发结算按同一顺序加锁，避免多级链路下互相死锁。
func ListUncreditedAgentEarnings(limit int) ([]*AgentEarningsOutbox, error) {
	var rows []*AgentEarningsOutbox
	err := DB.Where("credited_quota < amount").
		Order("agent_id ASC, id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

// CreditAgentEarnings 结算一行聚合分润的未入账增量。
//
// 用 credited_quota 的 CAS 抢占：多个结算者并发时只有一个能把 credited_quota 从旧值
// 推到新值，其余的 rowsAffected 为 0 直接跳过，因此同一笔增量不会被发两次。
// 返回实际入账的额度，0 表示没抢到或无增量。
func CreditAgentEarnings(row *AgentEarningsOutbox, ownerUserId int) (int, error) {
	if row == nil {
		return 0, errors.New("分润记录为空")
	}
	delta := row.Amount - row.CreditedQuota
	if delta <= 0 {
		return 0, nil
	}

	credited := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AgentEarningsOutbox{}).
			Where("id = ? AND credited_quota = ?", row.Id, row.CreditedQuota).
			Updates(map[string]interface{}{
				"credited_quota": row.CreditedQuota + delta,
				"updated_at":     common.GetTimestamp(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}

		if err := tx.Model(&Agent{}).Where("id = ?", row.AgentId).Updates(map[string]interface{}{
			"earning_quota":         gorm.Expr("earning_quota + ?", delta),
			"history_earning_quota": gorm.Expr("history_earning_quota + ?", delta),
		}).Error; err != nil {
			return err
		}

		var balanceAfter int
		if err := tx.Model(&Agent{}).Select("earning_quota").
			Where("id = ?", row.AgentId).Scan(&balanceAfter).Error; err != nil {
			return err
		}

		if err := tx.Create(&AgentLedger{
			AgentId:            row.AgentId,
			OwnerUserId:        ownerUserId,
			CounterpartyUserId: row.FromUserId,
			Direction:          AgentLedgerDirectionCredit,
			Amount:             delta,
			BalanceAfter:       balanceAfter,
			IsAggregated:       true,
			Source:             row.Source,
			RefType:            row.RefType,
			RefId:              row.RefId,
			IdempotencyKey:     "credit:" + row.IdempotencyKey + ":" + strconv.Itoa(row.CreditedQuota+delta),
		}).Error; err != nil {
			return err
		}
		credited = delta
		return nil
	})
	return credited, err
}

// SumAgentSettledEarnings 汇总某代理已入账的分润总额，用于对账。
func SumAgentSettledEarnings(agentId int) (int, error) {
	var total int
	err := DB.Model(&AgentLedger{}).
		Where("agent_id = ? AND direction = ?", agentId, AgentLedgerDirectionCredit).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	return total, err
}

// ListAgentLedger 分页取某代理的资金流水。
func ListAgentLedger(agentId, offset, limit int) ([]*AgentLedger, error) {
	var rows []*AgentLedger
	err := DB.Where("agent_id = ?", agentId).
		Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, err
}

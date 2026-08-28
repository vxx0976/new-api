package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentGroupCost 代理在某分组上的进货倍率，由其【上级】配置（平台直属代理由管理员配置），
// 代理本人只读。
//
// 「我卖给下级代理的批发价」不单独存储——就是写下级的这张表。一个价只有一份存储，
// 不存两份就不会出现上级调价后下级进货价没跟着变的对账问题。
type AgentGroupCost struct {
	Id       int     `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId  int     `json:"agent_id" gorm:"not null;uniqueIndex:idx_agent_group_cost,priority:1"`
	Group    string  `json:"group" gorm:"type:varchar(64);not null;column:group_name;uniqueIndex:idx_agent_group_cost,priority:2"`
	CostRate float64 `json:"cost_rate" gorm:"type:decimal(10,6);not null"`

	// 上级抬价必须延迟生效并通知下级，否则下级会在毫无预警的情况下开始倒贴。
	// PendingRate 为 0 表示没有待生效的调价。
	// decimal 列不加 default 标签：MySQL 会把 0 规范化为 0.000000 而 PostgreSQL 记作 0，
	// AutoMigrate 每次启动都会判定不一致并重复发 ALTER TABLE。零值由 Go 侧保证。
	PendingRate        float64 `json:"pending_rate" gorm:"type:decimal(10,6)"`
	PendingEffectiveAt int64   `json:"pending_effective_at" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (AgentGroupCost) TableName() string {
	return "agent_group_costs"
}

// AgentGroupSell 代理卖给【终端用户】的绝对售价倍率，由代理本人配置。
// 定价权是代理体系的产品卖点，上级不得代下级设置。
type AgentGroupSell struct {
	Id        int     `json:"id" gorm:"primaryKey;autoIncrement"`
	AgentId   int     `json:"agent_id" gorm:"not null;uniqueIndex:idx_agent_group_sell,priority:1"`
	Group     string  `json:"group" gorm:"type:varchar(64);not null;column:group_name;uniqueIndex:idx_agent_group_sell,priority:2"`
	SellRate  float64 `json:"sell_rate" gorm:"type:decimal(10,6);not null"`
	CreatedAt int64   `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64   `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (AgentGroupSell) TableName() string {
	return "agent_group_sells"
}

// GetAgentGroupCosts 取某代理全部分组的进货价，按分组名索引。
func GetAgentGroupCosts(agentId int) (map[string]*AgentGroupCost, error) {
	result := make(map[string]*AgentGroupCost)
	if agentId <= 0 {
		return result, errors.New("代理 id 为空")
	}
	var rows []*AgentGroupCost
	if err := DB.Where("agent_id = ?", agentId).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Group] = row
	}
	return result, nil
}

// GetAgentGroupSells 取某代理全部分组的售价，按分组名索引。
func GetAgentGroupSells(agentId int) (map[string]*AgentGroupSell, error) {
	result := make(map[string]*AgentGroupSell)
	if agentId <= 0 {
		return result, errors.New("代理 id 为空")
	}
	var rows []*AgentGroupSell
	if err := DB.Where("agent_id = ?", agentId).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Group] = row
	}
	return result, nil
}

// GetAgentGroupCostForUpdate 加锁取某代理某分组的进货价，不存在返回 (nil, nil)。
func GetAgentGroupCostForUpdate(tx *gorm.DB, agentId int, group string) (*AgentGroupCost, error) {
	if tx == nil {
		tx = DB
	}
	var row AgentGroupCost
	err := lockForUpdate(tx).Where("agent_id = ? AND group_name = ?", agentId, group).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetAgentGroupSellForUpdate 加锁取某代理某分组的售价，不存在返回 (nil, nil)。
func GetAgentGroupSellForUpdate(tx *gorm.DB, agentId int, group string) (*AgentGroupSell, error) {
	if tx == nil {
		tx = DB
	}
	var row AgentGroupSell
	err := lockForUpdate(tx).Where("agent_id = ? AND group_name = ?", agentId, group).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpsertAgentGroupCost 写入进货价并清掉待生效的调价。
func UpsertAgentGroupCost(tx *gorm.DB, agentId int, group string, rate float64) error {
	if tx == nil {
		tx = DB
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "agent_id"}, {Name: "group_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"cost_rate":            rate,
			"pending_rate":         0,
			"pending_effective_at": 0,
			"updated_at":           common.GetTimestamp(),
		}),
	}).Create(&AgentGroupCost{AgentId: agentId, Group: group, CostRate: rate}).Error
}

// SetAgentGroupCostPending 登记一次延迟生效的调价，当前价保持不变。
func SetAgentGroupCostPending(tx *gorm.DB, agentId int, group string, rate float64, effectiveAt int64) error {
	if tx == nil {
		tx = DB
	}
	return tx.Model(&AgentGroupCost{}).
		Where("agent_id = ? AND group_name = ?", agentId, group).
		Updates(map[string]interface{}{
			"pending_rate":         rate,
			"pending_effective_at": effectiveAt,
			"updated_at":           common.GetTimestamp(),
		}).Error
}

// ListDueAgentGroupCosts 取所有到期待生效的调价。
func ListDueAgentGroupCosts(now int64) ([]*AgentGroupCost, error) {
	var rows []*AgentGroupCost
	err := DB.Where("pending_effective_at > 0 AND pending_effective_at <= ?", now).
		Order("id ASC").Find(&rows).Error
	return rows, err
}

// ApplyAgentGroupCostPending 把待生效价落为当前价。
func ApplyAgentGroupCostPending(tx *gorm.DB, id int, rate float64) error {
	if tx == nil {
		tx = DB
	}
	return tx.Model(&AgentGroupCost{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"cost_rate":            rate,
			"pending_rate":         0,
			"pending_effective_at": 0,
			"updated_at":           common.GetTimestamp(),
		}).Error
}

// UpsertAgentGroupSell 写入代理对终端用户的售价。
func UpsertAgentGroupSell(tx *gorm.DB, agentId int, group string, rate float64) error {
	if tx == nil {
		tx = DB
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "agent_id"}, {Name: "group_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"sell_rate":  rate,
			"updated_at": common.GetTimestamp(),
		}),
	}).Create(&AgentGroupSell{AgentId: agentId, Group: group, SellRate: rate}).Error
}

// GetAgentGroupCostsByAgentIds 一次取回整条链路上各级的进货价，
// 供定价解析器展开链路快照时使用，避免逐级查询。
func GetAgentGroupCostsByAgentIds(agentIds []int, group string) (map[int]*AgentGroupCost, error) {
	result := make(map[int]*AgentGroupCost, len(agentIds))
	if len(agentIds) == 0 {
		return result, nil
	}
	var rows []*AgentGroupCost
	if err := DB.Where("agent_id IN ? AND group_name = ?", agentIds, group).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.AgentId] = row
	}
	return result, nil
}

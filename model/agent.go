package model

import (
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// 代理状态。代理持有资金账本，任何情况下都不做删除，只在这三个状态间流转。
const (
	AgentStatusActive    = 1 // 正常
	AgentStatusFrozen    = 2 // 冻结：拒绝新请求，账本保留，停止计提分润
	AgentStatusSuspended = 3 // 终止：已结清，仅留档
)

// 代理类型，二者互斥。
const (
	AgentTypeReseller  = "reseller"  // 有自己的域名和定价权，赚进货价与售价的差价
	AgentTypeAffiliate = "affiliate" // 无站点，把客户引到上级站点，按比例从上级差价里抽佣
)

// AgentMaxDepth 是 agent_path 的存储护栏，不是业务层数上限。
// agent_path 为 varchar(512)，单层最长 11 字节（10 位 id + 分隔符），40 层留有余量。
const AgentMaxDepth = 40

var (
	ErrAgentPathTooDeep = errors.New("代理层级超过上限")
	ErrAgentPathCycle   = errors.New("代理层级出现环")
)

// Agent 代理（租户）。平台直属代理 ParentAgentId = 0，AgentPath = "/"。
//
// 命名说明：本仓库的 MerchantID 指支付网关商户号（见 setting.WaffoPancakeMerchantID），
// 与本体系无关，因此代理体系一律使用 agent 而非 merchant。
type Agent struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	OwnerUserId   int    `json:"owner_user_id" gorm:"not null;uniqueIndex"`
	ParentAgentId int    `json:"parent_agent_id" gorm:"type:int;default:0;index"`
	AgentPath     string `json:"agent_path" gorm:"type:varchar(512);index"` // 根→父，形如 "/1/7/"，平台直属为 "/"
	Level         int    `json:"level" gorm:"type:int;default:0"`
	Type          string `json:"type" gorm:"type:varchar(16);index"`
	Name          string `json:"name" gorm:"type:varchar(100)"`
	Status        int    `json:"status" gorm:"type:int;default:1;index"`

	// PricingVersion 参与定价缓存的 key。上级改价时按子树整体 +1，
	// 使整条链路的缓存一次性失效——只失效被改的那一个代理会让下级继续按旧成本计费。
	PricingVersion int64 `json:"pricing_version" gorm:"type:bigint;default:1"`

	// 分润收入独立于 users.quota：users.quota 只做消费余额（不可提现），
	// EarningQuota 才是可提现的分润收入，二者不混用同一个数。
	EarningQuota        int `json:"earning_quota" gorm:"type:int;default:0"`         // 当前可提现余额
	HistoryEarningQuota int `json:"history_earning_quota" gorm:"type:int;default:0"` // 累计分润收入
	WithdrawnQuota      int `json:"withdrawn_quota" gorm:"type:int;default:0"`       // 累计已提现

	// RebateRatePercent 仅对 AgentTypeAffiliate 生效：本节点从其引来的客户消费中可抽取的比例。
	// 由【上级】配置（与进货价同理，一个价只存一份），默认 0 即不返佣。
	// decimal 列不加 default 标签，理由见 AgentGroupCost.PendingRate。
	RebateRatePercent float64 `json:"rebate_rate_percent" gorm:"type:decimal(7,4)"`

	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (Agent) TableName() string {
	return "agents"
}

func IsValidAgentType(t string) bool {
	return t == AgentTypeReseller || t == AgentTypeAffiliate
}

// BuildAgentPath 由父代理推出子代理的 agent_path。父为 nil 表示平台直属。
func BuildAgentPath(parent *Agent) string {
	if parent == nil {
		return "/"
	}
	return parent.AgentPath + strconv.Itoa(parent.Id) + "/"
}

// ParseAgentPath 解析 agent_path 为祖先 id 列表，根在前、父在后。
func ParseAgentPath(path string) []int {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// AgentPathContains 判断 agent_path 上是否已存在某个代理，用于建树时防环。
func AgentPathContains(path string, agentId int) bool {
	if agentId <= 0 || path == "" {
		return false
	}
	return strings.Contains(path, "/"+strconv.Itoa(agentId)+"/")
}

// AgentSubtreePrefix 返回匹配某代理整棵子树的 LIKE 前缀。
//
// 仅限平台/管理员使用。代理侧的任何查询都必须按 parent_agent_id 取直接子集，
// 一旦用了子树前缀，上级就能看到下下级，违反可见性边界。
func AgentSubtreePrefix(agent *Agent) string {
	if agent == nil {
		return ""
	}
	return agent.AgentPath + strconv.Itoa(agent.Id) + "/%"
}

// ValidateAgentPlacement 校验一个代理挂到指定父节点下是否合法（防环 + 深度护栏）。
// agentId 为 0 表示新建。
func ValidateAgentPlacement(agentId int, parent *Agent) error {
	path := BuildAgentPath(parent)
	if len(ParseAgentPath(path)) >= AgentMaxDepth {
		return ErrAgentPathTooDeep
	}
	if agentId > 0 && AgentPathContains(path, agentId) {
		return ErrAgentPathCycle
	}
	return nil
}

func GetAgentById(id int) (*Agent, error) {
	if id <= 0 {
		return nil, errors.New("代理 id 为空")
	}
	var agent Agent
	if err := DB.First(&agent, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func GetAgentByOwnerUserId(userId int) (*Agent, error) {
	if userId <= 0 {
		return nil, errors.New("用户 id 为空")
	}
	var agent Agent
	if err := DB.First(&agent, "owner_user_id = ?", userId).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgentsByIds 一次取回整条链路上的代理，避免逐级查询。
func GetAgentsByIds(ids []int) (map[int]*Agent, error) {
	result := make(map[int]*Agent, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var agents []*Agent
	if err := DB.Where("id IN ?", ids).Find(&agents).Error; err != nil {
		return nil, err
	}
	for _, agent := range agents {
		result[agent.Id] = agent
	}
	return result, nil
}

// GetAgentOwnerUsernames 批量取代理拥有者的用户名，避免下级列表逐行查库。
func GetAgentOwnerUsernames(ownerUserIds []int) (map[int]string, error) {
	result := make(map[int]string, len(ownerUserIds))
	if len(ownerUserIds) == 0 {
		return result, nil
	}
	var rows []struct {
		Id       int
		Username string
	}
	err := DB.Model(&User{}).Select("id", "username").Where("id IN ?", ownerUserIds).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Id] = row.Username
	}
	return result, nil
}

// ListDirectChildAgents 取直接下级。代理侧接口只能用这个。
func ListDirectChildAgents(parentAgentId int) ([]*Agent, error) {
	var agents []*Agent
	err := DB.Where("parent_agent_id = ?", parentAgentId).Order("id ASC").Find(&agents).Error
	return agents, err
}

// ListSubtreeAgentsAdminOnly 取整棵子树。仅平台/管理员可调，代理侧禁止使用。
func ListSubtreeAgentsAdminOnly(agent *Agent) ([]*Agent, error) {
	if agent == nil {
		return nil, errors.New("代理为空")
	}
	var agents []*Agent
	err := DB.Where("id = ? OR agent_path LIKE ?", agent.Id, AgentSubtreePrefix(agent)).
		Order("id ASC").Find(&agents).Error
	return agents, err
}

// CreateAgentWithTx 落库一个新代理。调用方必须先跑 ValidateAgentPlacement，
// 并在同一事务内写审计。
func CreateAgentWithTx(tx *gorm.DB, agent *Agent) error {
	if agent == nil {
		return errors.New("代理为空")
	}
	if agent.OwnerUserId <= 0 {
		return errors.New("代理拥有者为空")
	}
	if !IsValidAgentType(agent.Type) {
		return errors.New("代理类型不合法")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Create(agent).Error
}

// AgentOwnerTaken 判断某用户是否已经是某个代理的拥有者。
// owner_user_id 上有唯一索引兜底，这里只是为了把撞库错误换成可读的提示。
func AgentOwnerTaken(tx *gorm.DB, userId int) (bool, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&Agent{}).Where("owner_user_id = ?", userId).Count(&count).Error
	return count > 0, err
}

// LockAgentForUpdate 加锁读取代理，用于余额校验等需要串行化的场景。
func LockAgentForUpdate(tx *gorm.DB, agentId int) (*Agent, error) {
	if tx == nil {
		tx = DB
	}
	var agent Agent
	if err := lockForUpdate(tx).First(&agent, "id = ?", agentId).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

// AddAgentEarningQuota 调整代理的可提现分润余额，delta 可为负。
func AddAgentEarningQuota(tx *gorm.DB, agentId int, delta int) error {
	if tx == nil {
		tx = DB
	}
	return tx.Model(&Agent{}).Where("id = ?", agentId).
		Update("earning_quota", gorm.Expr("earning_quota + ?", delta)).Error
}

// AddAgentWithdrawnQuota 累加代理的已提现总额。
func AddAgentWithdrawnQuota(tx *gorm.DB, agentId int, delta int) error {
	if tx == nil {
		tx = DB
	}
	return tx.Model(&Agent{}).Where("id = ?", agentId).
		Update("withdrawn_quota", gorm.Expr("withdrawn_quota + ?", delta)).Error
}

// SetAgentSubtreeStatus 批量设置某代理及其整棵子树的状态，返回受影响行数。
func SetAgentSubtreeStatus(tx *gorm.DB, agent *Agent, status int) (int, error) {
	if agent == nil {
		return 0, errors.New("代理为空")
	}
	if tx == nil {
		tx = DB
	}
	res := tx.Model(&Agent{}).
		Where("id = ? OR agent_path LIKE ?", agent.Id, AgentSubtreePrefix(agent)).
		Updates(map[string]interface{}{
			"status":          status,
			"pricing_version": gorm.Expr("pricing_version + ?", 1),
		})
	return int(res.RowsAffected), res.Error
}

// ReparentAgentSubtree 把一个代理连同其整棵子树挂到新的父节点下。
// newParent 为 nil 表示挂到平台直属。
func ReparentAgentSubtree(tx *gorm.DB, agent *Agent, newParent *Agent) error {
	if agent == nil {
		return errors.New("代理为空")
	}
	if tx == nil {
		tx = DB
	}
	if err := ValidateAgentPlacement(agent.Id, newParent); err != nil {
		return err
	}

	oldPrefix := agent.AgentPath + strconv.Itoa(agent.Id) + "/"
	newPath := BuildAgentPath(newParent)
	newPrefix := newPath + strconv.Itoa(agent.Id) + "/"
	newParentId, newLevel := 0, 0
	if newParent != nil {
		newParentId = newParent.Id
		newLevel = newParent.Level + 1
	}
	levelDelta := newLevel - agent.Level

	// 先搬子树：子树的 path 都以 oldPrefix 开头，整体替换成 newPrefix。
	var descendants []*Agent
	if err := tx.Where("agent_path LIKE ?", oldPrefix+"%").Find(&descendants).Error; err != nil {
		return err
	}
	for _, node := range descendants {
		if err := tx.Model(&Agent{}).Where("id = ?", node.Id).Updates(map[string]interface{}{
			"agent_path":      newPrefix + strings.TrimPrefix(node.AgentPath, oldPrefix),
			"level":           node.Level + levelDelta,
			"pricing_version": gorm.Expr("pricing_version + ?", 1),
		}).Error; err != nil {
			return err
		}
	}

	return tx.Model(&Agent{}).Where("id = ?", agent.Id).Updates(map[string]interface{}{
		"parent_agent_id": newParentId,
		"agent_path":      newPath,
		"level":           newLevel,
		"pricing_version": gorm.Expr("pricing_version + ?", 1),
	}).Error
}

// BumpSubtreePricingVersion 在改价后使整棵子树的定价缓存失效。
func BumpSubtreePricingVersion(tx *gorm.DB, agent *Agent) error {
	if agent == nil {
		return errors.New("代理为空")
	}
	if tx == nil {
		tx = DB
	}
	return tx.Model(&Agent{}).
		Where("id = ? OR agent_path LIKE ?", agent.Id, AgentSubtreePrefix(agent)).
		Update("pricing_version", gorm.Expr("pricing_version + ?", 1)).Error
}

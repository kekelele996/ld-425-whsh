package model

import "time"

// MaterialUsage 节点用料登记记录：记录某批材料用在哪道施工节点。
// 生命周期 Registered（已登记）→ 验收通过 Confirmed（计入已安装量）；
// 一次验收不通过转为 PendingConfirm（待确认，不参与余量计算）。
type MaterialUsage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProjectID  uint      `gorm:"not null;index:idx_usage_project" json:"project_id"`
	NodeID     uint      `gorm:"not null;uniqueIndex:uk_usage_node_material,priority:1;index:idx_usage_node" json:"node_id"`
	MaterialID uint      `gorm:"not null;uniqueIndex:uk_usage_node_material,priority:2" json:"material_id"`
	Quantity   float64   `gorm:"type:decimal(12,2);not null;default:0" json:"quantity"`
	Status     string    `gorm:"size:32;not null;index" json:"status"`
	// ClientKey 由客户端在一次登记会话内生成，用于同一记录重复提交的幂等去重；
	// 可为 NULL，NULL 不参与唯一约束冲突。
	ClientKey *string   `gorm:"size:64;uniqueIndex:idx_usage_client_key" json:"client_key,omitempty"`
	Note      string    `gorm:"size:200" json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (MaterialUsage) TableName() string { return "material_usages" }

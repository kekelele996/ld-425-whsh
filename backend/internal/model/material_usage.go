package model

import "time"

// MaterialUsage 节点用料登记：把材料清单与施工节点关联起来。
// 验收前为 Pending（已登记待计入），验收通过转为 Counted（计入材料已安装量），
// 验收不通过转为 Excluded（待确认，不参与余量计算）。
type MaterialUsage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProjectID  uint      `gorm:"not null;index" json:"project_id"`
	NodeID     uint      `gorm:"not null;uniqueIndex:uk_node_material,priority:1;index" json:"node_id"`
	MaterialID uint      `gorm:"not null;uniqueIndex:uk_node_material,priority:2;index" json:"material_id"`
	Quantity   float64   `gorm:"type:decimal(12,2);not null;default:0" json:"quantity"`
	Status     string    `gorm:"size:32;not null;index" json:"status"`
	Remark     string    `gorm:"size:255" json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (MaterialUsage) TableName() string { return "material_usages" }

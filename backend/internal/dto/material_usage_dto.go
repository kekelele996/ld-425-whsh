package dto

import "time"

// RegisterMaterialUsageRequest 节点完工前登记用料请求。
type RegisterMaterialUsageRequest struct {
	NodeID     uint    `json:"node_id" binding:"required"`
	MaterialID uint    `json:"material_id" binding:"required"`
	Quantity   float64 `json:"quantity" binding:"required,gt=0"`
	Remark     string  `json:"remark" binding:"max=255"`
}

// MaterialUsageDTO 节点用料展示结构。
type MaterialUsageDTO struct {
	ID           uint      `json:"id"`
	ProjectID    uint      `json:"project_id"`
	NodeID       uint      `json:"node_id"`
	MaterialID   uint      `json:"material_id"`
	MaterialName string    `json:"material_name"`
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"`
	Status       string    `json:"status"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

package dto

import "time"

// RegisterMaterialUsageRequest 节点用料登记请求。
type RegisterMaterialUsageRequest struct {
	MaterialID uint    `json:"material_id" binding:"required"`
	Quantity   float64 `json:"quantity" binding:"required,gt=0"`
	// ClientKey 幂等键：客户端在一次登记会话内生成，同一记录重复提交时返回第一次结果。
	ClientKey string `json:"client_key" binding:"omitempty,max=64"`
	Note      string `json:"note" binding:"max=200"`
}

// MaterialUsageDTO 节点用料登记展示结构。
type MaterialUsageDTO struct {
	ID           uint      `json:"id"`
	ProjectID    uint      `json:"project_id"`
	NodeID       uint      `json:"node_id"`
	MaterialID   uint      `json:"material_id"`
	MaterialName string    `json:"material_name"`
	MaterialUnit string    `json:"material_unit"`
	Quantity     float64   `json:"quantity"`
	Status       string    `json:"status"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

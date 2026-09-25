package repository

import (
	"errors"
	"fmt"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

// MaterialUsageRepository 节点用料仓储接口。
type MaterialUsageRepository interface {
	Create(usage *model.MaterialUsage) error
	GetByNodeAndMaterial(nodeID, materialID uint) (*model.MaterialUsage, error)
	ListByNodeID(nodeID uint) ([]model.MaterialUsage, error)
	ListByProjectID(projectID uint) ([]model.MaterialUsage, error)
	// UpdateStatusByNode 批量更新某节点下全部用料记录的状态。
	UpdateStatusByNode(nodeID uint, status string) error
	// SumQuantityByMaterials 按材料汇总指定状态集合的用料数量。
	SumQuantityByMaterials(projectID uint, materialIDs []uint, statuses []string) (map[uint]float64, error)
}

type materialUsageRepository struct {
	db *gorm.DB
}

// NewMaterialUsageRepository 构造节点用料仓储。
func NewMaterialUsageRepository(db *gorm.DB) MaterialUsageRepository {
	return &materialUsageRepository{db: db}
}

func (r *materialUsageRepository) Create(usage *model.MaterialUsage) error {
	if err := r.db.Create(usage).Error; err != nil {
		return fmt.Errorf("create material usage: %w", err)
	}
	return nil
}

func (r *materialUsageRepository) GetByNodeAndMaterial(nodeID, materialID uint) (*model.MaterialUsage, error) {
	var usage model.MaterialUsage
	err := r.db.Where("node_id = ? AND material_id = ?", nodeID, materialID).First(&usage).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get material usage node=%d material=%d: %w", nodeID, materialID, err)
	}
	return &usage, nil
}

func (r *materialUsageRepository) ListByNodeID(nodeID uint) ([]model.MaterialUsage, error) {
	var usages []model.MaterialUsage
	if err := r.db.Where("node_id = ?", nodeID).Order("id ASC").Find(&usages).Error; err != nil {
		return nil, fmt.Errorf("list material usages by node %d: %w", nodeID, err)
	}
	return usages, nil
}

func (r *materialUsageRepository) ListByProjectID(projectID uint) ([]model.MaterialUsage, error) {
	var usages []model.MaterialUsage
	if err := r.db.Where("project_id = ?", projectID).Order("id ASC").Find(&usages).Error; err != nil {
		return nil, fmt.Errorf("list material usages by project %d: %w", projectID, err)
	}
	return usages, nil
}

func (r *materialUsageRepository) UpdateStatusByNode(nodeID uint, status string) error {
	if err := r.db.Model(&model.MaterialUsage{}).
		Where("node_id = ?", nodeID).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("update material usage status by node %d: %w", nodeID, err)
	}
	return nil
}

func (r *materialUsageRepository) SumQuantityByMaterials(projectID uint, materialIDs []uint, statuses []string) (map[uint]float64, error) {
	result := make(map[uint]float64)
	if len(materialIDs) == 0 {
		return result, nil
	}
	type row struct {
		MaterialID uint
		Total      float64
	}
	var rows []row
	query := r.db.Model(&model.MaterialUsage{}).
		Select("material_id, COALESCE(SUM(quantity), 0) AS total").
		Where("material_id IN ?", materialIDs).
		Group("material_id")
	if projectID != 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("sum material usage quantities: %w", err)
	}
	for _, item := range rows {
		result[item.MaterialID] = item.Total
	}
	return result, nil
}

package repository

import (
	"errors"
	"fmt"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

// MaterialUsageRepository 节点用料登记仓储接口。
type MaterialUsageRepository interface {
	Create(usage *model.MaterialUsage) error
	GetByID(id uint) (*model.MaterialUsage, error)
	GetByClientKey(clientKey string) (*model.MaterialUsage, error)
	GetByNodeAndMaterial(nodeID, materialID uint) (*model.MaterialUsage, error)
	ListByNodeID(nodeID uint) ([]model.MaterialUsage, error)
	ListByProjectID(projectID uint) ([]model.MaterialUsage, error)
	// SumConfirmedByMaterial 汇总单个材料已确认（验收通过）的用料量，即已安装量。
	SumConfirmedByMaterial(materialID uint) (float64, error)
	// SumConfirmedByMaterials 按材料分组汇总已确认用料量。
	SumConfirmedByMaterials(materialIDs []uint) (map[uint]float64, error)
	CountByMaterialID(materialID uint) (int64, error)
	CountByNodeID(nodeID uint) (int64, error)
	Update(usage *model.MaterialUsage) error
	Delete(id uint) error
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

func (r *materialUsageRepository) GetByID(id uint) (*model.MaterialUsage, error) {
	var usage model.MaterialUsage
	if err := r.db.First(&usage, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get material usage %d: %w", id, err)
	}
	return &usage, nil
}

func (r *materialUsageRepository) GetByClientKey(clientKey string) (*model.MaterialUsage, error) {
	var usage model.MaterialUsage
	if err := r.db.Where("client_key = ?", clientKey).First(&usage).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get material usage by client key: %w", err)
	}
	return &usage, nil
}

func (r *materialUsageRepository) GetByNodeAndMaterial(nodeID, materialID uint) (*model.MaterialUsage, error) {
	var usage model.MaterialUsage
	if err := r.db.Where("node_id = ? AND material_id = ?", nodeID, materialID).First(&usage).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get material usage by node %d material %d: %w", nodeID, materialID, err)
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

func (r *materialUsageRepository) SumConfirmedByMaterial(materialID uint) (float64, error) {
	var total float64
	err := r.db.Model(&model.MaterialUsage{}).
		Where("material_id = ? AND status = ?", materialID, "Confirmed").
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("sum confirmed usage by material %d: %w", materialID, err)
	}
	return total, nil
}

func (r *materialUsageRepository) SumConfirmedByMaterials(materialIDs []uint) (map[uint]float64, error) {
	if len(materialIDs) == 0 {
		return map[uint]float64{}, nil
	}
	type row struct {
		MaterialID uint
		Total      float64
	}
	var rows []row
	err := r.db.Model(&model.MaterialUsage{}).
		Select("material_id, COALESCE(SUM(quantity), 0) AS total").
		Where("material_id IN ? AND status = ?", materialIDs, "Confirmed").
		Group("material_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("sum confirmed usages: %w", err)
	}
	out := make(map[uint]float64, len(rows))
	for _, item := range rows {
		out[item.MaterialID] = item.Total
	}
	return out, nil
}

func (r *materialUsageRepository) CountByMaterialID(materialID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.MaterialUsage{}).Where("material_id = ?", materialID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count usages by material %d: %w", materialID, err)
	}
	return count, nil
}

func (r *materialUsageRepository) CountByNodeID(nodeID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.MaterialUsage{}).Where("node_id = ?", nodeID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count usages by node %d: %w", nodeID, err)
	}
	return count, nil
}

func (r *materialUsageRepository) Update(usage *model.MaterialUsage) error {
	if err := r.db.Save(usage).Error; err != nil {
		return fmt.Errorf("update material usage %d: %w", usage.ID, err)
	}
	return nil
}

func (r *materialUsageRepository) Delete(id uint) error {
	if err := r.db.Delete(&model.MaterialUsage{}, id).Error; err != nil {
		return fmt.Errorf("delete material usage %d: %w", id, err)
	}
	return nil
}

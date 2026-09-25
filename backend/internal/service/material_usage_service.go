package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
)

// UsageDetail 节点用料明细（附带材料展示信息）。
type UsageDetail struct {
	Usage        model.MaterialUsage
	MaterialName string
	Unit         string
}

// MaterialUsageService 节点用料服务接口。
type MaterialUsageService interface {
	Register(req *dto.RegisterMaterialUsageRequest) (*UsageDetail, error)
	ListDetailsByNode(nodeID uint) ([]UsageDetail, error)
}

type materialUsageService struct {
	usageRepo    repository.MaterialUsageRepository
	nodeRepo     repository.ConstructionRepository
	materialRepo repository.MaterialRepository
	logger       *slog.Logger
}

// NewMaterialUsageService 构造节点用料服务。
func NewMaterialUsageService(
	usageRepo repository.MaterialUsageRepository,
	nodeRepo repository.ConstructionRepository,
	materialRepo repository.MaterialRepository,
	logger *slog.Logger,
) MaterialUsageService {
	return &materialUsageService{usageRepo: usageRepo, nodeRepo: nodeRepo, materialRepo: materialRepo, logger: logger}
}

func (s *materialUsageService) Register(req *dto.RegisterMaterialUsageRequest) (*UsageDetail, error) {
	node, err := s.nodeRepo.GetByID(req.NodeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("construction node not found")
		}
		return nil, fmt.Errorf("get construction node: %w", err)
	}
	material, err := s.materialRepo.GetByID(req.MaterialID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("material item not found")
		}
		return nil, fmt.Errorf("get material item: %w", err)
	}

	// 节点完工前才能登记用料，已完工（待验收/已验收）节点不允许再补登。
	if node.Status == constants.ConstructionStatusCompleted {
		return nil, apperrors.NewConflict("material usage must be registered before the node is completed")
	}
	if material.ProjectID != node.ProjectID {
		return nil, apperrors.NewBadRequest("material and construction node must belong to the same project")
	}

	// 幂等：同一节点 + 同一材料重复提交时保留第一次结果，不重复计数。
	if existing, err := s.usageRepo.GetByNodeAndMaterial(req.NodeID, req.MaterialID); err == nil {
		s.logger.Info("duplicate material usage submission ignored", "node_id", req.NodeID, "material_id", req.MaterialID, "existing_id", existing.ID)
		return s.toDetail(existing), nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check existing material usage: %w", err)
	}

	// 累计已登记（待计入 + 已计入）不能超过采购量；待确认记录已被排除、不占用余量。
	reserved, err := s.usageRepo.SumQuantityByMaterials(node.ProjectID, []uint{req.MaterialID},
		[]string{constants.MaterialUsageStatusPending, constants.MaterialUsageStatusCounted})
	if err != nil {
		return nil, err
	}
	if used := reserved[req.MaterialID] + req.Quantity; used > material.Quantity+quantityEpsilon {
		return nil, apperrors.NewConflict(fmt.Sprintf(
			"total registered quantity %.2f exceeds purchased quantity %.2f", used, material.Quantity))
	}

	usage := &model.MaterialUsage{
		ProjectID:  node.ProjectID,
		NodeID:     req.NodeID,
		MaterialID: req.MaterialID,
		Quantity:   req.Quantity,
		Status:     constants.MaterialUsageStatusPending,
		Remark:     req.Remark,
	}
	if err := s.usageRepo.Create(usage); err != nil {
		if isDuplicateKeyError(err) {
			// 并发下唯一索引兜底：返回首次写入的记录。
			existing, getErr := s.usageRepo.GetByNodeAndMaterial(req.NodeID, req.MaterialID)
			if getErr != nil {
				return nil, fmt.Errorf("load material usage after duplicate key: %w", getErr)
			}
			s.logger.Info("duplicate material usage submission ignored on unique index", "node_id", req.NodeID, "material_id", req.MaterialID)
			return s.toDetail(existing), nil
		}
		return nil, fmt.Errorf("register material usage: %w", err)
	}
	s.logger.Info("material usage registered", "node_id", req.NodeID, "material_id", req.MaterialID, "quantity", req.Quantity)
	return s.toDetail(usage), nil
}

func (s *materialUsageService) ListDetailsByNode(nodeID uint) ([]UsageDetail, error) {
	usages, err := s.usageRepo.ListByNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("list material usages by node: %w", err)
	}
	details := make([]UsageDetail, 0, len(usages))
	for i := range usages {
		details = append(details, *s.toDetail(&usages[i]))
	}
	return details, nil
}

// toDetail 补全材料名称与单位；材料被删除时优雅降级为空值。
func (s *materialUsageService) toDetail(usage *model.MaterialUsage) *UsageDetail {
	detail := &UsageDetail{Usage: *usage}
	if material, err := s.materialRepo.GetByID(usage.MaterialID); err == nil {
		detail.MaterialName = material.Name
		detail.Unit = material.Unit
	}
	return detail
}

// isDuplicateKeyError 识别 MySQL / SQLite 的唯一约束冲突错误。
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate entry") || strings.Contains(message, "unique constraint")
}

// quantityEpsilon 数量比较容差，规避十进制浮点误差。
const quantityEpsilon = 0.000001

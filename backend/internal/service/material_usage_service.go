package service

import (
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"gorm.io/gorm"
)

// quantityEpsilon 数量比较容差，避免 decimal 浮点累计误差。
const quantityEpsilon = 1e-6

// MaterialUsageService 节点用料登记服务接口。
type MaterialUsageService interface {
	Register(nodeID uint, req *dto.RegisterMaterialUsageRequest) (*model.MaterialUsage, error)
	GetByID(id uint) (*model.MaterialUsage, error)
	ListByNode(nodeID uint) ([]model.MaterialUsage, error)
	ListByProject(projectID uint) ([]model.MaterialUsage, error)
	Delete(id uint) error
	// WithTx 返回绑定到指定事务的服务副本，保证施工验收与用料计数原子提交。
	WithTx(tx *gorm.DB) MaterialUsageService
	// ConfirmNodeUsages 验收通过：把节点下已登记用料计入已安装量；
	// 任一材料累计已安装量超过采购量则返回冲突错误，由事务回滚。
	ConfirmNodeUsages(nodeID uint) error
	// MarkNodeUsagesPendingConfirm 验收不通过：节点下已登记用料转为待确认，不参与余量计算。
	MarkNodeUsagesPendingConfirm(nodeID uint) error
}

type materialUsageService struct {
	usageRepo    repository.MaterialUsageRepository
	materialRepo repository.MaterialRepository
	nodeRepo     repository.ConstructionRepository
	logger       *slog.Logger
}

// NewMaterialUsageService 构造节点用料登记服务。
func NewMaterialUsageService(
	usageRepo repository.MaterialUsageRepository,
	materialRepo repository.MaterialRepository,
	nodeRepo repository.ConstructionRepository,
	logger *slog.Logger,
) MaterialUsageService {
	return &materialUsageService{usageRepo: usageRepo, materialRepo: materialRepo, nodeRepo: nodeRepo, logger: logger}
}

func (s *materialUsageService) WithTx(tx *gorm.DB) MaterialUsageService {
	return &materialUsageService{
		usageRepo:    repository.NewMaterialUsageRepository(tx),
		materialRepo: repository.NewMaterialRepository(tx),
		nodeRepo:     repository.NewConstructionRepository(tx),
		logger:       s.logger,
	}
}

func (s *materialUsageService) Register(nodeID uint, req *dto.RegisterMaterialUsageRequest) (*model.MaterialUsage, error) {
	// 幂等：同一记录重复提交（client_key 相同）时保留第一次结果，不重复计数。
	if req.ClientKey != "" {
		existing, err := s.usageRepo.GetByClientKey(req.ClientKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("check usage client key: %w", err)
		}
	}

	node, err := s.nodeRepo.GetByID(nodeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("construction node not found")
		}
		return nil, fmt.Errorf("get construction node: %w", err)
	}
	// 节点完工前登记本次用料；已完工但验收不通过的节点允许返工补登。
	if node.Status == constants.ConstructionStatusCompleted && node.AcceptanceStatus != constants.AcceptanceStatusFailed {
		return nil, apperrors.NewConflict("node already completed, register usage before node completion")
	}

	material, err := s.materialRepo.GetByID(req.MaterialID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("material item not found")
		}
		return nil, fmt.Errorf("get material item: %w", err)
	}
	if material.ProjectID != node.ProjectID {
		return nil, apperrors.NewBadRequest("material does not belong to the node's project")
	}
	// 材料到货后才能登记安装用料。
	if material.PurchaseStatus != constants.PurchaseStatusDelivered && material.PurchaseStatus != constants.PurchaseStatusInstalled {
		return nil, apperrors.NewConflict("material not delivered yet, cannot register usage")
	}
	// 同一节点同一材料只保留一条登记记录。
	dup, err := s.usageRepo.GetByNodeAndMaterial(nodeID, req.MaterialID)
	if err == nil {
		return dup, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check existing usage: %w", err)
	}

	usage := &model.MaterialUsage{
		ProjectID:  node.ProjectID,
		NodeID:     nodeID,
		MaterialID: req.MaterialID,
		Quantity:   req.Quantity,
		Status:     constants.UsageStatusRegistered,
		Note:       req.Note,
	}
	if req.ClientKey != "" {
		usage.ClientKey = &req.ClientKey
	}
	if err := s.usageRepo.Create(usage); err != nil {
		// 并发下由唯一索引兜底：client_key 冲突时返回第一次的记录。
		if req.ClientKey != "" {
			if existing, gerr := s.usageRepo.GetByClientKey(req.ClientKey); gerr == nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("register material usage: %w", err)
	}
	s.logger.Info("material usage registered", "node_id", nodeID, "material_id", req.MaterialID, "quantity", req.Quantity)
	return usage, nil
}

func (s *materialUsageService) GetByID(id uint) (*model.MaterialUsage, error) {
	usage, err := s.usageRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("material usage not found")
		}
		return nil, fmt.Errorf("get material usage: %w", err)
	}
	return usage, nil
}

func (s *materialUsageService) ListByNode(nodeID uint) ([]model.MaterialUsage, error) {
	usages, err := s.usageRepo.ListByNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("list material usages by node: %w", err)
	}
	return usages, nil
}

func (s *materialUsageService) ListByProject(projectID uint) ([]model.MaterialUsage, error) {
	usages, err := s.usageRepo.ListByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("list material usages by project: %w", err)
	}
	return usages, nil
}

func (s *materialUsageService) Delete(id uint) error {
	usage, err := s.GetByID(id)
	if err != nil {
		return err
	}
	if usage.Status == constants.UsageStatusConfirmed {
		return apperrors.NewConflict("confirmed usage cannot be deleted")
	}
	node, err := s.nodeRepo.GetByID(usage.NodeID)
	if err != nil {
		return fmt.Errorf("get construction node: %w", err)
	}
	if node.Status == constants.ConstructionStatusCompleted && node.AcceptanceStatus != constants.AcceptanceStatusFailed {
		return apperrors.NewConflict("node already completed, cannot remove usage")
	}
	if err := s.usageRepo.Delete(id); err != nil {
		return fmt.Errorf("delete material usage: %w", err)
	}
	return nil
}

func (s *materialUsageService) ConfirmNodeUsages(nodeID uint) error {
	usages, err := s.usageRepo.ListByNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("list material usages by node: %w", err)
	}
	// 已累计的已安装量在事务内逐条累加，避免多次查询竞态。
	running := map[uint]float64{}
	for i := range usages {
		usage := &usages[i]
		if usage.Status == constants.UsageStatusConfirmed {
			// 已确认的记录（幂等重放）保持不动，但参与本次采购量上限校验的基数。
			running[usage.MaterialID] += usage.Quantity
			continue
		}
		if usage.Status != constants.UsageStatusRegistered && usage.Status != constants.UsageStatusPendingConfirm {
			continue
		}

		material, err := s.materialRepo.GetByID(usage.MaterialID)
		if err != nil {
			return fmt.Errorf("get material item: %w", err)
		}
		// 上限校验以库内已确认量为基础，叠加本次待确认的各条登记。
		installed, err := s.usageRepo.SumConfirmedByMaterial(usage.MaterialID)
		if err != nil {
			return fmt.Errorf("sum installed quantity: %w", err)
		}
		base := math.Max(installed, running[usage.MaterialID])
		if base+usage.Quantity > material.Quantity+quantityEpsilon {
			return apperrors.NewConflict(
				fmt.Sprintf("材料「%s」累计已安装量将超过采购量（采购 %.2f，已安装 %.2f，本次 %.2f）",
					material.Name, material.Quantity, base, usage.Quantity),
			)
		}

		usage.Status = constants.UsageStatusConfirmed
		if err := s.usageRepo.Update(usage); err != nil {
			return fmt.Errorf("confirm material usage: %w", err)
		}
		running[usage.MaterialID] = base + usage.Quantity

		// 全部安装完成后，采购状态自动推进到 Installed。
		if material.PurchaseStatus != constants.PurchaseStatusInstalled &&
			running[usage.MaterialID] >= material.Quantity-quantityEpsilon {
			material.PurchaseStatus = constants.PurchaseStatusInstalled
			if err := s.materialRepo.Update(material); err != nil {
				return fmt.Errorf("advance material status: %w", err)
			}
		}
	}
	return nil
}

func (s *materialUsageService) MarkNodeUsagesPendingConfirm(nodeID uint) error {
	usages, err := s.usageRepo.ListByNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("list material usages by node: %w", err)
	}
	for i := range usages {
		usage := &usages[i]
		// 一次验收不通过即转为待确认；已确认记录不应出现在未通过验收的节点下，保持不动。
		if usage.Status != constants.UsageStatusRegistered {
			continue
		}
		usage.Status = constants.UsageStatusPendingConfirm
		if err := s.usageRepo.Update(usage); err != nil {
			return fmt.Errorf("mark material usage pending confirm: %w", err)
		}
	}
	return nil
}

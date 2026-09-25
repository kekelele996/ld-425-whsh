package handler

import (
	"encoding/json"

	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/model"
)

func toProjectDTO(project *model.RenovationProject) dto.ProjectDTO {
	return dto.ProjectDTO{
		ID:              project.ID,
		Name:            project.Name,
		HouseType:       project.HouseType,
		Area:            project.Area,
		DecorStyle:      project.DecorStyle,
		Address:         project.Address,
		OwnerID:         project.OwnerID,
		DesignerID:      project.DesignerID,
		ForemanID:       project.ForemanID,
		Status:          project.Status,
		ContractAmount:  project.ContractAmount,
		StartDate:       project.StartDate,
		ExpectedEndDate: project.ExpectedEndDate,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
	}
}

func toDesignDTO(phase *model.DesignPhase) dto.DesignDTO {
	return dto.DesignDTO{
		ID:            phase.ID,
		ProjectID:     phase.ProjectID,
		Name:          phase.Name,
		DesignerID:    phase.DesignerID,
		Status:        phase.Status,
		Version:       phase.Version,
		Description:   phase.Description,
		FileURLs:      parseStringSlice(phase.FileURLs),
		ReviewComment: phase.ReviewComment,
		ReviewerID:    phase.ReviewerID,
		CreatedAt:     phase.CreatedAt,
		UpdatedAt:     phase.UpdatedAt,
	}
}

// toMaterialDTO 转换材料展示结构，installed 为累计已安装量（验收通过的用料登记）。
func toMaterialDTO(item *model.MaterialItem, installed float64) dto.MaterialDTO {
	remaining := item.Quantity - installed
	if remaining < 0 {
		remaining = 0
	}
	return dto.MaterialDTO{
		ID:                item.ID,
		ProjectID:         item.ProjectID,
		Name:              item.Name,
		Category:          item.Category,
		Spec:              item.Spec,
		Brand:             item.Brand,
		Quantity:          item.Quantity,
		Unit:              item.Unit,
		UnitPrice:         item.UnitPrice,
		TotalPrice:        item.TotalPrice,
		PurchaseStatus:    item.PurchaseStatus,
		Supplier:          item.Supplier,
		Space:             item.Space,
		InstalledQuantity: installed,
		RemainingQuantity: remaining,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toBudgetDTO(item *model.BudgetItem) dto.BudgetDTO {
	return dto.BudgetDTO{
		ID:           item.ID,
		ProjectID:    item.ProjectID,
		Category:     item.Category,
		BudgetAmount: item.BudgetAmount,
		ActualAmount: item.ActualAmount,
		Variance:     item.Variance,
		Remark:       item.Remark,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func toConstructionDTO(node *model.ConstructionNode) dto.ConstructionDTO {
	return dto.ConstructionDTO{
		ID:               node.ID,
		ProjectID:        node.ProjectID,
		Name:             node.Name,
		PlannedStartDate: node.PlannedStartDate,
		PlannedEndDate:   node.PlannedEndDate,
		ActualStartDate:  node.ActualStartDate,
		ActualEndDate:    node.ActualEndDate,
		Status:           node.Status,
		AcceptanceStatus: node.AcceptanceStatus,
		AcceptancePhotos: parseStringSlice(node.AcceptancePhotos),
		AcceptanceNote:   node.AcceptanceNote,
		Usages:           []dto.MaterialUsageDTO{},
		CreatedAt:        node.CreatedAt,
		UpdatedAt:        node.UpdatedAt,
	}
}

// toMaterialUsageDTO 转换用料登记展示结构，附带材料名称与单位便于展示。
func toMaterialUsageDTO(usage *model.MaterialUsage, material *model.MaterialItem) dto.MaterialUsageDTO {
	out := dto.MaterialUsageDTO{
		ID:         usage.ID,
		ProjectID:  usage.ProjectID,
		NodeID:     usage.NodeID,
		MaterialID: usage.MaterialID,
		Quantity:   usage.Quantity,
		Status:     usage.Status,
		Note:       usage.Note,
		CreatedAt:  usage.CreatedAt,
		UpdatedAt:  usage.UpdatedAt,
	}
	if material != nil {
		out.MaterialName = material.Name
		out.MaterialUnit = material.Unit
	}
	return out
}

func parseStringSlice(raw string) []string {
	if raw == "" || raw == "null" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}

func toProjectDTOList(projects []model.RenovationProject) []dto.ProjectDTO {
	out := make([]dto.ProjectDTO, 0, len(projects))
	for _, item := range projects {
		out = append(out, toProjectDTO(&item))
	}
	return out
}

func toDesignDTOList(phases []model.DesignPhase) []dto.DesignDTO {
	out := make([]dto.DesignDTO, 0, len(phases))
	for _, item := range phases {
		out = append(out, toDesignDTO(&item))
	}
	return out
}

// toMaterialDTOList 批量转换材料展示结构，installedMap 为各材料累计已安装量。
func toMaterialDTOList(items []model.MaterialItem, installedMap map[uint]float64) []dto.MaterialDTO {
	out := make([]dto.MaterialDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toMaterialDTO(&item, installedMap[item.ID]))
	}
	return out
}

func toBudgetDTOList(items []model.BudgetItem) []dto.BudgetDTO {
	out := make([]dto.BudgetDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toBudgetDTO(&item))
	}
	return out
}

// toConstructionDTOList 批量转换施工节点展示结构，usageMap 为各节点的用料登记。
func toConstructionDTOList(nodes []model.ConstructionNode, usageMap map[uint][]dto.MaterialUsageDTO) []dto.ConstructionDTO {
	out := make([]dto.ConstructionDTO, 0, len(nodes))
	for _, item := range nodes {
		nodeDTO := toConstructionDTO(&item)
		if usages, ok := usageMap[item.ID]; ok {
			nodeDTO.Usages = usages
		}
		out = append(out, nodeDTO)
	}
	return out
}

// groupUsagesByNode 用料登记按节点分组并转换为展示结构。
func groupUsagesByNode(usages []model.MaterialUsage, materials map[uint]*model.MaterialItem) map[uint][]dto.MaterialUsageDTO {
	out := make(map[uint][]dto.MaterialUsageDTO)
	for i := range usages {
		usage := &usages[i]
		out[usage.NodeID] = append(out[usage.NodeID], toMaterialUsageDTO(usage, materials[usage.MaterialID]))
	}
	return out
}

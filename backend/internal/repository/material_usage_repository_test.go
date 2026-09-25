package repository

import (
	"testing"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

func newUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.MaterialItem{}, &model.ConstructionNode{}, &model.MaterialUsage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestMaterialUsageRepository_CreateDuplicate(t *testing.T) {
	db := newUsageTestDB(t)
	repo := NewMaterialUsageRepository(db)

	first := &model.MaterialUsage{ProjectID: 1, NodeID: 10, MaterialID: 20, Quantity: 3, Status: constants.MaterialUsageStatusPending}
	if err := repo.Create(first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := &model.MaterialUsage{ProjectID: 1, NodeID: 10, MaterialID: 20, Quantity: 9, Status: constants.MaterialUsageStatusPending}
	if err := repo.Create(second); err == nil {
		t.Fatal("expected unique constraint error for duplicate node+material")
	}
}

func TestMaterialUsageRepository_SumAndUpdateStatus(t *testing.T) {
	db := newUsageTestDB(t)
	repo := NewMaterialUsageRepository(db)

	records := []model.MaterialUsage{
		{ProjectID: 1, NodeID: 1, MaterialID: 100, Quantity: 2, Status: constants.MaterialUsageStatusCounted},
		{ProjectID: 1, NodeID: 2, MaterialID: 100, Quantity: 3, Status: constants.MaterialUsageStatusPending},
		{ProjectID: 1, NodeID: 3, MaterialID: 100, Quantity: 5, Status: constants.MaterialUsageStatusExcluded},
		{ProjectID: 1, NodeID: 1, MaterialID: 200, Quantity: 4, Status: constants.MaterialUsageStatusCounted},
	}
	for i := range records {
		if err := repo.Create(&records[i]); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	// 待确认记录不参与已安装量汇总。
	totals, err := repo.SumQuantityByMaterials(1, []uint{100, 200}, []string{constants.MaterialUsageStatusCounted})
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if totals[100] != 2 || totals[200] != 4 {
		t.Fatalf("unexpected counted totals: %v", totals)
	}

	// 待计入 + 已计入共同占用余量，待确认被排除。
	reserved, err := repo.SumQuantityByMaterials(1, []uint{100},
		[]string{constants.MaterialUsageStatusPending, constants.MaterialUsageStatusCounted})
	if err != nil {
		t.Fatalf("sum reserved: %v", err)
	}
	if reserved[100] != 5 {
		t.Fatalf("expected reserved 5, got %v", reserved)
	}

	// 节点 3 整改后验收通过：批量转为已计入。
	if err := repo.UpdateStatusByNode(3, constants.MaterialUsageStatusCounted); err != nil {
		t.Fatalf("update status: %v", err)
	}
	totals, err = repo.SumQuantityByMaterials(1, []uint{100}, []string{constants.MaterialUsageStatusCounted})
	if err != nil {
		t.Fatalf("sum after update: %v", err)
	}
	if totals[100] != 7 {
		t.Fatalf("expected counted 7, got %v", totals)
	}
}

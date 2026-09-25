package service

import (
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"gorm.io/gorm"
)

func newUsageFlowDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.MaterialItem{},
		&model.ConstructionNode{},
		&model.MaterialUsage{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

type usageFlow struct {
	materialSvc MaterialService
	usageSvc    MaterialUsageService
	constSvc    ConstructionService
}

func newUsageFlow(t *testing.T) (*usageFlow, uint, uint, uint) {
	t.Helper()
	db := newUsageFlowDB(t)
	log := slog.Default()

	materialRepo := repository.NewMaterialRepository(db)
	nodeRepo := repository.NewConstructionRepository(db)
	usageRepo := repository.NewMaterialUsageRepository(db)
	txMgr := repository.NewTransactionManager(db)

	flow := &usageFlow{
		materialSvc: NewMaterialService(materialRepo, usageRepo, log),
		usageSvc:    NewMaterialUsageService(usageRepo, nodeRepo, materialRepo, log),
		constSvc:    NewConstructionService(nodeRepo, txMgr, log),
	}

	material, err := flow.materialSvc.Create(&dto.CreateMaterialRequest{
		ProjectID: 1, Name: "抛光砖", Category: "瓷砖", Quantity: 10, Unit: "件",
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	node, err := flow.constSvc.Create(&dto.CreateConstructionRequest{ProjectID: 1, Name: "瓦工"})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	otherMaterial, err := flow.materialSvc.Create(&dto.CreateMaterialRequest{
		ProjectID: 2, Name: "别的项目材料", Category: "其他", Quantity: 100, Unit: "件",
	})
	if err != nil {
		t.Fatalf("create other material: %v", err)
	}
	return flow, material.ID, node.ID, otherMaterial.ID
}

// TestMaterialUsageFlow 串联：登记 -> 验收通过计入 -> 已安装/剩余量 -> 不通过转待确认 -> 幂等。
func TestMaterialUsageFlow(t *testing.T) {
	t.Run("register before completion and count on acceptance", func(t *testing.T) {
		flow, materialID, nodeID, _ := newUsageFlow(t)

		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: nodeID, MaterialID: materialID, Quantity: 4,
		}); err != nil {
			t.Fatalf("register usage: %v", err)
		}
		// 验收前：已安装量为 0，待计入不参与已安装。
		if installed, err := flow.materialSvc.InstalledQuantity(materialID); err != nil || installed != 0 {
			t.Fatalf("before acceptance installed = %v, err = %v; want 0", installed, err)
		}

		if _, err := flow.constSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
			t.Fatalf("complete node: %v", err)
		}
		if _, err := flow.constSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
			t.Fatalf("accept node: %v", err)
		}
		if installed, err := flow.materialSvc.InstalledQuantity(materialID); err != nil || installed != 4 {
			t.Fatalf("after acceptance installed = %v, err = %v; want 4", installed, err)
		}
	})

	t.Run("failed acceptance excludes usage from remaining calculation", func(t *testing.T) {
		flow, materialID, nodeID, _ := newUsageFlow(t)

		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: nodeID, MaterialID: materialID, Quantity: 6,
		}); err != nil {
			t.Fatalf("register usage: %v", err)
		}
		if _, err := flow.constSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
			t.Fatalf("complete node: %v", err)
		}
		if _, err := flow.constSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: false}); err != nil {
			t.Fatalf("reject acceptance: %v", err)
		}
		// 一次验收不通过 -> 待确认，不参与余量（已安装量）计算。
		if installed, err := flow.materialSvc.InstalledQuantity(materialID); err != nil || installed != 0 {
			t.Fatalf("after failed acceptance installed = %v, err = %v; want 0", installed, err)
		}
		details, err := flow.usageSvc.ListDetailsByNode(nodeID)
		if err != nil {
			t.Fatalf("list details: %v", err)
		}
		if len(details) != 1 || details[0].Usage.Status != constants.MaterialUsageStatusExcluded {
			t.Fatalf("expected single excluded usage, got %+v", details)
		}
	})

	t.Run("duplicate submission keeps the first result", func(t *testing.T) {
		flow, materialID, nodeID, _ := newUsageFlow(t)

		first, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: nodeID, MaterialID: materialID, Quantity: 2, Remark: "第一次",
		})
		if err != nil {
			t.Fatalf("first register: %v", err)
		}
		// 同节点同材料换个数量重复提交：保留第一次结果。
		second, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: nodeID, MaterialID: materialID, Quantity: 9, Remark: "第二次",
		})
		if err != nil {
			t.Fatalf("duplicate register should be idempotent, got %v", err)
		}
		if second.Usage.ID != first.Usage.ID || second.Usage.Quantity != 2 || second.Usage.Remark != "第一次" {
			t.Fatalf("duplicate submission altered first result: first=%+v second=%+v", first, second)
		}
		details, err := flow.usageSvc.ListDetailsByNode(nodeID)
		if err != nil {
			t.Fatalf("list details: %v", err)
		}
		if len(details) != 1 {
			t.Fatalf("expected exactly one usage record, got %d", len(details))
		}
	})

	t.Run("cumulative quantity cannot exceed purchased quantity", func(t *testing.T) {
		flow, materialID, _, _ := newUsageFlow(t)

		// 新建两个节点，分别登记同一材料：采购量 10。
		node1, err := flow.constSvc.Create(&dto.CreateConstructionRequest{ProjectID: 1, Name: "瓦工"})
		if err != nil {
			t.Fatalf("create node1: %v", err)
		}
		node2, err := flow.constSvc.Create(&dto.CreateConstructionRequest{ProjectID: 1, Name: "安装"})
		if err != nil {
			t.Fatalf("create node2: %v", err)
		}
		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: node1.ID, MaterialID: materialID, Quantity: 6,
		}); err != nil {
			t.Fatalf("register node1: %v", err)
		}
		// 待计入 + 已计入 = 6，再登记 5 超过采购量 10。
		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: node2.ID, MaterialID: materialID, Quantity: 5,
		}); err == nil {
			t.Fatal("expected conflict when cumulative usage exceeds purchased quantity")
		}
		// 刚好 10 允许登记。
		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: node2.ID, MaterialID: materialID, Quantity: 4,
		}); err != nil {
			t.Fatalf("register exact remaining quantity: %v", err)
		}
	})

	t.Run("cannot register usage on completed node or cross-project material", func(t *testing.T) {
		flow, materialID, nodeID, otherMaterialID := newUsageFlow(t)

		if _, err := flow.constSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
			t.Fatalf("complete node: %v", err)
		}
		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: nodeID, MaterialID: materialID, Quantity: 1,
		}); err == nil {
			t.Fatal("expected conflict when registering on completed node")
		}

		anotherNode, err := flow.constSvc.Create(&dto.CreateConstructionRequest{ProjectID: 1, Name: "油漆"})
		if err != nil {
			t.Fatalf("create node: %v", err)
		}
		if _, err := flow.usageSvc.Register(&dto.RegisterMaterialUsageRequest{
			NodeID: anotherNode.ID, MaterialID: otherMaterialID, Quantity: 1,
		}); err == nil {
			t.Fatal("expected bad request for cross-project material")
		}
	})
}

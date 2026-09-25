package service

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"gorm.io/gorm"
)

func newUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.MaterialItem{}, &model.ConstructionNode{}, &model.MaterialUsage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

type usageFixture struct {
	db              *gorm.DB
	materialSvc     MaterialService
	usageSvc        MaterialUsageService
	constructionSvc ConstructionService
}

func newUsageFixture(t *testing.T) usageFixture {
	db := newUsageTestDB(t)
	materialRepo := repository.NewMaterialRepository(db)
	nodeRepo := repository.NewConstructionRepository(db)
	usageRepo := repository.NewMaterialUsageRepository(db)
	logger := slog.Default()
	materialSvc := NewMaterialService(materialRepo, usageRepo, logger)
	usageSvc := NewMaterialUsageService(usageRepo, materialRepo, nodeRepo, logger)
	constructionSvc := NewConstructionService(nodeRepo, usageRepo, usageSvc, repository.NewTxRunner(db), logger)
	return usageFixture{db: db, materialSvc: materialSvc, usageSvc: usageSvc, constructionSvc: constructionSvc}
}

func (f usageFixture) createMaterial(t *testing.T, name string, qty float64) uint {
	t.Helper()
	material, err := f.materialSvc.Create(&dto.CreateMaterialRequest{
		ProjectID: 1, Name: name, Category: "瓷砖", Quantity: qty, Unit: "件",
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	if _, err := f.materialSvc.UpdateStatus(material.ID, constants.PurchaseStatusOrdered); err != nil {
		t.Fatalf("order material: %v", err)
	}
	if _, err := f.materialSvc.UpdateStatus(material.ID, constants.PurchaseStatusDelivered); err != nil {
		t.Fatalf("deliver material: %v", err)
	}
	return material.ID
}

func (f usageFixture) createNode(t *testing.T, name string) uint {
	t.Helper()
	node, err := f.constructionSvc.Create(&dto.CreateConstructionRequest{ProjectID: 1, Name: name})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node.ID
}

func (f usageFixture) startAndCompleteNode(t *testing.T, nodeID uint) {
	t.Helper()
	if _, err := f.constructionSvc.UpdateStatus(nodeID, constants.ConstructionStatusInProgress); err != nil {
		t.Fatalf("start node: %v", err)
	}
}

func TestMaterialUsage_AcceptanceFlowCountsInstalled(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "抛光砖", 10)
	nodeID := f.createNode(t, "瓦工")
	f.startAndCompleteNode(t, nodeID)

	// 节点完工前登记本次用料。
	if _, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{
		MaterialID: materialID, Quantity: 6, Note: "客厅铺贴",
	}); err != nil {
		t.Fatalf("register usage: %v", err)
	}

	if _, err := f.constructionSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete node: %v", err)
	}

	// 已登记但未验收：不参与已安装量与余量计算。
	installed, err := f.materialSvc.GetInstalledQuantity(materialID)
	if err != nil {
		t.Fatalf("get installed: %v", err)
	}
	if installed != 0 {
		t.Fatalf("registered usage must not count, got installed=%.2f", installed)
	}

	// 验收通过后计入已安装量。
	if _, err := f.constructionSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	installed, _ = f.materialSvc.GetInstalledQuantity(materialID)
	if installed != 6 {
		t.Fatalf("expected installed 6, got %.2f", installed)
	}

	// 重复提交验收：保留第一次结果，不重复计数。
	if _, err := f.constructionSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
		t.Fatalf("re-accept: %v", err)
	}
	installed, _ = f.materialSvc.GetInstalledQuantity(materialID)
	if installed != 6 {
		t.Fatalf("repeated acceptance must not double count, got %.2f", installed)
	}
	usages, _ := f.usageSvc.ListByNode(nodeID)
	if len(usages) != 1 || usages[0].Status != constants.UsageStatusConfirmed {
		t.Fatalf("expected single confirmed usage, got %+v", usages)
	}
}

func TestMaterialUsage_ExceedPurchaseQuantityRollsBack(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "抛光砖", 10)

	// 第一个节点装 6，通过验收。
	node1 := f.createNode(t, "瓦工")
	f.startAndCompleteNode(t, node1)
	if _, err := f.usageSvc.Register(node1, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 6}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := f.constructionSvc.UpdateStatus(node1, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := f.constructionSvc.Accept(node1, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// 第二个节点再装 5，累计 11 > 采购量 10：验收必须失败并整体回滚。
	node2 := f.createNode(t, "安装")
	f.startAndCompleteNode(t, node2)
	if _, err := f.usageSvc.Register(node2, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 5}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := f.constructionSvc.UpdateStatus(node2, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}
	_, err := f.constructionSvc.Accept(node2, &dto.AcceptConstructionRequest{Accepted: true})
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected business conflict, got %v", err)
	}

	// 事务回滚：节点验收状态仍为 Pending，用料仍为 Registered，已安装量仍为 6。
	node, _ := f.constructionSvc.GetByID(node2)
	if node.AcceptanceStatus != constants.AcceptanceStatusPending {
		t.Fatalf("expected rollback to Pending, got %s", node.AcceptanceStatus)
	}
	installed, _ := f.materialSvc.GetInstalledQuantity(materialID)
	if installed != 6 {
		t.Fatalf("expected installed unchanged 6, got %.2f", installed)
	}
}

func TestMaterialUsage_FailedAcceptanceGoesPendingConfirm(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "乳胶漆", 5)
	nodeID := f.createNode(t, "油漆")
	f.startAndCompleteNode(t, nodeID)
	if _, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 2}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := f.constructionSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// 一次验收不通过 → 待确认，不参与余量计算。
	if _, err := f.constructionSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: false}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	usages, _ := f.usageSvc.ListByNode(nodeID)
	if usages[0].Status != constants.UsageStatusPendingConfirm {
		t.Fatalf("expected PendingConfirm, got %s", usages[0].Status)
	}
	installed, _ := f.materialSvc.GetInstalledQuantity(materialID)
	if installed != 0 {
		t.Fatalf("pending confirm usage must not count, got %.2f", installed)
	}

	// 返工后再次验收通过 → 计入已安装量。
	if _, err := f.constructionSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
		t.Fatalf("re-accept: %v", err)
	}
	installed, _ = f.materialSvc.GetInstalledQuantity(materialID)
	if installed != 2 {
		t.Fatalf("expected installed 2 after rework pass, got %.2f", installed)
	}
}

func TestMaterialUsage_DuplicateSubmitKeepsFirst(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "筒灯", 20)
	nodeID := f.createNode(t, "安装")
	f.startAndCompleteNode(t, nodeID)

	first, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{
		MaterialID: materialID, Quantity: 3, ClientKey: "client-key-1",
	})
	if err != nil {
		t.Fatalf("register first: %v", err)
	}
	// 同一记录重复提交（数量被改成 9）：保留第一次结果。
	again, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{
		MaterialID: materialID, Quantity: 9, ClientKey: "client-key-1",
	})
	if err != nil {
		t.Fatalf("register again: %v", err)
	}
	if again.ID != first.ID || again.Quantity != 3 {
		t.Fatalf("expected first record (id=%d qty=3), got id=%d qty=%.2f", first.ID, again.ID, again.Quantity)
	}
	all, _ := f.usageSvc.ListByNode(nodeID)
	if len(all) != 1 {
		t.Fatalf("expected 1 usage row, got %d", len(all))
	}
}

func TestMaterialUsage_RegisterBlockedAfterCompleted(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "地板", 20)
	nodeID := f.createNode(t, "木工")
	f.startAndCompleteNode(t, nodeID)
	if _, err := f.constructionSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}
	_, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 1})
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected conflict registering after completion, got %v", err)
	}
}

func TestMaterialUsage_ManualInstalledStatusRejected(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "地板", 20)
	_, err := f.materialSvc.UpdateStatus(materialID, constants.PurchaseStatusInstalled)
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected conflict marking installed manually, got %v", err)
	}
}

func TestMaterialUsage_FullInstallAdvancesStatus(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "地板", 10)
	nodeID := f.createNode(t, "木工")
	f.startAndCompleteNode(t, nodeID)
	if _, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 10}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := f.constructionSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := f.constructionSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	material, _ := f.materialSvc.GetByID(materialID)
	if material.PurchaseStatus != constants.PurchaseStatusInstalled {
		t.Fatalf("expected Installed after full installation, got %s", material.PurchaseStatus)
	}
}

func TestMaterialUsage_DeleteAndQuantityGuards(t *testing.T) {
	f := newUsageFixture(t)
	materialID := f.createMaterial(t, "地板", 10)
	nodeID := f.createNode(t, "木工")
	f.startAndCompleteNode(t, nodeID)
	usage, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 4})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// 未确认登记可删除。
	if err := f.usageSvc.Delete(usage.ID); err != nil {
		t.Fatalf("delete unconfirmed: %v", err)
	}

	usage2, err := f.usageSvc.Register(nodeID, &dto.RegisterMaterialUsageRequest{MaterialID: materialID, Quantity: 4})
	if err != nil {
		t.Fatalf("register again: %v", err)
	}
	if _, err := f.constructionSvc.UpdateStatus(nodeID, constants.ConstructionStatusCompleted); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := f.constructionSvc.Accept(nodeID, &dto.AcceptConstructionRequest{Accepted: true}); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// 已确认记录不能删除；材料不能删除。
	if err := f.usageSvc.Delete(usage2.ID); err == nil {
		t.Fatal("expected error deleting confirmed usage")
	}
	if err := f.materialSvc.Delete(materialID); err == nil {
		t.Fatal("expected error deleting material with usage records")
	}

	// 采购量不能下调到已安装量以下。
	newQty := 3.0
	if _, err := f.materialSvc.Update(materialID, &dto.UpdateMaterialRequest{Quantity: &newQty}); err == nil {
		t.Fatal("expected conflict reducing quantity below installed")
	}
}

package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/home-renovation/platform/internal/config"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/handler"
	"github.com/home-renovation/platform/internal/logger"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"github.com/home-renovation/platform/internal/router"
	"github.com/home-renovation/platform/internal/service"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	log := logger.New("info")

	db, err := connectDB(cfg, log)
	if err != nil {
		log.Error("connect database failed", "error", err)
		os.Exit(1)
	}

	if err := migrate(db); err != nil {
		log.Error("migrate database failed", "error", err)
		os.Exit(1)
	}

	// 装配仓储层。
	projectRepo := repository.NewProjectRepository(db)
	designRepo := repository.NewDesignRepository(db)
	materialRepo := repository.NewMaterialRepository(db)
	usageRepo := repository.NewMaterialUsageRepository(db)
	budgetRepo := repository.NewBudgetRepository(db)
	constructionRepo := repository.NewConstructionRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)

	// 装配服务层。
	userSvc := service.NewUserService(userRepo, cfg.JWT, log)
	auditSvc := service.NewAuditService(auditRepo, log)
	projectSvc := service.NewProjectService(projectRepo, log)
	designSvc := service.NewDesignService(designRepo, log)
	materialSvc := service.NewMaterialService(materialRepo, usageRepo, log)
	budgetSvc := service.NewBudgetService(budgetRepo, log)
	usageSvc := service.NewMaterialUsageService(usageRepo, materialRepo, constructionRepo, log)
	constructionSvc := service.NewConstructionService(constructionRepo, usageRepo, usageSvc, repository.NewTxRunner(db), log)

	if err := userSvc.SeedIfEmpty(); err != nil {
		log.Error("seed users failed", "error", err)
		os.Exit(1)
	}
	if err := seedDemoData(projectSvc, designSvc, materialSvc, budgetSvc, constructionSvc, usageSvc, log); err != nil {
		log.Error("seed demo data failed", "error", err)
		os.Exit(1)
	}

	// 装配处理器与路由。
	engine := router.New(router.Deps{
		Config:         cfg,
		Logger:         log,
		UserSvc:        userSvc,
		AuditSvc:       auditSvc,
		AuditRepo:      auditRepo,
		ProjectH:       handler.NewProjectHandler(projectSvc),
		DesignH:        handler.NewDesignHandler(designSvc),
		MaterialH:      handler.NewMaterialHandler(materialSvc),
		BudgetH:        handler.NewBudgetHandler(budgetSvc),
		ConstructionH:  handler.NewConstructionHandler(constructionSvc, usageSvc, materialSvc),
		MaterialUsageH: handler.NewMaterialUsageHandler(usageSvc, materialSvc),
		AuditH:         handler.NewAuditHandler(auditSvc),
		UploadH:        handler.NewUploadHandler(),
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info("server starting", "addr", addr)
	if err := engine.Run(addr); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func connectDB(cfg *config.Config, log *slog.Logger) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		db, err = gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{})
		if err == nil {
			sqlDB, openErr := db.DB()
			if openErr == nil {
				sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
				sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
				sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
				if pingErr := sqlDB.Ping(); pingErr == nil {
					return db, nil
				}
			}
		}
		log.Warn("database not ready, retrying", "attempt", attempt+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("connect database: %w", err)
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.RenovationProject{},
		&model.DesignPhase{},
		&model.MaterialItem{},
		&model.BudgetItem{},
		&model.ConstructionNode{},
		&model.MaterialUsage{},
		&model.AuditLog{},
	)
}

func seedDemoData(
	projectSvc service.ProjectService,
	designSvc service.DesignService,
	materialSvc service.MaterialService,
	budgetSvc service.BudgetService,
	constructionSvc service.ConstructionService,
	usageSvc service.MaterialUsageService,
	log *slog.Logger,
) error {
	_, total, err := projectSvc.List("", "", 1, 1)
	if err != nil {
		return fmt.Errorf("check demo projects: %w", err)
	}
	if total > 0 {
		return nil
	}
	demoReq := dtoCreateProject()
	project, err := projectSvc.Create(demoReq)
	if err != nil {
		return fmt.Errorf("seed project: %w", err)
	}

	designs := []struct {
		name string
		desc string
	}{
		{"方案设计", "整体空间规划与风格提案"},
		{"效果图", "客厅与主卧效果图渲染"},
		{"施工图", "水电与木工施工图纸"},
		{"软装方案", "家具与布艺软装搭配"},
	}
	for _, item := range designs {
		if _, err := designSvc.Create(&dto.CreateDesignRequest{
			ProjectID: project.ID, Name: item.name, DesignerID: 2, Description: item.desc,
		}); err != nil {
			return fmt.Errorf("seed design %s: %w", item.name, err)
		}
	}

	materials := []struct {
		name, category, spec, brand, space string
		qty, price                         float64
	}{
		{"抛光砖", "瓷砖", "800x800", "东鹏", "客厅", 80, 168},
		{"实木地板", "地板", "1210x165", "大自然", "卧室", 42, 259},
		{"乳胶漆", "油漆", "5L", "多乐士", "客厅", 12, 328},
		{"LED筒灯", "灯具", "7W", "欧普", "厨房", 18, 59},
	}
	materialIDs := make(map[string]uint)
	for _, item := range materials {
		created, err := materialSvc.Create(&dto.CreateMaterialRequest{
			ProjectID: project.ID, Name: item.name, Category: item.category, Spec: item.spec,
			Brand: item.brand, Quantity: item.qty, Unit: "件", UnitPrice: item.price, Space: item.space,
		})
		if err != nil {
			return fmt.Errorf("seed material %s: %w", item.name, err)
		}
		materialIDs[item.name] = created.ID
	}

	budgets := []struct {
		category string
		budget   float64
		actual   float64
	}{
		{"Design", 80000, 45000},
		{"Material", 120000, 68000},
		{"Labor", 90000, 30000},
		{"Furniture", 60000, 0},
		{"Appliance", 50000, 0},
	}
	for _, item := range budgets {
		if _, err := budgetSvc.Create(&dto.CreateBudgetRequest{
			ProjectID: project.ID, Category: item.category, BudgetAmount: item.budget, ActualAmount: item.actual,
		}); err != nil {
			return fmt.Errorf("seed budget %s: %w", item.category, err)
		}
	}

	nodes := []struct {
		name, start, end string
	}{
		{"拆改", "2026-08-01", "2026-08-15"},
		{"水电", "2026-08-16", "2026-09-05"},
		{"木工", "2026-09-06", "2026-10-01"},
		{"瓦工", "2026-10-02", "2026-10-28"},
		{"油漆", "2026-10-29", "2026-11-20"},
		{"安装", "2026-11-21", "2026-12-15"},
		{"软装", "2026-12-16", "2026-12-31"},
	}
	nodeIDs := make(map[string]uint)
	for _, item := range nodes {
		created, err := constructionSvc.Create(&dto.CreateConstructionRequest{
			ProjectID: project.ID, Name: item.name, PlannedStartDate: &item.start, PlannedEndDate: &item.end,
		})
		if err != nil {
			return fmt.Errorf("seed construction %s: %w", item.name, err)
		}
		nodeIDs[item.name] = created.ID
	}

	// 种子用料演示：
	// 1) 抛光砖到货 → 瓦工节点完工前登记 30 件 → 验收通过计入已安装量（剩余 50 件）；
	// 2) 乳胶漆到货 → 油漆节点已完工待验收，登记 4 件（Registered，不计入已安装量）。
	tileID := materialIDs["抛光砖"]
	if _, err := materialSvc.UpdateStatus(tileID, constants.PurchaseStatusOrdered); err != nil {
		return fmt.Errorf("seed material order: %w", err)
	}
	if _, err := materialSvc.UpdateStatus(tileID, constants.PurchaseStatusDelivered); err != nil {
		return fmt.Errorf("seed material delivery: %w", err)
	}
	tilingNodeID := nodeIDs["瓦工"]
	if _, err := constructionSvc.UpdateStatus(tilingNodeID, constants.ConstructionStatusInProgress); err != nil {
		return fmt.Errorf("seed tiling node start: %w", err)
	}
	if _, err := usageSvc.Register(tilingNodeID, &dto.RegisterMaterialUsageRequest{
		MaterialID: tileID, Quantity: 30, Note: "客厅地面铺贴", ClientKey: "seed-tiling-tile",
	}); err != nil {
		return fmt.Errorf("seed tiling usage: %w", err)
	}
	if _, err := constructionSvc.UpdateStatus(tilingNodeID, constants.ConstructionStatusCompleted); err != nil {
		return fmt.Errorf("seed tiling node complete: %w", err)
	}
	if _, err := constructionSvc.Accept(tilingNodeID, &dto.AcceptConstructionRequest{
		Accepted: true, Note: "验收通过",
	}); err != nil {
		return fmt.Errorf("seed tiling accept: %w", err)
	}

	paintID := materialIDs["乳胶漆"]
	if _, err := materialSvc.UpdateStatus(paintID, constants.PurchaseStatusOrdered); err != nil {
		return fmt.Errorf("seed paint order: %w", err)
	}
	if _, err := materialSvc.UpdateStatus(paintID, constants.PurchaseStatusDelivered); err != nil {
		return fmt.Errorf("seed paint delivery: %w", err)
	}
	paintingNodeID := nodeIDs["油漆"]
	if _, err := constructionSvc.UpdateStatus(paintingNodeID, constants.ConstructionStatusInProgress); err != nil {
		return fmt.Errorf("seed painting node start: %w", err)
	}
	if _, err := usageSvc.Register(paintingNodeID, &dto.RegisterMaterialUsageRequest{
		MaterialID: paintID, Quantity: 4, Note: "待墙面验收", ClientKey: "seed-painting-paint",
	}); err != nil {
		return fmt.Errorf("seed painting usage: %w", err)
	}
	if _, err := constructionSvc.UpdateStatus(paintingNodeID, constants.ConstructionStatusCompleted); err != nil {
		return fmt.Errorf("seed painting node complete: %w", err)
	}

	log.Info("seeded demo project", "project", project.Name, "project_id", project.ID)
	return nil
}

func dtoCreateProject() *dto.CreateProjectRequest {
	start := "2026-08-01"
	end := "2026-12-31"
	return &dto.CreateProjectRequest{
		Name:            "星河湾三居室整装",
		HouseType:       "三室",
		Area:            128,
		DecorStyle:      "Modern",
		Address:         "上海市浦东新区星河湾",
		OwnerID:         4,
		DesignerID:      2,
		ForemanID:       3,
		ContractAmount:  428000,
		StartDate:       &start,
		ExpectedEndDate: &end,
	}
}

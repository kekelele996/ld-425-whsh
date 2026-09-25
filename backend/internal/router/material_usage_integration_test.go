package router

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/config"
	"github.com/home-renovation/platform/internal/handler"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"github.com/home-renovation/platform/internal/service"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupIntegration(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.RenovationProject{}, &model.DesignPhase{},
		&model.MaterialItem{}, &model.MaterialUsage{}, &model.BudgetItem{},
		&model.ConstructionNode{}, &model.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("Contractor123"), bcrypt.DefaultCost)
	if err := db.Create(&model.User{Username: "contractor", PasswordHash: string(hash), Role: "Contractor", Name: "老李"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	log := slog.Default()
	userSvc := service.NewUserService(repository.NewUserRepository(db), config.JWTConfig{Secret: "test-secret"}, log)
	auditRepo := repository.NewAuditLogRepository(db)
	materialRepo := repository.NewMaterialRepository(db)
	usageRepo := repository.NewMaterialUsageRepository(db)
	constructionRepo := repository.NewConstructionRepository(db)
	txMgr := repository.NewTransactionManager(db)
	materialSvc := service.NewMaterialService(materialRepo, usageRepo, log)
	usageSvc := service.NewMaterialUsageService(usageRepo, constructionRepo, materialRepo, log)
	constructionSvc := service.NewConstructionService(constructionRepo, txMgr, log)
	projectSvc := service.NewProjectService(repository.NewProjectRepository(db), log)
	designSvc := service.NewDesignService(repository.NewDesignRepository(db), log)
	budgetSvc := service.NewBudgetService(repository.NewBudgetRepository(db), log)
	auditSvc := service.NewAuditService(auditRepo, log)

	engine := New(Deps{
		Config:         &config.Config{Server: config.ServerConfig{Mode: "test"}},
		Logger:         log,
		UserSvc:        userSvc,
		AuditSvc:       auditSvc,
		AuditRepo:      auditRepo,
		ProjectH:       handler.NewProjectHandler(projectSvc),
		DesignH:        handler.NewDesignHandler(designSvc),
		MaterialH:      handler.NewMaterialHandler(materialSvc),
		MaterialUsageH: handler.NewMaterialUsageHandler(usageSvc),
		BudgetH:        handler.NewBudgetHandler(budgetSvc),
		ConstructionH:  handler.NewConstructionHandler(constructionSvc, usageSvc),
		AuditH:         handler.NewAuditHandler(auditSvc),
		UploadH:        handler.NewUploadHandler(),
	})
	return engine, db
}

func doJSON(t *testing.T, engine *gin.Engine, token, method, path string, body any) apiEnvelope {
	t.Helper()
	var reader *bytes.Buffer
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewBuffer(raw)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code >= 500 {
		t.Fatalf("%s %s -> status %d body %s", method, path, w.Code, w.Body.String())
	}
	var env apiEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %s %s: %v body=%s", method, path, err, w.Body.String())
	}
	return env
}

func TestMaterialUsageHTTPIntegration(t *testing.T) {
	engine, db := setupIntegration(t)

	// 登录获取 token。
	login := doJSON(t, engine, "", http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "contractor", "password": "Contractor123"})
	if login.Code != 0 {
		t.Fatalf("login failed: %s", login.Message)
	}
	var loginData struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(login.Data, &loginData)
	token := loginData.Token

	// 直接插一条项目与材料（contractor 无创建权限）。
	if err := db.Create(&model.RenovationProject{Name: "测试项目", HouseType: "三室", Area: 90, DecorStyle: "Modern", Status: "InProgress"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.MaterialItem{ProjectID: 1, Name: "抛光砖", Category: "瓷砖", Quantity: 10, Unit: "件", PurchaseStatus: "Delivered"}).Error; err != nil {
		t.Fatal(err)
	}

	// 直接插入节点（contractor 无创建节点权限，创建属于 Admin/PM）。
	node1 := &model.ConstructionNode{ProjectID: 1, Name: "瓦工", Status: "InProgress", AcceptanceStatus: "Pending", AcceptancePhotos: "[]"}
	if err := db.Create(node1).Error; err != nil {
		t.Fatal(err)
	}
	node2 := &model.ConstructionNode{ProjectID: 1, Name: "安装", Status: "InProgress", AcceptanceStatus: "Pending", AcceptancePhotos: "[]"}
	if err := db.Create(node2).Error; err != nil {
		t.Fatal(err)
	}

	// 登记 6 件。
	env := doJSON(t, engine, token, http.MethodPost, "/api/v1/material-usages",
		map[string]any{"node_id": node1.ID, "material_id": 1, "quantity": 6})
	if env.Code != 0 {
		t.Fatalf("register usage: %s", env.Message)
	}

	// 重复提交（不同数量）-> 保留第一次。
	env = doJSON(t, engine, token, http.MethodPost, "/api/v1/material-usages",
		map[string]any{"node_id": node1.ID, "material_id": 1, "quantity": 9})
	if env.Code != 0 {
		t.Fatalf("duplicate register should be idempotent: %s", env.Message)
	}
	var dup struct {
		Quantity float64 `json:"quantity"`
	}
	_ = json.Unmarshal(env.Data, &dup)
	if dup.Quantity != 6 {
		t.Fatalf("expected first quantity 6, got %v", dup.Quantity)
	}

	// 超量登记（第二个节点登记 5，累计 11 > 10）-> 409。
	env = doJSON(t, engine, token, http.MethodPost, "/api/v1/material-usages",
		map[string]any{"node_id": node2.ID, "material_id": 1, "quantity": 5})
	if env.Code == 0 {
		t.Fatal("expected conflict exceeding purchased quantity")
	}

	// 材料列表：已安装 0，剩余 10。
	env = doJSON(t, engine, token, http.MethodGet, "/api/v1/materials?project_id=1", nil)
	var materials []struct {
		Installed float64 `json:"installed_quantity"`
		Remaining float64 `json:"remaining_quantity"`
	}
	_ = json.Unmarshal(env.Data, &materials)
	if len(materials) != 1 || materials[0].Installed != 0 || materials[0].Remaining != 10 {
		t.Fatalf("before accept material view wrong: %+v", materials)
	}

	// 节点 1 完工 + 验收不通过 -> 待确认，不计余量。
	env = doJSON(t, engine, token, http.MethodPut, "/api/v1/constructions/1/status",
		map[string]string{"status": "Completed"})
	if env.Code != 0 {
		t.Fatalf("complete: %s", env.Message)
	}
	env = doJSON(t, engine, token, http.MethodPut, "/api/v1/constructions/1/accept",
		map[string]any{"accepted": false, "note": "空鼓"})
	if env.Code != 0 {
		t.Fatalf("reject: %s", env.Message)
	}
	env = doJSON(t, engine, token, http.MethodGet, "/api/v1/materials?project_id=1", nil)
	_ = json.Unmarshal(env.Data, &materials)
	if materials[0].Installed != 0 || materials[0].Remaining != 10 {
		t.Fatalf("after failed accept, installed/remaining wrong: %+v", materials)
	}

	// 节点 2 此时登记 10 应允许（待确认的 6 不占余量）。
	env = doJSON(t, engine, token, http.MethodPost, "/api/v1/material-usages",
		map[string]any{"node_id": node2.ID, "material_id": 1, "quantity": 10})
	if env.Code != 0 {
		t.Fatalf("register 10 after exclusion should succeed: %s", env.Message)
	}

	// 节点 2 完工 + 验收通过 -> 已安装 10，剩余 0。
	env = doJSON(t, engine, token, http.MethodPut, "/api/v1/constructions/2/status",
		map[string]string{"status": "Completed"})
	if env.Code != 0 {
		t.Fatalf("complete node2: %s", env.Message)
	}
	env = doJSON(t, engine, token, http.MethodPut, "/api/v1/constructions/2/accept",
		map[string]any{"accepted": true})
	if env.Code != 0 {
		t.Fatalf("accept node2: %s", env.Message)
	}
	env = doJSON(t, engine, token, http.MethodGet, "/api/v1/materials?project_id=1", nil)
	_ = json.Unmarshal(env.Data, &materials)
	if materials[0].Installed != 10 || materials[0].Remaining != 0 {
		t.Fatalf("after pass accept, installed/remaining wrong: %+v", materials)
	}

	// 节点详情带 usages 明细，且展开能看到两种状态。
	env = doJSON(t, engine, token, http.MethodGet, "/api/v1/constructions?project_id=1", nil)
	var nodes []struct {
		ID     uint `json:"id"`
		Usages []struct {
			Status       string  `json:"status"`
			Quantity     float64 `json:"quantity"`
			MaterialName string  `json:"material_name"`
		} `json:"usages"`
	}
	_ = json.Unmarshal(env.Data, &nodes)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	byID := map[uint]int{}
	for _, n := range nodes {
		byID[n.ID] = len(n.Usages)
		if len(n.Usages) > 0 && n.Usages[0].MaterialName != "抛光砖" {
			t.Fatalf("usage material name not populated: %+v", n.Usages)
		}
	}
	if byID[1] != 1 || byID[2] != 1 {
		t.Fatalf("each node should carry 1 usage, got %v", byID)
	}

	// GET /material-usages/node?node_id=1 明细接口。
	env = doJSON(t, engine, token, http.MethodGet, "/api/v1/material-usages/node?node_id=1", nil)
	var usages []struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(env.Data, &usages)
	if len(usages) != 1 || usages[0].Status != "Excluded" {
		t.Fatalf("node1 usage should be Excluded, got %+v", usages)
	}
}

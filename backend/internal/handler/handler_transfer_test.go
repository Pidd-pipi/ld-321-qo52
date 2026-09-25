package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

var transferDBSeq atomic.Int64

func newTransferRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:transfer-test-%d?mode=memory&cache=shared", transferDBSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Machine{}, &model.FarmTask{}, &model.Transfer{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&[]model.Machine{
		{ID: "m2", Code: "NJ-2026-002", Name: "雷沃谷神收割机", Field: "南湾稻田", Status: constants.MachineIdle, CurrentTask: constants.MachineIdleTask},
		{ID: "m1", Code: "NJ-2026-001", Name: "东方红 1804", Field: "北岭 1 号田", Status: constants.MachineWorking, CurrentTask: "春耕翻地"},
	}).Error; err != nil {
		t.Fatalf("seed machines: %v", err)
	}
	if err := db.Create(&model.FarmTask{ID: "t9", Type: "施肥", Field: "南湾稻田", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002", RecommendedDriver: "刘强"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dashRepo := repository.NewDashboardRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	dashboardSvc := service.NewDashboardService(dashRepo, transferRepo, nil, logger)
	transferSvc := service.NewTransferService(transferRepo, dashRepo, nil, logger)

	r := gin.New()
	v1 := r.Group("/api/v1")
	dh := NewDashboardHandler(dashboardSvc)
	th := NewTransferHandler(transferSvc)
	v1.POST("/transfers", th.Create)
	v1.GET("/transfers", th.List)
	v1.POST("/transfers/:id/arrive", th.ConfirmArrival)
	v1.POST("/transfers/:id/cancel", th.Cancel)
	v1.POST("/tasks/:id/dispatch", dh.Dispatch)
	return r, db
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}) (apiResp, int) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp apiResp
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp, w.Code
}

// 完整 HTTP 流程：发起 → 在途派单被拒(409) → 到达确认 → 地块更新/恢复派 → 再派单成功。
func TestTransferFlow_HTTP(t *testing.T) {
	r, db := newTransferRouter(t)

	// 1. 非空闲农机发起被拒（409）
	resp, status := doJSON(t, r, http.MethodPost, "/api/v1/transfers", map[string]string{
		"machineCode": "NJ-2026-001", "toField": "东河麦田",
		"estimatedArrival": "2026-09-26 10:00:00", "applicant": "小王",
	})
	if status != http.StatusConflict || resp.Code != constants.CodeConflict {
		t.Fatalf("busy machine create = %d/%d %s", status, resp.Code, resp.Message)
	}

	// 2. 参数校验失败（400）
	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/transfers", map[string]string{
		"machineCode": "NJ-2026-002", "toField": "东河麦田",
		"estimatedArrival": "bad-time", "applicant": "小王",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid eta status = %d", status)
	}

	// 3. 空闲农机发起成功（201）
	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/transfers", map[string]string{
		"machineCode": "NJ-2026-002", "toField": "东河麦田",
		"estimatedArrival": "2026-09-26 10:00:00", "applicant": "小王",
	})
	if status != http.StatusCreated || resp.Code != constants.CodeOK {
		t.Fatalf("create = %d/%d %s", status, resp.Code, resp.Message)
	}
	var created struct {
		TransferID string `json:"transferId"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(resp.Data, &created); err != nil || created.TransferID == "" {
		t.Fatalf("create data invalid: %s err %v", string(resp.Data), err)
	}

	// 4. 重复发起冲突
	_, status = doJSON(t, r, http.MethodPost, "/api/v1/transfers", map[string]string{
		"machineCode": "NJ-2026-002", "toField": "北岭 1 号田",
		"estimatedArrival": "2026-09-26 11:00:00", "applicant": "小王",
	})
	if status != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, want 409", status)
	}

	// 5. 在途期间派单被拒
	_, status = doJSON(t, r, http.MethodPost, "/api/v1/tasks/t9/dispatch", nil)
	if status != http.StatusConflict {
		t.Fatalf("dispatch in transit status = %d, want 409", status)
	}

	// 6. 按农机查看记录
	resp, status = doJSON(t, r, http.MethodGet, "/api/v1/transfers?machineCode=NJ-2026-002", nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	var listData struct {
		Items []model.Transfer `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(resp.Data, &listData); err != nil || listData.Total != 1 {
		t.Fatalf("list data = %s err %v", string(resp.Data), err)
	}
	row := listData.Items[0]
	if row.FromField != "南湾稻田" || row.ToField != "东河麦田" || row.Status != constants.TransferStatusInTransit {
		t.Fatalf("unexpected transfer row: %+v", row)
	}

	// 7. 到达确认
	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/transfers/"+created.TransferID+"/arrive", nil)
	if status != http.StatusOK {
		t.Fatalf("arrive = %d %s", status, resp.Message)
	}
	var m model.Machine
	if err := db.Where("code = ?", "NJ-2026-002").First(&m).Error; err != nil {
		t.Fatalf("load machine: %v", err)
	}
	if m.Field != "东河麦田" || m.Status != constants.MachineIdle {
		t.Fatalf("machine after arrive = field %s status %s", m.Field, m.Status)
	}

	// 8. 恢复派单成功
	_, status = doJSON(t, r, http.MethodPost, "/api/v1/tasks/t9/dispatch", nil)
	if status != http.StatusOK {
		t.Fatalf("dispatch after arrive status = %d, want 200", status)
	}
}

// 取消分支：取消后回到原地块、记录失败原因，并可重新发起。
func TestTransferCancelFlow_HTTP(t *testing.T) {
	r, db := newTransferRouter(t)

	resp, status := doJSON(t, r, http.MethodPost, "/api/v1/transfers", map[string]string{
		"machineCode": "NJ-2026-002", "toField": "东河麦田",
		"estimatedArrival": "2026-09-26 12:00:00", "applicant": "小李",
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, resp.Message)
	}
	var created struct {
		TransferID string `json:"transferId"`
	}
	_ = json.Unmarshal(resp.Data, &created)

	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/transfers/"+created.TransferID+"/cancel", map[string]string{"operator": "小李"})
	if status != http.StatusOK {
		t.Fatalf("cancel = %d %s", status, resp.Message)
	}

	var m model.Machine
	if err := db.Where("code = ?", "NJ-2026-002").First(&m).Error; err != nil {
		t.Fatalf("load machine: %v", err)
	}
	if m.Field != "南湾稻田" || m.Status != constants.MachineIdle {
		t.Fatalf("machine after cancel = field %s status %s", m.Field, m.Status)
	}
	var tr model.Transfer
	if err := db.First(&tr, "id = ?", created.TransferID).Error; err != nil {
		t.Fatalf("load transfer: %v", err)
	}
	if tr.Status != constants.TransferStatusCancelled || tr.FailReason == "" {
		t.Fatalf("transfer after cancel = status %s reason %q", tr.Status, tr.FailReason)
	}

	// 已取消的单子不能重复操作
	if _, code := doJSON(t, r, http.MethodPost, "/api/v1/transfers/"+created.TransferID+"/arrive", nil); code != http.StatusConflict {
		t.Fatalf("arrive cancelled = %d, want 409", code)
	}
}

// 非申请人不能取消他人的转场单（403）。
func TestTransferCancelByOtherApplicant_HTTP(t *testing.T) {
	r, _ := newTransferRouter(t)

	resp, status := doJSON(t, r, http.MethodPost, "/api/v1/transfers", map[string]string{
		"machineCode": "NJ-2026-002", "toField": "东河麦田",
		"estimatedArrival": "2026-09-26 13:00:00", "applicant": "小李",
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, resp.Message)
	}
	var created struct {
		TransferID string `json:"transferId"`
	}
	_ = json.Unmarshal(resp.Data, &created)

	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/transfers/"+created.TransferID+"/cancel", map[string]string{"operator": "陌生人"})
	if status != http.StatusForbidden || resp.Code != constants.CodeForbidden {
		t.Fatalf("cancel by other = %d/%d %s, want 403", status, resp.Code, resp.Message)
	}

	// 未带操作人时返回 400
	if _, code := doJSON(t, r, http.MethodPost, "/api/v1/transfers/"+created.TransferID+"/cancel", map[string]string{}); code != http.StatusBadRequest {
		t.Fatalf("cancel without operator code = %d, want 400", code)
	}

	// 转场单仍然在途，未被改动
	if _, code := doJSON(t, r, http.MethodGet, "/api/v1/transfers?machineCode=NJ-2026-002", nil); code != http.StatusOK {
		t.Fatalf("list = %d", code)
	}
}

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"log/slog"

	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// stubStore 转场仓储测试替身（实现 service.TransferStore）。
type stubStore struct {
	startErr  error
	finishErr error
}

func (s *stubStore) StartTransfer(in repository.StartTransferInput) (*model.Transfer, error) {
	if s.startErr != nil {
		return nil, s.startErr
	}
	return &model.Transfer{
		ID: in.ID, MachineCode: in.MachineCode, ToField: in.ToField,
		FromField: "南湾稻田", Status: constants.TransferInTransit,
	}, nil
}

func (s *stubStore) FinishTransfer(id string, arrived bool, reason string) (*model.Transfer, error) {
	if s.finishErr != nil {
		return nil, s.finishErr
	}
	st := constants.TransferArrived
	if !arrived {
		st = constants.TransferCancelled
	}
	return &model.Transfer{ID: id, Status: st, FailReason: reason}, nil
}

func (s *stubStore) FindByID(string) (*model.Transfer, error) { return &model.Transfer{}, nil }
func (s *stubStore) ListByMachine(string) ([]model.Transfer, error) {
	return []model.Transfer{{ID: "tr-1", MachineCode: "NJ-2026-002", Status: constants.TransferInTransit}}, nil
}
func (s *stubStore) HasActiveTransfer(string) (bool, error) { return false, nil }

func newTransferRouter(store service.TransferStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	svc := service.NewTransferService(store, redisClient, slog.Default())
	h := NewTransferHandler(svc)
	r := gin.New()
	r.GET("/api/v1/transfers", h.List)
	r.POST("/api/v1/transfers", h.Start)
	r.POST("/api/v1/transfers/:id/arrive", h.Arrive)
	r.POST("/api/v1/transfers/:id/cancel", h.Cancel)
	return r
}

func perform(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestStartTransferHTTPSuccess(t *testing.T) {
	r := newTransferRouter(&stubStore{})
	future := time.Now().Add(2 * time.Hour).Format(constants.TransferTimeLayout)
	w := perform(r, http.MethodPost, "/api/v1/transfers",
		`{"machineCode":"NJ-2026-002","toField":"东河麦田","expectedArriveAt":"`+future+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("code=%d message=%s", resp.Code, resp.Message)
	}
	var tr model.Transfer
	if err := json.Unmarshal(resp.Data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Status != constants.TransferInTransit {
		t.Fatalf("status=%q", tr.Status)
	}
}

func TestStartTransferHTTPBadRequest(t *testing.T) {
	r := newTransferRouter(&stubStore{})
	w := perform(r, http.MethodPost, "/api/v1/transfers", `{"machineCode":"NJ-1"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestStartTransferHTTPConflict(t *testing.T) {
	r := newTransferRouter(&stubStore{startErr: repository.ErrMachineNotIdle})
	future := time.Now().Add(2 * time.Hour).Format(constants.TransferTimeLayout)
	w := perform(r, http.MethodPost, "/api/v1/transfers",
		`{"machineCode":"NJ-2026-001","toField":"东河麦田","expectedArriveAt":"`+future+`"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d body=%s", w.Code, w.Body.String())
	}
	var resp apiResp
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != constants.CodeConflict {
		t.Fatalf("code=%d", resp.Code)
	}
}

func TestArriveTransferHTTPSuccessAndConflict(t *testing.T) {
	r := newTransferRouter(&stubStore{})
	w := perform(r, http.MethodPost, "/api/v1/transfers/tr-1/arrive", "")
	if w.Code != http.StatusOK {
		t.Fatalf("arrive status=%d", w.Code)
	}

	r2 := newTransferRouter(&stubStore{finishErr: repository.ErrTransferNotActive})
	w2 := perform(r2, http.MethodPost, "/api/v1/transfers/tr-1/arrive", "")
	if w2.Code != http.StatusConflict {
		t.Fatalf("want 409 for ended transfer, got %d", w2.Code)
	}
}

func TestCancelTransferHTTPEmptyBody(t *testing.T) {
	r := newTransferRouter(&stubStore{})
	w := perform(r, http.MethodPost, "/api/v1/transfers/tr-1/cancel", "")
	if w.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestListTransfersHTTP(t *testing.T) {
	r := newTransferRouter(&stubStore{})
	w := perform(r, http.MethodGet, "/api/v1/transfers?machineCode=NJ-2026-002", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d", w.Code)
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []model.Transfer `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].MachineCode != "NJ-2026-002" {
		t.Fatalf("unexpected items: %+v", resp.Data.Items)
	}
}

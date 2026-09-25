package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/redis/go-redis/v9"
)

// fakeTransferStore 转场仓储的测试替身。
type fakeTransferStore struct {
	active     bool
	startErr   error
	finishErr  error
	lastInput  repository.StartTransferInput
	finishedID string
	arrived    bool
	reason     string
	transfers  []model.Transfer
}

func (f *fakeTransferStore) StartTransfer(in repository.StartTransferInput) (*model.Transfer, error) {
	f.lastInput = in
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &model.Transfer{ID: in.ID, MachineCode: in.MachineCode, ToField: in.ToField, Status: constants.TransferInTransit}, nil
}

func (f *fakeTransferStore) FinishTransfer(id string, arrived bool, reason string) (*model.Transfer, error) {
	f.finishedID = id
	f.arrived = arrived
	f.reason = reason
	if f.finishErr != nil {
		return nil, f.finishErr
	}
	st := constants.TransferArrived
	if !arrived {
		st = constants.TransferCancelled
	}
	return &model.Transfer{ID: id, Status: st, FailReason: reason}, nil
}

func (f *fakeTransferStore) FindByID(string) (*model.Transfer, error) {
	return &model.Transfer{}, nil
}

func (f *fakeTransferStore) ListByMachine(string) ([]model.Transfer, error) {
	return f.transfers, nil
}

func (f *fakeTransferStore) HasActiveTransfer(string) (bool, error) {
	return f.active, nil
}

// newTestTransferService 使用不可达的 Redis；缓存失效失败仅记录告警，不影响业务断言。
func newTestTransferService(store TransferStore) *TransferService {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	return NewTransferService(store, client, slog.Default())
}

func TestStartTransferValidation(t *testing.T) {
	svc := newTestTransferService(&fakeTransferStore{})
	future := time.Now().Add(time.Hour).Format(constants.TransferTimeLayout)

	cases := []struct {
		name       string
		toField    string
		arriveAt   string
		wantSubstr string
	}{
		{"empty target", "", future, "目标地块"},
		{"bad time format", "东河田", "2026/06/01 08:00", "时间格式"},
		{"past time", "东河田", time.Now().Add(-time.Hour).Format(constants.TransferTimeLayout), "必须晚于"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Start(context.Background(), "NJ-1", tc.toField, tc.arriveAt, "")
			var bizErr *apperrors.BusinessError
			if !errors.As(err, &bizErr) {
				t.Fatalf("want BusinessError, got %v", err)
			}
			if !strings.Contains(bizErr.Message, tc.wantSubstr) {
				t.Fatalf("message=%q, want contains %q", bizErr.Message, tc.wantSubstr)
			}
		})
	}
}

func TestStartTransferOnlyIdleMachine(t *testing.T) {
	svc := newTestTransferService(&fakeTransferStore{startErr: repository.ErrMachineNotIdle})
	future := time.Now().Add(time.Hour).Format(constants.TransferTimeLayout)
	_, err := svc.Start(context.Background(), "NJ-2026-002", "东河田", future, "")
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) || bizErr.Code != constants.CodeConflict {
		t.Fatalf("want conflict business error, got %v", err)
	}
}

func TestStartTransferDuplicateRejected(t *testing.T) {
	svc := newTestTransferService(&fakeTransferStore{startErr: repository.ErrActiveTransferExist})
	future := time.Now().Add(time.Hour).Format(constants.TransferTimeLayout)
	_, err := svc.Start(context.Background(), "NJ-2026-002", "东河田", future, "")
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) || bizErr.Code != constants.CodeConflict {
		t.Fatalf("want conflict for duplicate transfer, got %v", err)
	}
}

func TestStartTransferSuccessDefaultsApplicant(t *testing.T) {
	store := &fakeTransferStore{}
	svc := newTestTransferService(store)
	future := time.Now().Add(2 * time.Hour).Format(constants.TransferTimeLayout)
	tr, err := svc.Start(context.Background(), "NJ-2026-002", "东河田", future, "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if tr.Status != constants.TransferInTransit {
		t.Fatalf("status=%q", tr.Status)
	}
	if store.lastInput.Applicant != constants.TransferDefaultApplicant {
		t.Fatalf("default applicant = %q", store.lastInput.Applicant)
	}
}

func TestCancelTransferDefaultReason(t *testing.T) {
	store := &fakeTransferStore{}
	svc := newTestTransferService(store)
	if _, err := svc.Cancel(context.Background(), "tr-1", "  "); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if store.arrived || store.finishedID != "tr-1" {
		t.Fatalf("unexpected cancel state: arrived=%v id=%s", store.arrived, store.finishedID)
	}
	if store.reason != constants.TransferCancelReason {
		t.Fatalf("reason = %q, want default %q", store.reason, constants.TransferCancelReason)
	}
}

func TestArriveTransfer(t *testing.T) {
	store := &fakeTransferStore{}
	svc := newTestTransferService(store)
	if _, err := svc.Arrive(context.Background(), "tr-1"); err != nil {
		t.Fatalf("arrive: %v", err)
	}
	if !store.arrived {
		t.Fatal("arrive flag should be true")
	}
}

func TestFinishAlreadyEndedConflict(t *testing.T) {
	svc := newTestTransferService(&fakeTransferStore{finishErr: repository.ErrTransferNotActive})
	if _, err := svc.Arrive(context.Background(), "tr-1"); err == nil {
		t.Fatal("want conflict error for ended transfer")
	}
}

func TestHasActiveTransferGuard(t *testing.T) {
	store := &fakeTransferStore{active: true}
	svc := newTestTransferService(store)
	active, err := svc.HasActiveTransfer("NJ-2026-002")
	if err != nil || !active {
		t.Fatalf("active=%v err=%v", active, err)
	}
}

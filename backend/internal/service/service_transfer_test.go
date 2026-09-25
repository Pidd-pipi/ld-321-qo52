package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	bizerr "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var transferTestSeq atomic.Int64

// newTransferTestEnv 构造基于内存 SQLite 的转场测试环境（纯 Go 驱动，无需 CGO）。
func newTransferTestEnv(t *testing.T) (*TransferService, *DashboardService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:svc-transfer-%d?mode=memory&cache=shared", transferTestSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&model.Machine{}, &model.FarmTask{}, &model.Transfer{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	machines := []model.Machine{
		{ID: "m1", Code: "NJ-2026-001", Name: "东方红 1804", Field: "北岭 1 号田", Status: constants.MachineWorking, CurrentTask: "春耕翻地"},
		{ID: "m2", Code: "NJ-2026-002", Name: "雷沃谷神收割机", Field: "南湾稻田", Status: constants.MachineIdle, CurrentTask: constants.MachineIdleTask},
		{ID: "m3", Code: "NJ-2026-003", Name: "中联履带拖拉机", Field: "西坡旱地", Status: constants.MachineRepair, CurrentTask: "液压检修"},
	}
	if err := db.Create(&machines).Error; err != nil {
		t.Fatalf("seed machines: %v", err)
	}
	task := model.FarmTask{ID: "t2", Type: "播种", Field: "西坡旱地", Status: constants.TaskPending, RecommendedMachine: "NJ-2026-002", RecommendedDriver: "何燕"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dashRepo := repository.NewDashboardRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	transferSvc := NewTransferService(transferRepo, dashRepo, nil, logger)
	dashboardSvc := NewDashboardService(dashRepo, transferRepo, nil, logger)
	return transferSvc, dashboardSvc, db
}

const etaTomorrow = "2026-09-26 09:30:00"

func findMachine(t *testing.T, db *gorm.DB, code string) model.Machine {
	t.Helper()
	var m model.Machine
	if err := db.Where("code = ?", code).First(&m).Error; err != nil {
		t.Fatalf("load machine %s: %v", code, err)
	}
	return m
}

// 只有空闲农机可以发起转场。
func TestTransferCreate_IdleOnly(t *testing.T) {
	svc, _, _ := newTransferTestEnv(t)
	ctx := context.Background()

	cases := []struct {
		name        string
		code        string
		wantBusyErr bool
	}{
		{"作业中农机拒绝", "NJ-2026-001", true},
		{"维修中农机拒绝", "NJ-2026-003", true},
		{"空闲农机允许", "NJ-2026-002", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := svc.Create(ctx, tc.code, "东河麦田", etaTomorrow, "调度员小王")
			if tc.wantBusyErr {
				var busy *bizerr.MachineBusyError
				if !errors.As(err, &busy) {
					t.Fatalf("want MachineBusyError, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("create transfer: %v", err)
			}
			if tr.Status != constants.TransferStatusInTransit || tr.FromField != "南湾稻田" || tr.ToField != "东河麦田" {
				t.Fatalf("unexpected transfer: %+v", tr)
			}
		})
	}
}

// 发起后农机进入在途状态，所属地块暂不改变。
func TestTransferCreate_SetsTransferringKeepsField(t *testing.T) {
	svc, _, db := newTransferTestEnv(t)
	tr, err := svc.Create(context.Background(), "NJ-2026-002", "东河麦田", etaTomorrow, "调度员小王")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	m := findMachine(t, db, "NJ-2026-002")
	if m.Status != constants.MachineTransferring {
		t.Errorf("machine status = %s, want %s", m.Status, constants.MachineTransferring)
	}
	if m.Field != "南湾稻田" {
		t.Errorf("field changed in transit: %s", m.Field)
	}
	_ = tr
}

// 同一台农机不能同时存在两张未结束（在途）转场单。
func TestTransferCreate_DuplicateRejected(t *testing.T) {
	svc, _, _ := newTransferTestEnv(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "NJ-2026-002", "东河麦田", etaTomorrow, "调度员小王"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, "NJ-2026-002", "北岭 1 号田", etaTomorrow, "调度员小王")
	var busy *bizerr.MachineBusyError
	if !errors.As(err, &busy) {
		t.Fatalf("second create want MachineBusyError, got %v", err)
	}
}

// 在途期间调度按钮必须拒绝派单。
func TestDispatch_RejectedWhileInTransit(t *testing.T) {
	transferSvc, dashboardSvc, _ := newTransferTestEnv(t)
	ctx := context.Background()
	if _, err := transferSvc.Create(ctx, "NJ-2026-002", "东河麦田", etaTomorrow, "调度员小王"); err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	_, err := dashboardSvc.Dispatch(ctx, "t2")
	var blocked *bizerr.DispatchBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("dispatch want DispatchBlockedError, got %v", err)
	}
	if blocked.MachineCode != "NJ-2026-002" || blocked.TransferID == "" {
		t.Fatalf("blocked error missing context: %+v", blocked)
	}
}

// 到达确认后：所属地块更新为目标地块、状态恢复空闲、恢复可派。
func TestTransferArrive_UpdatesFieldAndResumesDispatch(t *testing.T) {
	transferSvc, dashboardSvc, db := newTransferTestEnv(t)
	ctx := context.Background()
	tr, err := transferSvc.Create(ctx, "NJ-2026-002", "东河麦田", etaTomorrow, "调度员小王")
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	arrived, err := transferSvc.ConfirmArrival(ctx, tr.ID)
	if err != nil {
		t.Fatalf("arrive: %v", err)
	}
	if arrived.Status != constants.TransferStatusArrived || arrived.ConfirmedAt == nil {
		t.Fatalf("unexpected arrived transfer: %+v", arrived)
	}
	m := findMachine(t, db, "NJ-2026-002")
	if m.Field != "东河麦田" || m.Status != constants.MachineIdle || m.CurrentTask != constants.MachineIdleTask {
		t.Fatalf("machine after arrival = field %s status %s task %s", m.Field, m.Status, m.CurrentTask)
	}

	// 到达后派单恢复正常。
	res, err := dashboardSvc.Dispatch(ctx, "t2")
	if err != nil {
		t.Fatalf("dispatch after arrival: %v", err)
	}
	if res["status"] != constants.TaskDispatched {
		t.Fatalf("dispatch result = %v", res)
	}
	m = findMachine(t, db, "NJ-2026-002")
	if m.Status != constants.MachineWorking {
		t.Fatalf("machine status after dispatch = %s, want 作业中", m.Status)
	}
}

// 申请人取消后：转场单标记已取消并记录失败原因，农机回到原地块并恢复可派。
func TestTransferCancel_BackToOrigin(t *testing.T) {
	transferSvc, dashboardSvc, db := newTransferTestEnv(t)
	ctx := context.Background()
	tr, err := transferSvc.Create(ctx, "NJ-2026-002", "东河麦田", etaTomorrow, "调度员小王")
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	cancelled, err := transferSvc.Cancel(ctx, tr.ID, "调度员小王")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != constants.TransferStatusCancelled || cancelled.FailReason == "" {
		t.Fatalf("unexpected cancelled transfer: %+v", cancelled)
	}
	m := findMachine(t, db, "NJ-2026-002")
	if m.Field != "南湾稻田" || m.Status != constants.MachineIdle {
		t.Fatalf("machine after cancel = field %s status %s", m.Field, m.Status)
	}

	// 取消后旧单结束，可重新发起，且派单不再被拦。
	if _, err := transferSvc.Create(ctx, "NJ-2026-002", "北岭 1 号田", etaTomorrow, "调度员小王"); err != nil {
		t.Fatalf("re-create after cancel: %v", err)
	}
	if _, err := transferSvc.Cancel(ctx, tr.ID, "调度员小王"); err == nil {
		t.Fatal("cancelling an already cancelled transfer must fail")
	}
	if _, err := transferSvc.ConfirmArrival(ctx, tr.ID); err == nil {
		t.Fatal("arriving a cancelled transfer must fail")
	}

	// 取消第二张在途单后派单恢复。
	list, err := transferSvc.ListByMachine(ctx, "NJ-2026-002")
	if err != nil || len(list) != 2 {
		t.Fatalf("list transfers = %d, err %v", len(list), err)
	}
	open := list[0]
	if _, err := transferSvc.Cancel(ctx, open.ID, "调度员小王"); err != nil {
		t.Fatalf("cancel second transfer: %v", err)
	}
	if _, err := dashboardSvc.Dispatch(ctx, "t2"); err != nil {
		t.Fatalf("dispatch after cancel: %v", err)
	}
}

// 转场记录可按农机查看，页面字段含起点、目标、状态和失败原因。
func TestTransferList_ByMachine(t *testing.T) {
	transferSvc, _, _ := newTransferTestEnv(t)
	ctx := context.Background()
	tr, err := transferSvc.Create(ctx, "NJ-2026-002", "东河麦田", etaTomorrow, "调度员小王")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := transferSvc.Cancel(ctx, tr.ID, "调度员小王"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	mine, err := transferSvc.ListByMachine(ctx, "NJ-2026-002")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(mine) != 1 {
		t.Fatalf("want 1 record for NJ-2026-002, got %d", len(mine))
	}
	row := mine[0]
	if row.FromField != "南湾稻田" || row.ToField != "东河麦田" || row.Status != constants.TransferStatusCancelled || row.FailReason == "" {
		t.Fatalf("record fields missing: %+v", row)
	}

	others, err := transferSvc.ListByMachine(ctx, "NJ-2026-001")
	if err != nil || len(others) != 0 {
		t.Fatalf("want 0 record for NJ-2026-001, got %d err %v", len(others), err)
	}
}

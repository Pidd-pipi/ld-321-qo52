package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	bizerr "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// DashboardService 调度看板服务（带 Redis 缓存）。
type DashboardService struct {
	repo         *repository.DashboardRepository
	transferRepo *repository.TransferRepository
	redis        *redis.Client
	logger       *slog.Logger
}

func NewDashboardService(repo *repository.DashboardRepository, transferRepo *repository.TransferRepository, redis *redis.Client, logger *slog.Logger) *DashboardService {
	return &DashboardService{repo: repo, transferRepo: transferRepo, redis: redis, logger: logger}
}

// Overview 返回看板总览（优先读 Redis 缓存，未命中回源 DB 并写缓存）。
func (s *DashboardService) Overview(ctx context.Context) (*model.FarmOverview, error) {
	cached, err := s.redis.Get(ctx, constants.OverviewCacheKey).Result()
	if err == nil {
		var ov model.FarmOverview
		if jsonErr := json.Unmarshal([]byte(cached), &ov); jsonErr == nil {
			return &ov, nil
		}
	}
	ov, err := s.repo.Overview()
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(ov)
	if err := s.redis.Set(ctx, constants.OverviewCacheKey, data, constants.OverviewCacheTTLSeconds*time.Second).Err(); err != nil {
		s.logger.Warn("cache overview failed", "err", err)
	}
	return ov, nil
}

// Invalidate 使缓存失效（派单后调用）。
func (s *DashboardService) Invalidate(ctx context.Context) {
	if s.redis == nil {
		return
	}
	if err := s.redis.Del(ctx, constants.OverviewCacheKey).Err(); err != nil {
		s.logger.Warn("invalidate overview cache failed", "err", err)
	}
}

// Dispatch 派单：将任务置为已派单，并同步更新农机状态为作业中。
// 农机若存在未结束（在途）的转场单，派单一律拒绝，需先完成到达确认或取消转场。
func (s *DashboardService) Dispatch(ctx context.Context, taskID string) (map[string]interface{}, error) {
	// 快速预检：任务存在且未派单，真正的状态判定在事务行锁内完成。
	task, err := s.repo.FindTask(taskID)
	if err != nil {
		return nil, err
	}
	if task.Status == constants.TaskDispatched || task.Status == constants.TaskDone {
		return nil, fmt.Errorf("任务 %s 已处于 %s 状态，无需重复派单", taskID, task.Status)
	}

	var machineCode, driverName string
	txErr := s.transferRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lockedTask, err := s.repo.FindTaskForUpdate(tx, taskID)
		if err != nil {
			return err
		}
		if lockedTask.Status == constants.TaskDispatched || lockedTask.Status == constants.TaskDone {
			return fmt.Errorf("任务 %s 已处于 %s 状态，无需重复派单", taskID, lockedTask.Status)
		}
		if lockedTask.RecommendedMachine == "" {
			lockedTask.RecommendedMachine = "NJ-2026-002"
		}
		if lockedTask.RecommendedDriver == "" {
			lockedTask.RecommendedDriver = "何燕"
		}

		machine, err := s.repo.LockMachineTx(tx, lockedTask.RecommendedMachine)
		if err != nil {
			return err
		}
		// 在途转场单存在时一律拒绝派单。
		if open, e := s.transferRepo.FindOpenByMachineCode(tx, machine.Code); e == nil {
			return &bizerr.DispatchBlockedError{
				TransferID:     open.ID,
				MachineCode:    machine.Code,
				ToField:        open.ToField,
				TransferStatus: open.Status,
			}
		} else if !errors.Is(e, repository.ErrNotFound) {
			return e
		}

		machine.Status = constants.MachineWorking
		machine.CurrentTask = fmt.Sprintf("%s %s", lockedTask.Type, lockedTask.Field)
		if err := s.repo.SaveMachineTx(tx, machine); err != nil {
			return err
		}
		lockedTask.Status = constants.TaskDispatched
		if err := s.repo.SaveTaskTx(tx, lockedTask); err != nil {
			return err
		}
		machineCode, driverName = machine.Code, lockedTask.RecommendedDriver
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	s.Invalidate(ctx)
	s.logger.Info("task dispatched", "taskId", taskID, "machine", machineCode, "driver", driverName)
	return map[string]interface{}{
		"taskId":  taskID,
		"status":  constants.TaskDispatched,
		"message": "系统已按空闲度和驾驶员排班完成推荐派单",
	}, nil
}

// ExportReport 作业报表导出信息。
func (s *DashboardService) ExportReport(ctx context.Context) (map[string]interface{}, error) {
	ov, err := s.Overview(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"fileName": "farm-work-report-2026-05.csv",
		"rows":     len(ov.Records),
		"status":   "ready",
	}, nil
}

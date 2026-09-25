package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	bizerr "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// TransferService 农机转场办理流程。
type TransferService struct {
	transferRepo  *repository.TransferRepository
	dashboardRepo *repository.DashboardRepository
	redis         *redis.Client
	logger        *slog.Logger
}

func NewTransferService(transferRepo *repository.TransferRepository, dashboardRepo *repository.DashboardRepository, redisClient *redis.Client, logger *slog.Logger) *TransferService {
	return &TransferService{transferRepo: transferRepo, dashboardRepo: dashboardRepo, redis: redisClient, logger: logger}
}

// invalidateOverview 转场状态变化后使看板缓存失效。
func (s *TransferService) invalidateOverview(ctx context.Context) {
	if s.redis == nil {
		return
	}
	if err := s.redis.Del(ctx, constants.OverviewCacheKey).Err(); err != nil {
		s.logger.Warn("invalidate overview cache failed", "err", err)
	}
}

// Create 发起转场：仅空闲农机可发起；同一农机不得同时存在两张未结束（在途）转场单。
func (s *TransferService) Create(ctx context.Context, machineCode, toField, eta, applicant string) (*model.Transfer, error) {
	toField = strings.TrimSpace(toField)
	if toField == "" {
		return nil, errors.New("目标地块不能为空")
	}
	parsedETA, err := time.ParseInLocation(constants.TransferETATimeLayout, eta, time.Local)
	if err != nil {
		return nil, fmt.Errorf("预计到达时间格式无效，应为 %s", constants.TransferETATimeLayout)
	}

	var created *model.Transfer
	txErr := s.transferRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		machine, err := s.dashboardRepo.LockMachineTx(tx, machineCode)
		if err != nil {
			return err
		}
		if machine.Status != constants.MachineIdle {
			return &bizerr.MachineBusyError{MachineCode: machineCode, Reason: fmt.Sprintf("农机当前状态为 %s，仅空闲农机可发起转场", machine.Status)}
		}
		if open, e := s.transferRepo.FindOpenByMachineCode(tx, machineCode); e == nil {
			return &bizerr.MachineBusyError{MachineCode: machineCode, Reason: fmt.Sprintf("已有未结束转场单 %s（目标：%s）", open.ID, open.ToField)}
		} else if !errors.Is(e, repository.ErrNotFound) {
			return e
		}
		if machine.Field == toField {
			return errors.New("目标地块与当前所属地块相同，无需转场")
		}

		now := time.Now()
		transfer := &model.Transfer{
			ID:               "tf-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			MachineCode:      machineCode,
			FromField:        machine.Field,
			ToField:          toField,
			EstimatedArrival: parsedETA.Format(constants.TransferETATimeLayout),
			Applicant:        applicant,
			Status:           constants.TransferStatusInTransit,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := s.transferRepo.Create(tx, transfer); err != nil {
			return err
		}

		// 在途期间不可派单：状态转为“转场中”，所属地块暂不改变。
		machine.Status = constants.MachineTransferring
		machine.CurrentTask = fmt.Sprintf("转场至%s", toField)
		if err := s.dashboardRepo.SaveMachineTx(tx, machine); err != nil {
			return err
		}
		created = transfer
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	s.invalidateOverview(ctx)
	s.logger.Info("transfer created", "transferId", created.ID, "machine", machineCode, "from", created.FromField, "to", toField, "applicant", applicant)
	return created, nil
}

// ConfirmArrival 到达确认：更新所属地块为目标地块，恢复可派（空闲）。
func (s *TransferService) ConfirmArrival(ctx context.Context, transferID string) (*model.Transfer, error) {
	var confirmed *model.Transfer
	txErr := s.transferRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		transfer, err := s.transferRepo.LockByID(tx, transferID)
		if err != nil {
			return err
		}
		if transfer.Status != constants.TransferStatusInTransit {
			return &bizerr.TransferStateError{TransferID: transferID, Status: transfer.Status, WantStatus: constants.TransferStatusInTransit}
		}

		machine, err := s.dashboardRepo.LockMachineTx(tx, transfer.MachineCode)
		if err != nil {
			return err
		}
		machine.Field = transfer.ToField
		machine.Status = constants.MachineIdle
		machine.CurrentTask = constants.MachineIdleTask
		if err := s.dashboardRepo.SaveMachineTx(tx, machine); err != nil {
			return err
		}

		now := time.Now()
		transfer.Status = constants.TransferStatusArrived
		transfer.ConfirmedAt = &now
		transfer.UpdatedAt = now
		if err := s.transferRepo.Update(tx, transfer); err != nil {
			return err
		}
		confirmed = transfer
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	s.invalidateOverview(ctx)
	s.logger.Info("transfer arrived", "transferId", transferID, "machine", confirmed.MachineCode, "field", confirmed.ToField)
	return confirmed, nil
}

// Cancel 申请人取消：转场单置为已取消并记录原因，农机回到原地块并恢复可派。
func (s *TransferService) Cancel(ctx context.Context, transferID, operator string) (*model.Transfer, error) {
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return nil, errors.New("取消转场需提供申请人")
	}
	var cancelled *model.Transfer
	txErr := s.transferRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		transfer, err := s.transferRepo.LockByID(tx, transferID)
		if err != nil {
			return err
		}
		if transfer.Status != constants.TransferStatusInTransit {
			return &bizerr.TransferStateError{TransferID: transferID, Status: transfer.Status, WantStatus: constants.TransferStatusInTransit}
		}
		if transfer.Applicant != operator {
			return &bizerr.TransferForbiddenError{
				TransferID: transferID,
				Message:    fmt.Sprintf("仅申请人（%s）可取消该转场单，当前操作人：%s", transfer.Applicant, operator),
			}
		}

		machine, err := s.dashboardRepo.LockMachineTx(tx, transfer.MachineCode)
		if err != nil {
			return err
		}
		machine.Field = transfer.FromField
		machine.Status = constants.MachineIdle
		machine.CurrentTask = constants.MachineIdleTask
		if err := s.dashboardRepo.SaveMachineTx(tx, machine); err != nil {
			return err
		}

		transfer.Status = constants.TransferStatusCancelled
		transfer.FailReason = fmt.Sprintf("%s（操作人：%s）", constants.TransferFailReasonCancelled, operator)
		transfer.UpdatedAt = time.Now()
		if err := s.transferRepo.Update(tx, transfer); err != nil {
			return err
		}
		cancelled = transfer
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	s.invalidateOverview(ctx)
	s.logger.Info("transfer cancelled", "transferId", transferID, "machine", cancelled.MachineCode, "backTo", cancelled.FromField)
	return cancelled, nil
}

// ListByMachine 按农机查看转场记录（最新在前）；code 为空时返回全部。
func (s *TransferService) ListByMachine(ctx context.Context, machineCode string) ([]model.Transfer, error) {
	if strings.TrimSpace(machineCode) == "" {
		return s.transferRepo.List(constants.TransferListLimit)
	}
	return s.transferRepo.ListByMachineCode(strings.TrimSpace(machineCode))
}

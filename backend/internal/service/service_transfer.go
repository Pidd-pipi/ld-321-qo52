package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// TransferStore 转场数据访问接口，便于测试替换。
type TransferStore interface {
	StartTransfer(in repository.StartTransferInput) (*model.Transfer, error)
	FinishTransfer(id string, arrived bool, failReason string) (*model.Transfer, error)
	FindByID(id string) (*model.Transfer, error)
	ListByMachine(machineCode string) ([]model.Transfer, error)
	HasActiveTransfer(machineCode string) (bool, error)
}

// TransferService 农机转场办理服务。
type TransferService struct {
	repo   TransferStore
	redis  *redis.Client
	logger *slog.Logger
}

func NewTransferService(repo TransferStore, redisClient *redis.Client, logger *slog.Logger) *TransferService {
	return &TransferService{repo: repo, redis: redisClient, logger: logger}
}

// HasActiveTransfer 农机是否存在未结束的在途转场单（派单前的调度校验）。
func (s *TransferService) HasActiveTransfer(machineCode string) (bool, error) {
	return s.repo.HasActiveTransfer(machineCode)
}

// Start 发起转场：仅空闲农机可发起，登记目标地块与预计到达时间。
func (s *TransferService) Start(ctx context.Context, machineCode, toField, expectedArriveAt, applicant string) (*model.Transfer, error) {
	toField = strings.TrimSpace(toField)
	expectedArriveAt = strings.TrimSpace(expectedArriveAt)
	if toField == "" || expectedArriveAt == "" {
		return nil, apperrors.New(constants.CodeBadRequest, "目标地块和预计到达时间不能为空")
	}
	expected, err := time.ParseInLocation(constants.TransferTimeLayout, expectedArriveAt, time.Local)
	if err != nil {
		return nil, apperrors.New(constants.CodeBadRequest, fmt.Sprintf("预计到达时间格式应为 %s", constants.TransferTimeLayout))
	}
	if !expected.After(time.Now()) {
		return nil, apperrors.New(constants.CodeBadRequest, "预计到达时间必须晚于当前时间")
	}
	if applicant = strings.TrimSpace(applicant); applicant == "" {
		applicant = constants.TransferDefaultApplicant
	}

	tr, err := s.repo.StartTransfer(repository.StartTransferInput{
		ID:               fmt.Sprintf("%s-%s", constants.TransferIDPrefix, uuid.NewString()),
		MachineCode:      machineCode,
		ToField:          toField,
		ExpectedArriveAt: expectedArriveAt,
		Applicant:        applicant,
	})
	if err != nil {
		return nil, s.wrapStartError(err, machineCode)
	}
	s.invalidateOverview(ctx)
	s.logger.Info("transfer started",
		"transferId", tr.ID, "machine", tr.MachineCode, "from", tr.FromField, "to", tr.ToField)
	return tr, nil
}

// Arrive 到达确认：更新所属地块为目标地块，农机恢复空闲可派。
func (s *TransferService) Arrive(ctx context.Context, id string) (*model.Transfer, error) {
	tr, err := s.repo.FinishTransfer(id, true, "")
	if err != nil {
		return nil, s.wrapFinishError(err, id)
	}
	s.invalidateOverview(ctx)
	s.logger.Info("transfer arrived", "transferId", id, "machine", tr.MachineCode, "field", tr.ToField)
	return tr, nil
}

// Cancel 申请人取消转场：农机回到原地块并恢复空闲，记录失败原因。
func (s *TransferService) Cancel(ctx context.Context, id, reason string) (*model.Transfer, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = constants.TransferCancelReason
	}
	tr, err := s.repo.FinishTransfer(id, false, reason)
	if err != nil {
		return nil, s.wrapFinishError(err, id)
	}
	s.invalidateOverview(ctx)
	s.logger.Info("transfer cancelled", "transferId", id, "machine", tr.MachineCode, "reason", reason)
	return tr, nil
}

// List 查看转场记录，可按农机编号过滤。
func (s *TransferService) List(machineCode string) ([]model.Transfer, error) {
	return s.repo.ListByMachine(strings.TrimSpace(machineCode))
}

// invalidateOverview 使看板缓存失效，状态变更即时可见。
func (s *TransferService) invalidateOverview(ctx context.Context) {
	if err := s.redis.Del(ctx, constants.OverviewCacheKey).Err(); err != nil {
		s.logger.Warn("invalidate overview cache failed", "err", err)
	}
}

func (s *TransferService) wrapStartError(err error, machineCode string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return apperrors.New(constants.CodeNotFound, fmt.Sprintf("农机 %s 不存在", machineCode))
	case errors.Is(err, repository.ErrMachineNotIdle):
		return apperrors.Conflict(fmt.Sprintf("农机 %s 当前不是空闲状态，只有空闲农机可以发起转场", machineCode))
	case errors.Is(err, repository.ErrActiveTransferExist):
		return apperrors.Conflict(fmt.Sprintf("农机 %s 已有未结束的转场单，不能重复发起", machineCode))
	default:
		return err
	}
}

func (s *TransferService) wrapFinishError(err error, id string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return apperrors.New(constants.CodeNotFound, fmt.Sprintf("转场单 %s 不存在", id))
	case errors.Is(err, repository.ErrTransferNotActive):
		return apperrors.Conflict(fmt.Sprintf("转场单 %s 已结束，不能重复操作", id))
	default:
		return err
	}
}

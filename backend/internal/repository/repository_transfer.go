package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 转场流程哨兵错误，service 层据此转换为业务错误。
var (
	ErrMachineNotIdle      = errors.New("machine is not idle")
	ErrActiveTransferExist = errors.New("active transfer already exists")
	ErrTransferNotActive   = errors.New("transfer is not in transit")
)

// TransferRepository 农机转场单数据访问。
type TransferRepository struct {
	db *gorm.DB
}

func NewTransferRepository(db *gorm.DB) *TransferRepository {
	return &TransferRepository{db: db}
}

// StartTransferInput 发起转场入参（快照字段由事务内锁定的农机行填充）。
type StartTransferInput struct {
	ID               string
	MachineCode      string
	ToField          string
	ExpectedArriveAt string
	Applicant        string
}

// StartTransfer 在同一事务内校验农机状态并发起转场：
// 仅空闲农机可发起；同一农机不允许存在第二张未结束转场单；
// 校验通过后农机状态置为转场中，并写入在途转场单。
func (r *TransferRepository) StartTransfer(in StartTransferInput) (*model.Transfer, error) {
	tr := &model.Transfer{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var machine model.Machine
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&machine, "code = ?", in.MachineCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock machine: %w", err)
		}
		if machine.Status != constants.MachineIdle {
			return ErrMachineNotIdle
		}
		var active int64
		if err := tx.Model(&model.Transfer{}).
			Where("machine_code = ? AND status = ?", in.MachineCode, constants.TransferInTransit).
			Count(&active).Error; err != nil {
			return fmt.Errorf("count active transfer: %w", err)
		}
		if active > 0 {
			return ErrActiveTransferExist
		}

		tr = &model.Transfer{
			ID:               in.ID,
			MachineCode:      machine.Code,
			MachineName:      machine.Name,
			FromField:        machine.Field,
			ToField:          in.ToField,
			ExpectedArriveAt: in.ExpectedArriveAt,
			Status:           constants.TransferInTransit,
			Applicant:        in.Applicant,
		}
		machine.Status = constants.MachineTransferring
		if err := tx.Save(&machine).Error; err != nil {
			return fmt.Errorf("set machine transferring: %w", err)
		}
		if err := tx.Create(tr).Error; err != nil {
			return fmt.Errorf("create transfer: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("start transfer for %s: %w", in.MachineCode, err)
	}
	return tr, nil
}

// FinishTransfer 结束在途转场单。
// arrived=true：到达确认，更新所属地块为目标地块，农机恢复空闲可派；
// arrived=false：申请人取消，农机回到原地块并恢复空闲。
func (r *TransferRepository) FinishTransfer(id string, arrived bool, failReason string) (*model.Transfer, error) {
	var result *model.Transfer
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var tr model.Transfer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&tr, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock transfer: %w", err)
		}
		if tr.Status != constants.TransferInTransit {
			return ErrTransferNotActive
		}

		var machine model.Machine
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&machine, "code = ?", tr.MachineCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock machine: %w", err)
		}

		now := time.Now()
		if arrived {
			tr.Status = constants.TransferArrived
			tr.ArrivedAt = &now
			machine.Field = tr.ToField
		} else {
			tr.Status = constants.TransferCancelled
			tr.CancelledAt = &now
			tr.FailReason = failReason
			machine.Field = tr.FromField
		}
		machine.Status = constants.MachineIdle

		if err := tx.Save(&tr).Error; err != nil {
			return fmt.Errorf("save transfer: %w", err)
		}
		if err := tx.Save(&machine).Error; err != nil {
			return fmt.Errorf("restore machine: %w", err)
		}
		result = &tr
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finish transfer %s: %w", id, err)
	}
	return result, nil
}

// FindByID 按主键查找转场单。
func (r *TransferRepository) FindByID(id string) (*model.Transfer, error) {
	var tr model.Transfer
	err := r.db.First(&tr, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find transfer: %w", err)
	}
	return &tr, nil
}

// ListByMachine 按农机编号查询转场记录（新单在前）；编号为空时返回全部。
func (r *TransferRepository) ListByMachine(machineCode string) ([]model.Transfer, error) {
	query := r.db.Model(&model.Transfer{}).Order("created_at DESC")
	if machineCode != "" {
		query = query.Where("machine_code = ?", machineCode)
	}
	var transfers []model.Transfer
	if err := query.Find(&transfers).Error; err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	return transfers, nil
}

// HasActiveTransfer 判断农机是否存在未结束（在途）转场单。
func (r *TransferRepository) HasActiveTransfer(machineCode string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.Transfer{}).
		Where("machine_code = ? AND status = ?", machineCode, constants.TransferInTransit).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count active transfer: %w", err)
	}
	return count > 0, nil
}

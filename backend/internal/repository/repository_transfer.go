package repository

import (
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TransferRepository 农机转场单数据访问。
type TransferRepository struct {
	db *gorm.DB
}

func NewTransferRepository(db *gorm.DB) *TransferRepository {
	return &TransferRepository{db: db}
}

// DB 返回事务句柄，供 service 层包裹多表事务。
func (r *TransferRepository) DB() *gorm.DB {
	return r.db
}

// Create 在事务内创建转场单。
func (r *TransferRepository) Create(tx *gorm.DB, t *model.Transfer) error {
	if err := tx.Create(t).Error; err != nil {
		return fmt.Errorf("create transfer: %w", err)
	}
	return nil
}

// FindByID 按主键查找转场单。
func (r *TransferRepository) FindByID(id string) (*model.Transfer, error) {
	var t model.Transfer
	err := r.db.First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find transfer: %w", err)
	}
	return &t, nil
}

// LockByID 在事务内锁定转场单行。
func (r *TransferRepository) LockByID(tx *gorm.DB, id string) (*model.Transfer, error) {
	var t model.Transfer
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock transfer: %w", err)
	}
	return &t, nil
}

// Update 在事务内更新转场单。
func (r *TransferRepository) Update(tx *gorm.DB, t *model.Transfer) error {
	if err := tx.Save(t).Error; err != nil {
		return fmt.Errorf("update transfer: %w", err)
	}
	return nil
}

// FindOpenByMachineCode 查找农机当前未结束（在途）的转场单；无则返回 ErrNotFound。
func (r *TransferRepository) FindOpenByMachineCode(tx *gorm.DB, code string) (*model.Transfer, error) {
	var t model.Transfer
	err := tx.Where("machine_code = ? AND status = ?", code, constants.TransferStatusInTransit).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find open transfer: %w", err)
	}
	return &t, nil
}

// ListByMachineCode 按农机查看转场记录，最新发起的在前。
func (r *TransferRepository) ListByMachineCode(code string) ([]model.Transfer, error) {
	var list []model.Transfer
	if err := r.db.Where("machine_code = ?", code).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list transfers by machine: %w", err)
	}
	return list, nil
}

// List 查看全部转场记录，最新发起的在前。
func (r *TransferRepository) List(limit int) ([]model.Transfer, error) {
	var list []model.Transfer
	q := r.db.Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	return list, nil
}

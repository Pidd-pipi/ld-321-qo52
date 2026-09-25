package repository

import (
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound 哨兵错误。
var ErrNotFound = errors.New("record not found")

// DashboardRepository 看板数据访问。
type DashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// Overview 组装看板总览。
func (r *DashboardRepository) Overview() (*model.FarmOverview, error) {
	ov := &model.FarmOverview{}
	if err := r.db.Order("score DESC").Find(&ov.Items).Error; err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	if err := r.db.Find(&ov.Machines).Error; err != nil {
		return nil, fmt.Errorf("load machines: %w", err)
	}
	if err := r.db.Find(&ov.Tasks).Error; err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	if err := r.db.Order("captured_at DESC").Limit(50).Find(&ov.Tracks).Error; err != nil {
		return nil, fmt.Errorf("load tracks: %w", err)
	}
	if err := r.db.Order("work_date DESC").Find(&ov.Records).Error; err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}
	if err := r.db.Find(&ov.Maintenance).Error; err != nil {
		return nil, fmt.Errorf("load maintenance: %w", err)
	}
	if err := r.db.Find(&ov.Drivers).Error; err != nil {
		return nil, fmt.Errorf("load drivers: %w", err)
	}
	if err := r.db.Order("created_at DESC").Limit(constants.TransferListLimit).Find(&ov.Transfers).Error; err != nil {
		return nil, fmt.Errorf("load transfers: %w", err)
	}
	ov.Board = r.board(ov)
	ov.Stats = r.stats(ov.Records)
	return ov, nil
}

// board 计算调度看板。
func (r *DashboardRepository) board(ov *model.FarmOverview) model.DispatchBoard {
	var idle, working int
	var workingList, dueList []string
	for _, m := range ov.Machines {
		switch m.Status {
		case "空闲":
			idle++
		case "作业中":
			working++
			workingList = append(workingList, fmt.Sprintf("%s %s", m.Code, m.CurrentTask))
		}
	}
	for _, m := range ov.Maintenance {
		dueList = append(dueList, fmt.Sprintf("%s %s", m.MachineCode, m.Title))
	}
	return model.DispatchBoard{
		TodayTodos:      len(ov.Tasks),
		IdleMachines:    idle,
		WorkingMachines: workingList,
		DueMaintenance:  dueList,
		SevenDayAreas:   []int{96, 122, 138, 166, 203, 88, 156},
		TrendLabels:     []string{"5/25", "5/26", "5/27", "5/28", "5/29", "5/30", "5/31"},
	}
}

// stats 汇总作业统计。
func (r *DashboardRepository) stats(records []model.WorkRecord) model.Stats {
	var s model.Stats
	for _, rec := range records {
		s.TotalAreaMu += rec.AreaMu
		s.TotalHours += rec.ActualHours
		s.FuelCost += rec.FuelCost
	}
	return s
}

// FindTask 查找任务。
func (r *DashboardRepository) FindTask(id string) (*model.FarmTask, error) {
	var t model.FarmTask
	err := r.db.First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	return &t, nil
}

// UpdateTask 更新任务。
func (r *DashboardRepository) UpdateTask(t *model.FarmTask) error {
	if err := r.db.Save(t).Error; err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

// FindMachineByCode 按农机编号查找农机。
func (r *DashboardRepository) FindMachineByCode(code string) (*model.Machine, error) {
	var m model.Machine
	err := r.db.First(&m, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find machine by code: %w", err)
	}
	return &m, nil
}

// UpdateMachine 更新农机。
func (r *DashboardRepository) UpdateMachine(m *model.Machine) error {
	if err := r.db.Save(m).Error; err != nil {
		return fmt.Errorf("update machine: %w", err)
	}
	return nil
}

// LockMachineTx 在事务内锁定农机行，保证并发派单/转场时状态检查不串单。
func (r *DashboardRepository) LockMachineTx(tx *gorm.DB, code string) (*model.Machine, error) {
	var m model.Machine
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock machine: %w", err)
	}
	return &m, nil
}

// SaveMachineTx 在事务内保存农机。
func (r *DashboardRepository) SaveMachineTx(tx *gorm.DB, m *model.Machine) error {
	if err := tx.Save(m).Error; err != nil {
		return fmt.Errorf("save machine: %w", err)
	}
	return nil
}

// FindTaskForUpdate 在事务内锁定任务行。
func (r *DashboardRepository) FindTaskForUpdate(tx *gorm.DB, id string) (*model.FarmTask, error) {
	var t model.FarmTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock task: %w", err)
	}
	return &t, nil
}

// SaveTaskTx 在事务内保存任务。
func (r *DashboardRepository) SaveTaskTx(tx *gorm.DB, t *model.FarmTask) error {
	if err := tx.Save(t).Error; err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// UserRepository 用户数据访问。
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var u model.User
	err := r.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &u, nil
}

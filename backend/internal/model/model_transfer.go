package model

import "time"

// Transfer 农机转场单：记录农机在地块之间的转场过程。
//
// 状态流转：在途（发起）→ 已到达（确认到达，所属地块更新为目标地块）
//
//	在途（发起）→ 已取消（申请人取消，所属地块恢复为起点地块）
type Transfer struct {
	ID               string     `gorm:"primaryKey;size:40" json:"id"`
	MachineCode      string     `gorm:"size:32;not null;index:idx_transfer_machine_status,priority:1" json:"machineCode"`
	FromField        string     `gorm:"size:64;not null" json:"fromField"`
	ToField          string     `gorm:"size:64;not null" json:"toField"`
	EstimatedArrival string     `gorm:"size:32;not null" json:"estimatedArrival"`
	Applicant        string     `gorm:"size:64;not null" json:"applicant"`
	Status           string     `gorm:"size:20;not null;index:idx_transfer_machine_status,priority:2" json:"status"`
	FailReason       string     `gorm:"size:255;not null;default:''" json:"failReason"`
	ConfirmedAt      *time.Time `gorm:"index" json:"confirmedAt,omitempty"`
	CreatedAt        time.Time  `gorm:"index" json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

package model

import "time"

// Transfer 农机转场单：记录农机在地块之间的转场办理过程。
type Transfer struct {
	ID               string     `gorm:"primaryKey;size:48" json:"id"`
	MachineCode      string     `gorm:"size:32;index;not null" json:"machineCode"`
	MachineName      string     `gorm:"size:64" json:"machineName"`
	FromField        string     `gorm:"size:64" json:"fromField"`
	ToField          string     `gorm:"size:64" json:"toField"`
	ExpectedArriveAt string     `gorm:"size:32" json:"expectedArriveAt"`
	Status           string     `gorm:"size:16;index" json:"status"`
	FailReason       string     `gorm:"size:255" json:"failReason"`
	Applicant        string     `gorm:"size:64" json:"applicant"`
	CreatedAt        time.Time  `json:"createdAt"`
	ArrivedAt        *time.Time `json:"arrivedAt"`
	CancelledAt      *time.Time `json:"cancelledAt"`
}

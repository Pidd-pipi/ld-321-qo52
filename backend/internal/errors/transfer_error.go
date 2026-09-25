package errors

import "fmt"

// MachineBusyError 农机当前不可发起转场：不是空闲状态，或已存在未结束的转场单。
type MachineBusyError struct {
	MachineCode string
	Reason      string
}

func (e *MachineBusyError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return fmt.Sprintf("农机 %s 当前不可发起转场", e.MachineCode)
}

// TransferStateError 转场单当前状态不允许执行目标操作。
type TransferStateError struct {
	TransferID string
	Status     string
	WantStatus string
}

func (e *TransferStateError) Error() string {
	return fmt.Sprintf("转场单 %s 当前状态为 %s，仅 %s 状态可执行该操作", e.TransferID, e.Status, e.WantStatus)
}

// TransferForbiddenError 操作者无权操作该转场单（如非申请人尝试取消）。
type TransferForbiddenError struct {
	TransferID string
	Message    string
}

func (e *TransferForbiddenError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("无权操作转场单 %s", e.TransferID)
}

// DispatchBlockedError 在途期间调度被拒绝。
type DispatchBlockedError struct {
	TransferID     string
	MachineCode    string
	ToField        string
	TransferStatus string
}

func (e *DispatchBlockedError) Error() string {
	if e.TransferID != "" {
		return fmt.Sprintf("农机 %s 正在转场前往 %s（转场单 %s），到达确认前不可派单", e.MachineCode, e.ToField, e.TransferID)
	}
	return fmt.Sprintf("农机 %s 正在转场，到达确认前不可派单", e.MachineCode)
}

package dto

// CreateTransferRequest 发起转场请求。
type CreateTransferRequest struct {
	MachineCode      string `json:"machineCode" binding:"required,max=32"`
	ToField          string `json:"toField" binding:"required,max=64"`
	ExpectedArriveAt string `json:"expectedArriveAt" binding:"required,max=32"`
	Applicant        string `json:"applicant" binding:"max=64"`
}

// CancelTransferRequest 取消转场请求，失败原因可留空。
type CancelTransferRequest struct {
	Reason string `json:"reason" binding:"max=255"`
}

// TransferQuery 转场记录查询参数。
type TransferQuery struct {
	MachineCode string `form:"machineCode"`
}

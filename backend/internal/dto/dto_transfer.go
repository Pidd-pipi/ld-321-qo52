package dto

import "github.com/go-playground/validator/v10"

// validate 转场接口共用的校验器。
var validate = validator.New()

// CreateTransferRequest 发起转场请求体。
type CreateTransferRequest struct {
	MachineCode      string `json:"machineCode" validate:"required,max=32"`
	ToField          string `json:"toField" validate:"required,max=64"`
	EstimatedArrival string `json:"estimatedArrival" validate:"required,datetime=2006-01-02 15:04:05"`
	Applicant        string `json:"applicant" validate:"required,max=64"`
}

// Validate 校验发起转场参数。
func (r *CreateTransferRequest) Validate() error {
	return validate.Struct(r)
}

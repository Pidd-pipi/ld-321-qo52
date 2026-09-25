package handler

import (
	"errors"
	"net/http"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	bizerr "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/agridispatch/agridispatch/internal/util"
	"github.com/gin-gonic/gin"
)

// TransferHandler 农机转场办理处理器。
type TransferHandler struct {
	transferSvc *service.TransferService
}

func NewTransferHandler(transferSvc *service.TransferService) *TransferHandler {
	return &TransferHandler{transferSvc: transferSvc}
}

// Create 发起转场。
func (h *TransferHandler) Create(c *gin.Context) {
	var req dto.CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "参数无效：machineCode/toField/estimatedArrival(YYYY-MM-DD HH:mm:ss)/applicant 必填")
		return
	}
	if err := req.Validate(); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, err.Error())
		return
	}
	transfer, err := h.transferSvc.Create(c.Request.Context(), req.MachineCode, req.ToField, req.EstimatedArrival, req.Applicant)
	if err != nil {
		h.failTransfer(c, err)
		return
	}
	c.JSON(http.StatusCreated, util.Response{Code: constants.CodeOK, Message: "ok", Data: gin.H{
		"transferId": transfer.ID,
		"status":     transfer.Status,
		"message":    "转场单已发起，农机在途期间不可派单",
	}})
}

// ConfirmArrival 到达确认。
func (h *TransferHandler) ConfirmArrival(c *gin.Context) {
	transfer, err := h.transferSvc.ConfirmArrival(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.failTransfer(c, err)
		return
	}
	util.OK(c, gin.H{
		"transferId":  transfer.ID,
		"machineCode": transfer.MachineCode,
		"status":      transfer.Status,
		"field":       transfer.ToField,
		"message":     "到达已确认，所属地块已更新，农机恢复可派",
	})
}

// Cancel 申请人取消转场。
func (h *TransferHandler) Cancel(c *gin.Context) {
	var body struct {
		Operator string `json:"operator"`
	}
	_ = c.ShouldBindJSON(&body)
	transfer, err := h.transferSvc.Cancel(c.Request.Context(), c.Param("id"), body.Operator)
	if err != nil {
		h.failTransfer(c, err)
		return
	}
	util.OK(c, gin.H{
		"transferId":  transfer.ID,
		"machineCode": transfer.MachineCode,
		"status":      transfer.Status,
		"field":       transfer.FromField,
		"message":     "转场已取消，农机回到原地块",
	})
}

// List 转场记录，支持按农机编号筛选：GET /transfers?machineCode=...
func (h *TransferHandler) List(c *gin.Context) {
	list, err := h.transferSvc.ListByMachine(c.Request.Context(), c.Query("machineCode"))
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"items": list, "total": len(list)})
}

// failTransfer 将转场业务错误映射为统一响应。
func (h *TransferHandler) failTransfer(c *gin.Context, err error) {
	var busy *bizerr.MachineBusyError
	var state *bizerr.TransferStateError
	var forbidden *bizerr.TransferForbiddenError
	switch {
	case errors.As(err, &forbidden):
		util.Fail(c, http.StatusForbidden, constants.CodeForbidden, err.Error())
	case errors.As(err, &busy), errors.As(err, &state):
		util.Fail(c, http.StatusConflict, constants.CodeConflict, err.Error())
	default:
		util.FailError(c, err)
	}
}

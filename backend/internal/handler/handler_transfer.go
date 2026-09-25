package handler

import (
	"net/http"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
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

// Start 发起转场：登记目标地块与预计到达时间。
func (h *TransferHandler) Start(c *gin.Context) {
	var req dto.CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "请求参数不合法: "+err.Error())
		return
	}
	tr, err := h.transferSvc.Start(c.Request.Context(), req.MachineCode, req.ToField, req.ExpectedArriveAt, req.Applicant)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.Created(c, tr)
}

// Arrive 到达确认：更新所属地块并恢复可派。
func (h *TransferHandler) Arrive(c *gin.Context) {
	tr, err := h.transferSvc.Arrive(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, tr)
}

// Cancel 申请人取消转场：回到原地块。
func (h *TransferHandler) Cancel(c *gin.Context) {
	var req dto.CancelTransferRequest
	// 取消允许空 body，绑定错误仅在 body 非法 JSON 时发生，可忽略继续走默认原因。
	_ = c.ShouldBindJSON(&req)
	tr, err := h.transferSvc.Cancel(c.Request.Context(), c.Param("id"), req.Reason)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, tr)
}

// List 转场记录查询，可按农机编号过滤。
func (h *TransferHandler) List(c *gin.Context) {
	var query dto.TransferQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "查询参数不合法: "+err.Error())
		return
	}
	transfers, err := h.transferSvc.List(query.MachineCode)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"items": transfers})
}

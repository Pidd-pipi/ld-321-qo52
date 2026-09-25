package constants

// 农机转场状态
const (
	TransferInTransit = "在途"
	TransferArrived   = "已到达"
	TransferCancelled = "已取消"
)

// 转场单相关默认值
const (
	TransferIDPrefix         = "tr"
	TransferCancelReason     = "申请人取消"
	TransferDefaultApplicant = "系统管理员"
	TransferTimeLayout       = "2006-01-02 15:04"
)

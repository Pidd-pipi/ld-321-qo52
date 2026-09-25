package constants

// 农机转场流程相关常量。
const (
	// TransferStatusInTransit 在途：转场单已发起，农机不可派单。
	TransferStatusInTransit = "在途"
	// TransferStatusArrived 已到达：到达确认完成，所属地块已更新。
	TransferStatusArrived = "已到达"
	// TransferStatusCancelled 已取消：申请人取消，农机回到原地块。
	TransferStatusCancelled = "已取消"

	// TransferFailReasonCancelled 申请人取消时记录的失败/终止原因。
	TransferFailReasonCancelled = "申请人取消"
	// MachineIdleTask 空闲农机的当前任务占位文案。
	MachineIdleTask = "可派单"

	// TransferETATimeLayout 预计到达时间的时间格式。
	TransferETATimeLayout = "2006-01-02 15:04:05"

	// TransferListLimit 未指定农机时返回的转场记录上限。
	TransferListLimit = 200
)

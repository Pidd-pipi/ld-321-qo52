export const APP_NAME = 'AgriDispatch 农机调度';
export const API_BASE = '/api';

// 农机状态
export const MACHINE_IDLE = '空闲';
export const MACHINE_WORKING = '作业中';
export const MACHINE_REPAIR = '维修中';
export const MACHINE_TRANSFERRING = '转场中';

// 转场单状态
export const TRANSFER_IN_TRANSIT = '在途';
export const TRANSFER_ARRIVED = '已到达';
export const TRANSFER_CANCELLED = '已取消';

export const STATUS_COLORS: Record<string, string> = {
  空闲: 'success',
  作业中: 'warning',
  维修中: 'danger',
  转场中: 'primary',
  待派单: 'info',
  已派单: 'warning',
  已完成: 'success',
  在途: 'primary',
  已到达: 'success',
  已取消: 'info',
};

export const APP_NAME = 'AgriDispatch 农机调度';
export const API_BASE = '/api';

export const MACHINE_IDLE = '空闲';
export const MACHINE_WORKING = '作业中';
export const MACHINE_REPAIR = '维修中';
export const MACHINE_TRANSFERRING = '转场中';

export const TRANSFER_IN_TRANSIT = '在途';
export const TRANSFER_ARRIVED = '已到达';
export const TRANSFER_CANCELLED = '已取消';

export const STATUS_COLORS: Record<string, string> = {
  [MACHINE_IDLE]: 'success',
  [MACHINE_WORKING]: 'warning',
  [MACHINE_REPAIR]: 'danger',
  [MACHINE_TRANSFERRING]: 'primary',
  待派单: 'info',
  已派单: 'warning',
  已完成: 'success',
  [TRANSFER_IN_TRANSIT]: 'primary',
  [TRANSFER_ARRIVED]: 'success',
  [TRANSFER_CANCELLED]: 'info',
};

// 后端业务错误码 → HTTP 状态码映射中与转场相关的冲突码。
export const HTTP_CONFLICT = 409;

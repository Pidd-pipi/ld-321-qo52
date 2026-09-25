<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  MACHINE_IDLE,
  STATUS_COLORS,
  TRANSFER_ARRIVED,
  TRANSFER_CANCELLED,
  TRANSFER_IN_TRANSIT,
} from '../constants/app.constants';
import { logger } from '../logger/logger';
import {
  arriveTransfer,
  cancelTransfer,
  createTransfer,
  type CreateTransferPayload,
} from '../services/transfer.service';
import type { Machine, Transfer } from '../types/domain';

const props = defineProps<{
  transfers: Transfer[];
  machines: Machine[];
  defaultMachineCode?: string;
}>();

const emit = defineEmits<{ (e: 'changed'): void }>();

const filterMachine = ref('');

const formRef = ref();
const submitting = ref(false);
const form = reactive({
  machineCode: '',
  toField: '',
  expectedTime: null as Date | null,
});

const formRules = {
  machineCode: [{ required: true, message: '请选择要转场的农机', trigger: 'change' }],
  toField: [{ required: true, message: '请填写目标地块', trigger: 'blur' }],
  expectedTime: [{ required: true, message: '请选择预计到达时间', trigger: 'change' }],
};

// 仅空闲农机可以发起转场。
const idleMachines = computed(() => props.machines.filter((m) => m.status === MACHINE_IDLE));

// 按农机筛选转场记录；新单在后端已按创建时间倒序返回。
const filteredTransfers = computed(() =>
  filterMachine.value
    ? props.transfers.filter((t) => t.machineCode === filterMachine.value)
    : props.transfers,
);

watch(
  () => props.defaultMachineCode,
  (code) => {
    if (code) {
      form.machineCode = code;
    }
  },
  { immediate: true },
);

const pad = (value: number) => String(value).padStart(2, '0');

const formatExpectedTime = (time: Date) =>
  `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(time.getDate())} ${pad(
    time.getHours(),
  )}:${pad(time.getMinutes())}`;

const handleCreate = async () => {
  if (!formRef.value || !form.expectedTime) {
    return;
  }
  try {
    await formRef.value.validate();
  } catch {
    return;
  }
  const payload: CreateTransferPayload = {
    machineCode: form.machineCode,
    toField: form.toField.trim(),
    expectedArriveAt: formatExpectedTime(form.expectedTime),
  };
  submitting.value = true;
  try {
    await createTransfer(payload);
    ElMessage.success('转场单已发起，农机进入在途状态');
    form.machineCode = '';
    form.toField = '';
    form.expectedTime = null;
    emit('changed');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '发起转场失败');
    logger.error('create transfer failed', err);
  } finally {
    submitting.value = false;
  }
};

const handleArrive = async (row: Transfer) => {
  try {
    await ElMessageBox.confirm(
      `确认 ${row.machineCode} 已到达「${row.toField}」？确认后所属地块将更新并恢复可派单。`,
      '到达确认',
      { type: 'success', confirmButtonText: '确认到达', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  try {
    await arriveTransfer(row.id);
    ElMessage.success('已确认到达，农机恢复可派单');
    emit('changed');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '到达确认失败');
  }
};

const handleCancel = async (row: Transfer) => {
  let reason = '';
  try {
    const { value } = await ElMessageBox.prompt('取消后农机将回到原地块，可填写失败原因（选填）', '取消转场', {
      confirmButtonText: '确认取消',
      cancelButtonText: '再想想',
      inputType: 'textarea',
      inputPlaceholder: '如：道路临时管制 / 天气原因，不填默认“申请人取消”',
      inputValue: '',
    });
    reason = value;
  } catch {
    return;
  }
  try {
    await cancelTransfer(row.id, reason);
    ElMessage.success('转场已取消，农机回到原地块');
    emit('changed');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '取消转场失败');
  }
};
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-lg font-black">农机转场办理</h2>
      <el-select
        v-model="filterMachine"
        placeholder="按农机查看转场记录"
        size="small"
        clearable
        filterable
        class="w-60"
      >
        <el-option
          v-for="machine in machines"
          :key="machine.code"
          :label="`${machine.code} ${machine.name}`"
          :value="machine.code"
        />
      </el-select>
    </div>

    <el-form
      ref="formRef"
      :model="form"
      :rules="formRules"
      label-width="92px"
      class="mb-4 rounded-md bg-slate-50 p-3"
      @submit.prevent
    >
      <div class="grid gap-3 md:grid-cols-4">
        <el-form-item label="转场农机" prop="machineCode">
          <el-select v-model="form.machineCode" placeholder="仅空闲农机可发起" filterable class="w-full">
            <el-option
              v-for="machine in idleMachines"
              :key="machine.code"
              :label="`${machine.code}（${machine.field}）`"
              :value="machine.code"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="目标地块" prop="toField">
          <el-input v-model="form.toField" placeholder="如：东河麦田" maxlength="64" />
        </el-form-item>
        <el-form-item label="预计到达" prop="expectedTime">
          <el-date-picker
            v-model="form.expectedTime"
            type="datetime"
            placeholder="选择预计到达时间"
            format="YYYY-MM-DD HH:mm"
            class="w-full"
          />
        </el-form-item>
        <el-form-item label=" " class="md:justify-end">
          <el-button type="primary" :loading="submitting" @click="handleCreate">发起转场</el-button>
        </el-form-item>
      </div>
    </el-form>

    <el-table :data="filteredTransfers" size="small" empty-text="暂无转场记录">
      <el-table-column prop="machineCode" label="农机" width="130">
        <template #default="{ row }">
          <div>{{ row.machineCode }}</div>
          <div class="text-xs text-slate-400">{{ row.machineName }}</div>
        </template>
      </el-table-column>
      <el-table-column label="起点 → 目标" min-width="200">
        <template #default="{ row }">
          <span>{{ row.fromField }}</span>
          <span class="mx-1 text-slate-400">→</span>
          <span class="font-medium">{{ row.toField }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="expectedArriveAt" label="预计到达" width="150" />
      <el-table-column prop="applicant" label="申请人" width="100" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="STATUS_COLORS[row.status] ?? 'info'" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="failReason" label="失败原因" min-width="160">
        <template #default="{ row }">
          <span :class="row.failReason ? 'text-rose-600' : 'text-slate-300'">
            {{ row.failReason || '—' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <template v-if="row.status === TRANSFER_IN_TRANSIT">
            <el-button size="small" type="success" @click="handleArrive(row)">到达确认</el-button>
            <el-button size="small" type="danger" plain @click="handleCancel(row)">取消</el-button>
          </template>
          <span v-else-if="row.status === TRANSFER_ARRIVED" class="text-xs text-slate-400">
            {{ row.arrivedAt || '' }}
          </span>
          <span v-else-if="row.status === TRANSFER_CANCELLED" class="text-xs text-slate-400">
            {{ row.cancelledAt || '' }}
          </span>
        </template>
      </el-table-column>
    </el-table>
  </section>
</template>

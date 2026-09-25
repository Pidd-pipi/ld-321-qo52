<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { STATUS_COLORS, TRANSFER_IN_TRANSIT } from '../constants/app.constants';
import { arriveTransfer, cancelTransfer } from '../services/transfer.service';
import type { Machine, Transfer } from '../types/domain';

const props = defineProps<{
  transfers: Transfer[];
  machines: Machine[];
  machineCode: string;
}>();
const emit = defineEmits<{
  (e: 'refresh'): void;
  (e: 'update:machineCode', value: string): void;
}>();

const busyId = ref('');

const machineOptions = computed(() => props.machines.map((m) => m.code));

const handleArrive = async (row: Transfer) => {
  try {
    await ElMessageBox.confirm(`确认 ${row.machineCode} 已到达「${row.toField}」？确认后所属地块将更新并恢复可派。`, '到达确认', {
      type: 'success',
      confirmButtonText: '已到达',
      cancelButtonText: '再等等',
    });
  } catch {
    return;
  }
  busyId.value = row.id;
  try {
    const res = await arriveTransfer(row.id);
    ElMessage.success(res.message);
    emit('refresh');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '到达确认失败');
  } finally {
    busyId.value = '';
  }
};

const handleCancel = async (row: Transfer) => {
  try {
    await ElMessageBox.confirm(`申请人「${row.applicant}」取消后，${row.machineCode} 将回到起点「${row.fromField}」，确认取消该转场单？`, '取消转场', {
      type: 'warning',
      confirmButtonText: '确认取消',
      cancelButtonText: '继续转场',
    });
  } catch {
    return;
  }
  busyId.value = row.id;
  try {
    const res = await cancelTransfer(row.id, row.applicant);
    ElMessage.success(res.message);
    emit('refresh');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '取消转场失败');
  } finally {
    busyId.value = '';
  }
};
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <div class="mb-3 flex items-center justify-between">
      <h2 class="text-lg font-black">农机转场办理</h2>
      <el-select
        :model-value="machineCode"
        placeholder="按农机筛选"
        size="small"
        clearable
        class="w-44"
        @update:model-value="emit('update:machineCode', $event || '')"
      >
        <el-option v-for="code in machineOptions" :key="code" :label="code" :value="code" />
      </el-select>
    </div>

    <el-alert
      type="info"
      :closable="false"
      class="mb-3"
      title="流程：仅空闲农机可发起转场 → 在途期间拒绝派单 → 到达确认后更新所属地块并恢复可派；申请人可取消，取消后农机回到原地块。"
    />

    <el-table :data="transfers" size="small" empty-text="暂无转场记录">
      <el-table-column prop="machineCode" label="农机编号" width="130" />
      <el-table-column prop="fromField" label="起点地块" width="130" />
      <el-table-column label="目标地块" width="130">
        <template #default="{ row }">
          <span class="font-semibold text-emerald-700">{{ row.toField }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="estimatedArrival" label="预计到达" width="170" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="STATUS_COLORS[row.status] || 'info'" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="失败原因" min-width="160">
        <template #default="{ row }">
          <span :class="row.failReason ? 'text-rose-600' : 'text-slate-400'">
            {{ row.failReason || '—' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="applicant" label="申请人" width="100" />
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <template v-if="row.status === TRANSFER_IN_TRANSIT">
            <el-button size="small" type="success" :loading="busyId === row.id" @click="handleArrive(row)">到达确认</el-button>
            <el-button size="small" type="danger" plain :disabled="busyId === row.id" @click="handleCancel(row)">取消</el-button>
          </template>
          <span v-else class="text-xs text-slate-400">已结束</span>
        </template>
      </el-table-column>
    </el-table>
  </section>
</template>

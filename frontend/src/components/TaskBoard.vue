<script setup lang="ts">
import { ElMessage } from 'element-plus';
import { MACHINE_TRANSFERRING, STATUS_COLORS, TRANSFER_IN_TRANSIT } from '../constants/app.constants';
import { dispatchTask } from '../services/storage.service';
import type { FarmTask, Machine, Transfer } from '../types/domain';

const props = defineProps<{
  tasks: FarmTask[];
  machines?: Machine[];
  transfers?: Transfer[];
}>();
const emit = defineEmits<{ (e: 'dispatched'): void }>();

// 推荐农机若处于在途转场，派单按钮直接禁用（后端仍会二次拦截）。
const openTransferOf = (code: string) =>
  (props.transfers ?? []).find((t) => t.machineCode === code && t.status === TRANSFER_IN_TRANSIT);
const machineStatus = (code: string) => (props.machines ?? []).find((m) => m.code === code)?.status;

const handleDispatch = async (task: FarmTask) => {
  try {
    const result = await dispatchTask(task.id);
    ElMessage.success(result.message);
    emit('dispatched');
  } catch (err) {
    ElMessage.warning(err instanceof Error ? err.message : '派单失败');
  }
};
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <h2 class="mb-3 text-lg font-black">作业任务调度</h2>
    <div class="grid gap-3">
      <article v-for="task in tasks" :key="task.id" class="rounded-md border border-slate-200 p-3">
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <strong>{{ task.type }} · {{ task.field }}</strong>
              <el-tag :type="STATUS_COLORS[task.status]" size="small">{{ task.status }}</el-tag>
            </div>
            <p class="mt-1 text-sm text-slate-600">
              {{ task.areaMu }} 亩 · 预计 {{ task.estimatedHours }} 小时 · {{ task.plannedWindow }}
            </p>
            <p class="mt-1 text-sm text-emerald-700">
              推荐 {{ task.recommendedMachine }} / {{ task.recommendedDriver }}
            </p>
            <p
              v-if="openTransferOf(task.recommendedMachine)"
              class="mt-1 text-xs text-rose-600"
            >
              推荐农机转场前往「{{ openTransferOf(task.recommendedMachine)?.toField }}」，到达确认前不可派单
            </p>
          </div>
          <el-tooltip
            :disabled="machineStatus(task.recommendedMachine) !== MACHINE_TRANSFERRING"
            content="农机在途中，到达确认前不可派单"
            placement="top"
          >
            <span>
              <el-button
                size="small"
                type="primary"
                :disabled="!!openTransferOf(task.recommendedMachine)"
                @click="handleDispatch(task)"
              >
                一键派单
              </el-button>
            </span>
          </el-tooltip>
        </div>
      </article>
    </div>
  </section>
</template>

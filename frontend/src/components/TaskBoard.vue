<script setup lang="ts">
import { computed } from 'vue';
import { ElMessage } from 'element-plus';
import { MACHINE_TRANSFERRING, STATUS_COLORS } from '../constants/app.constants';
import { dispatchTask } from '../services/storage.service';
import type { FarmTask, Machine } from '../types/domain';

const props = defineProps<{ tasks: FarmTask[]; machines: Machine[] }>();
const emit = defineEmits<{ (e: 'dispatched'): void }>();

const machineMap = computed(() => {
  const map = new Map<string, Machine>();
  props.machines.forEach((machine) => map.set(machine.code, machine));
  return map;
});

// 推荐农机处于转场在途时，调度按钮拒绝派单。
const isTransferring = (task: FarmTask) =>
  machineMap.value.get(task.recommendedMachine)?.status === MACHINE_TRANSFERRING;

const handleDispatch = async (task: FarmTask) => {
  if (isTransferring(task)) {
    ElMessage.warning(`推荐农机 ${task.recommendedMachine} 正在转场途中，暂不可派单`);
    return;
  }
  try {
    const result = await dispatchTask(task.id);
    ElMessage.success(result.message);
    emit('dispatched');
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '派单失败');
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
              <el-tag
                v-if="isTransferring(task)"
                type="primary"
                size="small"
                effect="plain"
              >推荐农机转场中</el-tag>
            </div>
            <p class="mt-1 text-sm text-slate-600">
              {{ task.areaMu }} 亩 · 预计 {{ task.estimatedHours }} 小时 · {{ task.plannedWindow }}
            </p>
            <p class="mt-1 text-sm text-emerald-700">
              推荐 {{ task.recommendedMachine }} / {{ task.recommendedDriver }}
            </p>
          </div>
          <el-tooltip
            :disabled="!isTransferring(task)"
            content="农机转场在途，调度已拒绝，请到达确认后再派单"
            placement="top"
          >
            <span>
              <el-button
                size="small"
                type="primary"
                :disabled="isTransferring(task)"
                @click="handleDispatch(task)"
              >一键派单</el-button>
            </span>
          </el-tooltip>
        </div>
      </article>
    </div>
  </section>
</template>

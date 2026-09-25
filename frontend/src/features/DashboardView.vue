<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import CreateTransferDialog from '../components/CreateTransferDialog.vue';
import DriverRoster from '../components/DriverRoster.vue';
import MachineTable from '../components/MachineTable.vue';
import MaintenanceList from '../components/MaintenanceList.vue';
import MapTrackPanel from '../components/MapTrackPanel.vue';
import MetricCard from '../components/MetricCard.vue';
import RecordStats from '../components/RecordStats.vue';
import TaskBoard from '../components/TaskBoard.vue';
import TransferPanel from '../components/TransferPanel.vue';
import { logger } from '../logger/logger';
import { fetchFarmOverview } from '../services/storage.service';
import type { FarmOverview, Machine } from '../types/domain';

const overview = ref<FarmOverview>();
const loading = ref(true);
const error = ref('');

const reload = async () => {
  overview.value = await fetchFarmOverview();
};

onMounted(async () => {
  try {
    await reload();
    logger.info('farm overview loaded');
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败';
  } finally {
    loading.value = false;
  }
});

// 转场办理
const dialogVisible = ref(false);
const activeMachine = ref<Machine | null>(null);
const transferMachineFilter = ref('');

const fields = computed(() => [...new Set((overview.value?.machines ?? []).map((m) => m.field).filter(Boolean))]);
const visibleTransfers = computed(() => {
  const list = overview.value?.transfers ?? [];
  return transferMachineFilter.value
    ? list.filter((t) => t.machineCode === transferMachineFilter.value)
    : list;
});

const openTransfer = (machine: Machine) => {
  activeMachine.value = machine;
  dialogVisible.value = true;
};

const onDataChanged = async () => {
  try {
    await reload();
  } catch (err) {
    error.value = err instanceof Error ? err.message : '刷新失败';
  }
};
</script>

<template>
  <div class="mx-auto max-w-7xl space-y-5 px-6 py-6">
    <el-alert v-if="error" :title="error" type="error" show-icon />
    <el-skeleton v-if="loading" :rows="8" animated />

    <template v-if="overview">
      <section class="grid gap-4 md:grid-cols-4">
        <MetricCard label="今日待办" :value="String(overview.board.todayTodos)" note="调度看板实时刷新" />
        <MetricCard label="空闲农机" :value="String(overview.board.idleMachines)" note="可直接派单" />
        <MetricCard label="累计作业面积" :value="`${overview.stats.totalAreaMu}亩`" note="日报/月报统计" />
        <MetricCard label="油耗成本" :value="`¥${overview.stats.fuelCost}`" note="按作业记录汇总" />
      </section>

      <section class="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
        <TaskBoard
          :tasks="overview.tasks"
          :machines="overview.machines"
          :transfers="overview.transfers"
          @dispatched="onDataChanged"
        />
        <MapTrackPanel :tracks="overview.tracks" />
      </section>

      <MachineTable :machines="overview.machines" @transfer="openTransfer" />

      <TransferPanel
        :transfers="visibleTransfers"
        :machines="overview.machines"
        :machine-code="transferMachineFilter"
        @refresh="onDataChanged"
        @update:machine-code="transferMachineFilter = $event"
      />

      <CreateTransferDialog
        v-model="dialogVisible"
        :machine="activeMachine"
        :fields="fields"
        @created="onDataChanged"
      />

      <section class="grid gap-4 lg:grid-cols-[1fr_0.9fr]">
        <RecordStats :records="overview.records" />
        <MaintenanceList :reminders="overview.maintenance" />
      </section>

      <DriverRoster :drivers="overview.drivers" />

      <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
        <h2 class="mb-3 text-lg font-black">近 7 日作业量趋势</h2>
        <div class="grid grid-cols-7 items-end gap-2">
          <div v-for="(value, index) in overview.board.sevenDayAreas" :key="index" class="text-center">
            <div class="rounded-t bg-emerald-600" :style="{ height: `${value / 2}px` }" />
            <p class="mt-2 text-xs text-slate-500">{{ overview.board.trendLabels[index] }}</p>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

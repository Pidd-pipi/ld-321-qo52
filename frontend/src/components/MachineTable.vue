<script setup lang="ts">
import { computed, ref } from 'vue';
import { MACHINE_IDLE, MACHINE_TRANSFERRING, STATUS_COLORS } from '../constants/app.constants';
import type { Machine } from '../types/domain';

const props = defineProps<{ machines: Machine[] }>();
const emit = defineEmits<{ (e: 'transfer', machineCode: string): void }>();

const statusFilter = ref('all');
const filterOptions = [
  { label: '全部', value: 'all' },
  { label: '空闲', value: MACHINE_IDLE },
  { label: '作业中', value: '作业中' },
  { label: '维修中', value: '维修中' },
  { label: '转场中', value: MACHINE_TRANSFERRING },
];

const filteredMachines = computed(() =>
  statusFilter.value === 'all'
    ? props.machines
    : props.machines.filter((m) => m.status === statusFilter.value),
);

const requestTransfer = (machine: Machine) => emit('transfer', machine.code);
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
    <div class="mb-3 flex items-center justify-between">
      <h2 class="text-lg font-black">农机档案</h2>
      <el-select v-model="statusFilter" placeholder="按状态筛选" size="small" class="w-36">
        <el-option v-for="opt in filterOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>
    </div>
    <el-table :data="filteredMachines" size="small">
      <el-table-column prop="code" label="编号" width="120" />
      <el-table-column prop="name" label="农机" />
      <el-table-column prop="model" label="型号" width="110" />
      <el-table-column prop="horsepower" label="马力" width="80" />
      <el-table-column prop="field" label="所属地块" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="STATUS_COLORS[row.status]" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="qrCode" label="二维码" width="110" />
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-tooltip
            v-if="row.status === MACHINE_IDLE"
            content="发起跨地块转场"
            placement="top"
          >
            <el-button size="small" type="primary" plain @click="requestTransfer(row)">转场</el-button>
          </el-tooltip>
          <el-tooltip v-else content="只有空闲农机可以发起转场" placement="top">
            <el-button size="small" disabled>转场</el-button>
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>
  </section>
</template>

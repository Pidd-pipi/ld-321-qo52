<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import type { FormInstance, FormRules } from 'element-plus';
import { createTransfer } from '../services/transfer.service';
import type { Machine } from '../types/domain';

const props = defineProps<{
  modelValue: boolean;
  machine?: Machine | null;
  fields: string[];
}>();
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'created'): void;
}>();

const formRef = ref<FormInstance>();
const submitting = ref(false);

const form = reactive({
  toField: '',
  estimatedArrival: '',
  applicant: '',
});

const rules: FormRules = {
  toField: [{ required: true, message: '请选择或填写目标地块', trigger: 'change' }],
  estimatedArrival: [{ required: true, message: '请选择预计到达时间', trigger: 'change' }],
  applicant: [{ required: true, message: '请填写申请人', trigger: 'blur' }],
};

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      form.toField = '';
      form.estimatedArrival = '';
      form.applicant = '';
      formRef.value?.clearValidate();
    }
  },
);

const close = () => emit('update:modelValue', false);

const submit = async () => {
  if (!props.machine || !formRef.value) return;
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) return;
  submitting.value = true;
  try {
    await createTransfer({
      machineCode: props.machine.code,
      toField: form.toField,
      estimatedArrival: form.estimatedArrival,
      applicant: form.applicant,
    });
    ElMessage.success(`转场单已发起：${props.machine.code} → ${form.toField}`);
    emit('created');
    close();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '发起转场失败');
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <el-dialog title="发起农机转场" :model-value="modelValue" width="460px" @update:model-value="close">
    <div v-if="machine" class="mb-4 rounded-md bg-slate-50 p-3 text-sm">
      <p><span class="text-slate-500">农机：</span><strong>{{ machine.code }} {{ machine.name }}</strong></p>
      <p><span class="text-slate-500">起点地块：</span>{{ machine.field }}</p>
      <p class="text-xs text-slate-400">发起后农机进入在途状态，期间调度按钮将拒绝派单。</p>
    </div>
    <el-form ref="formRef" :model="form" :rules="rules" label-width="96px">
      <el-form-item label="目标地块" prop="toField">
        <el-select v-model="form.toField" filterable allow-create placeholder="选择或输入目标地块" class="w-full">
          <el-option v-for="field in fields.filter((f) => f !== machine?.field)" :key="field" :label="field" :value="field" />
        </el-select>
      </el-form-item>
      <el-form-item label="预计到达" prop="estimatedArrival">
        <el-date-picker
          v-model="form.estimatedArrival"
          type="datetime"
          placeholder="选择预计到达时间"
          format="YYYY-MM-DD HH:mm:ss"
          value-format="YYYY-MM-DD HH:mm:ss"
          :disabled-date="(date: Date) => date.getTime() < Date.now() - 24 * 3600 * 1000"
          class="w-full"
        />
      </el-form-item>
      <el-form-item label="申请人" prop="applicant">
        <el-input v-model="form.applicant" maxlength="64" placeholder="请填写申请人姓名" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="close">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">确认发起</el-button>
    </template>
  </el-dialog>
</template>

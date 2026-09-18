<script setup lang="ts">
import { ref, computed } from 'vue';
import { X, Save } from '@lucide/vue';
import { type Job, type Group, routes, blocks, schedules, misfires, validateJob } from './domain';
const props = defineProps<{ job: Job; groups: Group[]; busy: boolean; serverError?: string }>();
const emit = defineEmits<{ save: [job: Job]; close: [] }>();
const original = JSON.stringify(props.job);
const form = ref<Job>({ ...props.job });
const error = ref('');
defineExpose({ isDirty: computed(() => JSON.stringify(form.value) !== original) });
function save() { error.value = validateJob(form.value); if (!error.value) emit('save', structuredClone({ ...form.value })); }
</script>
<template>
  <section class="drawer" role="dialog" aria-modal="true" :aria-label="job.id ? '编辑任务' : '新增任务'">
    <header><div><small>{{ job.id ? `任务 #${job.id}` : 'BEAN' }}</small><h2>{{ job.id ? '编辑任务' : '新增任务' }}</h2></div><button class="icon" aria-label="关闭任务编辑" title="关闭" :disabled="busy" @click="emit('close')"><X :size="18" /></button></header>
    <div class="editor-form">
      <div class="form-body">
        <p v-if="error || serverError" class="error" role="alert">{{ error || serverError }}</p>
        <label>执行器<select v-model.number="form.jobGroup" required :disabled="busy"><option :value="0" disabled>选择执行器</option><option v-if="job.id && !groups.some(g => g.id === form.jobGroup)" :value="form.jobGroup">#{{ form.jobGroup }}</option><option v-for="g in groups" :key="g.id" :value="g.id">{{ g.title }} · {{ g.appname }}</option></select></label>
        <label>任务描述<input v-model="form.jobDesc" required maxlength="255" :disabled="busy" /></label>
        <div class="field-pair"><label>调度类型<select v-model="form.scheduleType" :disabled="busy"><option v-for="[key, label] in schedules" :key="key" :value="key">{{ label }}</option></select></label><label>调度过期策略<select v-model="form.misfireStrategy" :disabled="busy"><option v-for="[key, label] in misfires" :key="key" :value="key">{{ label }}</option></select></label></div>
        <label v-if="form.scheduleType !== 'NONE'">{{ form.scheduleType === 'CRON' ? 'Cron' : '间隔（秒）' }}<input :value="form.scheduleConf" @input="form.scheduleConf = ($event.target as HTMLInputElement).value" required :type="form.scheduleType === 'FIX_RATE' ? 'number' : 'text'" min="1" step="1" spellcheck="false" class="mono" :disabled="busy" /></label>
        <div class="field-pair"><label>负责人<input v-model="form.author" required :disabled="busy" /></label><label>报警邮箱<input v-model="form.alarmEmail" :disabled="busy" /></label></div>
        <div class="field-pair"><label>运行模式<input :value="form.glueType" readonly /></label><label>路由策略<select v-model="form.executorRouteStrategy" :disabled="busy"><option v-for="[key, label] in routes" :key="key" :value="key">{{ label }}</option></select></label></div>
        <label>Handler<input v-model="form.executorHandler" :required="form.glueType === 'BEAN'" class="mono" :disabled="busy" /></label>
        <label>任务参数<textarea v-model="form.executorParam" rows="4" spellcheck="false" :disabled="busy" /></label>
        <label>阻塞策略<select v-model="form.executorBlockStrategy" :disabled="busy"><option v-for="[key, label] in blocks" :key="key" :value="key">{{ label }}</option></select></label>
        <div class="field-pair"><label>超时（秒）<input v-model.number="form.executorTimeout" type="number" min="0" max="2147483647" step="1" required :disabled="busy" /></label><label>失败重试次数<input v-model.number="form.executorFailRetryCount" type="number" min="0" max="2147483647" step="1" required :disabled="busy" /></label></div>
        <label>子任务 ID<input v-model="form.childJobId" class="mono" :disabled="busy" /></label>
      </div>
      <footer><button type="button" :disabled="busy" @click="emit('close')">取消</button><button class="primary" type="button" :disabled="busy" @click="save"><Save :size="16" />保存</button></footer>
    </div>
  </section>
</template>

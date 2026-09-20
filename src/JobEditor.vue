<script setup lang="ts">
import { computed, ref } from 'vue';
import { CalendarClock, Save, X } from '@lucide/vue';
import { type Job, type Group, routes, blocks, schedules, misfires, validateJob } from './domain';
import { t } from './i18n';
import CronBuilder from './CronBuilder.vue';
import UiSelect from './UiSelect.vue';

const props = defineProps<{ job: Job; groups: Group[]; connectionId: string; busy: boolean; suspended?: boolean; serverError?: string }>();
const emit = defineEmits<{ save: [job: Job]; close: [] }>();
const original = JSON.stringify(props.job);
const form = ref<Job>({ ...props.job });
const error = ref(''), cronOpen = ref(false), cronStyle = ref<Record<string, string>>({});
const groupOptions = computed(() => [{ value: 0, label: t('选择执行器'), disabled: true }, ...(props.job.id && !props.groups.some(g => g.id === form.value.jobGroup) ? [{ value: form.value.jobGroup, label: `#${form.value.jobGroup}` }] : []), ...props.groups.map(g => ({ value: g.id ?? 0, label: `${g.title} · ${g.appname}` }))]);
const scheduleOptions = computed(() => schedules.map(([value, label]) => ({ value, label: t(label) })));
const misfireOptions = computed(() => misfires.map(([value, label]) => ({ value, label: t(label) })));
const routeOptions = computed(() => routes.map(([value, label]) => ({ value, label: t(label) })));
const blockOptions = computed(() => blocks.map(([value, label]) => ({ value, label: t(label) })));

defineExpose({ isDirty: computed(() => JSON.stringify(form.value) !== original) });
function toggleCron(event: MouseEvent) {
  if (cronOpen.value) { cronOpen.value = false; return; }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const width = Math.min(680, window.innerWidth - 24);
  const top = Math.max(42, Math.min(rect.bottom + 8, window.innerHeight - 625));
  cronStyle.value = { top: `${top}px`, left: `${Math.max(12, Math.min(rect.right - width, window.innerWidth - width - 12))}px`, width: `${width}px` };
  cronOpen.value = true;
}
function save() {
  error.value = t(validateJob(form.value));
  if (!error.value) { cronOpen.value = false; emit('save', structuredClone({ ...form.value })); }
}
</script>

<template>
  <section class="modal editor-dialog job-dialog" role="dialog" aria-modal="true" :aria-label="t(job.id ? '编辑任务' : '新增任务')">
    <header><div><h2>{{ t(job.id ? '编辑任务' : '新增任务') }} <small v-if="job.id">#{{ job.id }}</small></h2></div><button class="icon" :aria-label="t('关闭')" :disabled="busy" @click="emit('close')"><X :size="18" /></button></header>
    <div class="editor-form"><div class="form-body job-form">
      <p v-if="error || serverError" class="error" role="alert">{{ error || serverError }}</p>
      <section class="job-section"><h3>{{ t('基础配置') }}</h3><div class="job-fields">
        <label class="required">{{ t('执行器') }}<UiSelect v-model="form.jobGroup" :options="groupOptions" :aria-label="t('执行器')" :disabled="busy" /></label>
        <label class="required">{{ t('任务描述') }}<input v-model="form.jobDesc" required maxlength="255" :disabled="busy" :placeholder="t('任务描述')" /></label>
        <label class="required">{{ t('负责人') }}<input v-model="form.author" required :disabled="busy" :placeholder="t('负责人')" /></label>
        <label>{{ t('报警邮箱') }}<input v-model="form.alarmEmail" type="text" :disabled="busy" :placeholder="t('报警邮箱')" /></label>
      </div></section>
      <section class="job-section"><h3>{{ t('调度配置') }}</h3><div class="job-fields">
        <label class="required">{{ t('调度类型') }}<UiSelect v-model="form.scheduleType" :options="scheduleOptions" :aria-label="t('调度类型')" :disabled="busy" /></label>
        <div v-if="form.scheduleType !== 'NONE'" class="cron-field"><label class="required">{{ form.scheduleType === 'CRON' ? t('Cron 表达式') : t('间隔（秒）') }}<input v-model="form.scheduleConf" required :type="form.scheduleType === 'FIX_RATE' ? 'number' : 'text'" min="1" step="1" spellcheck="false" class="mono" :disabled="busy" /></label><button v-if="form.scheduleType === 'CRON'" class="cron-open" type="button" :disabled="busy" :aria-expanded="cronOpen" @click="toggleCron"><CalendarClock :size="16" /><span>{{ t('配置') }}</span></button></div>
        <label>{{ t('调度过期策略') }}<UiSelect v-model="form.misfireStrategy" :options="misfireOptions" :aria-label="t('调度过期策略')" :disabled="busy" /></label>
      </div></section>
      <section class="job-section"><h3>{{ t('任务配置') }}</h3><div class="job-fields">
        <label>{{ t('运行模式') }}<input :value="form.glueType" readonly /></label>
        <label :class="{ required: form.glueType === 'BEAN' }">Handler<input v-model="form.executorHandler" :required="form.glueType === 'BEAN'" class="mono" :disabled="busy" placeholder="JobHandler" /></label>
        <label class="wide">{{ t('任务参数') }}<textarea v-model="form.executorParam" rows="2" spellcheck="false" :disabled="busy" /></label>
      </div></section>
      <section class="job-section"><h3>{{ t('高级配置') }}</h3><div class="job-fields">
        <label>{{ t('路由策略') }}<UiSelect v-model="form.executorRouteStrategy" :options="routeOptions" :aria-label="t('路由策略')" :disabled="busy" /></label>
        <label>{{ t('子任务 ID') }}<input v-model="form.childJobId" class="mono" :disabled="busy" /></label>
        <label>{{ t('阻塞策略') }}<UiSelect v-model="form.executorBlockStrategy" :options="blockOptions" :aria-label="t('阻塞策略')" :disabled="busy" /></label>
        <label>{{ t('超时（秒）') }}<input v-model.number="form.executorTimeout" type="number" min="0" max="2147483647" step="1" required :disabled="busy" /></label>
        <label>{{ t('失败重试次数') }}<input v-model.number="form.executorFailRetryCount" type="number" min="0" max="2147483647" step="1" required :disabled="busy" /></label>
      </div></section>
    </div><footer><button type="button" :disabled="busy" @click="emit('close')">{{ t('取消') }}</button><button class="primary" type="button" :disabled="busy" @click="save"><Save :size="16" />{{ t('保存') }}</button></footer></div>
    <Teleport to="body"><CronBuilder v-if="cronOpen && !suspended && form.scheduleType === 'CRON'" :style="cronStyle" :value="form.scheduleConf" :connection-id="connectionId" @apply="value => { form.scheduleConf = value; cronOpen = false; }" @close="cronOpen = false" /></Teleport>
  </section>
</template>

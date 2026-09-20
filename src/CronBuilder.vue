<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { invoke } from './bridge';
import { t } from './i18n';
import { buildCron, cronFields, defaultCronRules, parseCron, type CronField, type CronMode } from './cron';

const props = defineProps<{ value: string; connectionId: string }>();
const emit = defineEmits<{ apply: [value: string]; close: [] }>();
const parsed = parseCron(props.value);
const rules = ref(parsed || defaultCronRules());
const raw = ref(props.value || buildCron(rules.value));
const currentField = ref<CronField>('second');
const field = computed(() => cronFields.find(item => item.key === currentField.value)!);
const rule = computed(() => rules.value[currentField.value]);
const choices = computed(() => Array.from({ length: field.value.max - field.value.min + 1 }, (_, i) => field.value.min + i));
const times = ref<string[]>([]), busy = ref(false), error = ref('');
let request = 0;
const labels: Record<CronField, string> = { second: '秒', minute: '分钟', hour: '小时', day: '日', month: '月', week: '周', year: '年' };

function changeMode(mode: CronMode) {
  rule.value.mode = mode;
  if (currentField.value === 'day' && mode !== 'any') rules.value.week.mode = 'any';
  if (currentField.value === 'week' && mode !== 'any') rules.value.day.mode = 'any';
}
watch(rules, () => { raw.value = buildCron(rules.value); }, { deep: true });
watch(raw, () => { request++; times.value = []; error.value = ''; });
async function preview(): Promise<boolean> {
  const current = ++request; busy.value = true; error.value = ''; times.value = [];
  try {
    const result = await invoke<string[]>(props.connectionId, 'xxljob/nextTriggerTime', { form: { scheduleType: 'CRON', scheduleConf: raw.value.trim() } });
    if (current !== request) return false;
    times.value = result;
    return result.length > 0;
  } catch (e) { if (current === request) error.value = e instanceof Error ? e.message : String(e); return false; }
  finally { if (current === request) busy.value = false; }
}
async function apply() { if (await preview()) emit('apply', raw.value.trim()); }
onMounted(() => { if (raw.value.trim()) void preview(); });
</script>

<template>
  <div class="cron-popover" role="dialog" :aria-label="t('Cron 配置器')">
    <div class="cron-head"><strong>{{ t('Cron 配置器') }}</strong><button class="icon" type="button" :aria-label="t('关闭')" @click="emit('close')">×</button></div>
    <div class="cron-tabs" role="tablist" :aria-label="t('Cron 字段')"><button v-for="item in cronFields" :key="item.key" type="button" role="tab" :aria-selected="currentField === item.key" :class="{ selected: currentField === item.key }" @click="currentField = item.key">{{ t(labels[item.key]) }}</button></div>
    <div class="cron-body">
      <div class="cron-rule-options">
        <label><input v-model="rule.mode" type="radio" value="any" @change="changeMode('any')" />{{ t('每个') }}{{ t(labels[currentField]) }}</label>
        <label><input v-model="rule.mode" type="radio" value="cycle" @change="changeMode('cycle')" />{{ t('周期') }} {{ t('从') }} <input v-model.number="rule.start" type="number" :min="field.min" :max="field.max" :aria-label="t('起始值')" @focus="changeMode('cycle')" /> — <input v-model.number="rule.end" type="number" :min="field.min" :max="field.max" :aria-label="t('结束值')" @focus="changeMode('cycle')" /></label>
        <label><input v-model="rule.mode" type="radio" value="step" @change="changeMode('step')" />{{ t('从') }} <input v-model.number="rule.start" type="number" :min="field.min" :max="field.max" :aria-label="t('起始值')" @focus="changeMode('step')" /> {{ t('开始，每隔') }} <input v-model.number="rule.step" type="number" min="1" :max="field.max - field.min + 1" :aria-label="t('间隔值')" @focus="changeMode('step')" /> {{ t('执行一次') }}</label>
        <label><input v-model="rule.mode" type="radio" value="specific" @change="changeMode('specific')" />{{ t('指定') }}</label>
      </div>
      <div class="cron-specific" role="group" :aria-label="t('指定')"><label v-for="number in choices" :key="number"><input v-model="rule.values" type="checkbox" :value="number" @change="changeMode('specific')" />{{ currentField === 'second' || currentField === 'minute' ? String(number).padStart(2, '0') : number }}</label></div>
      <label class="cron-expression">{{ t('Cron 表达式') }}<input v-model="raw" class="mono" spellcheck="false" /></label>
      <p v-if="error" class="cron-error" role="alert">{{ error }}</p>
      <div class="cron-preview"><strong>{{ t('最近五次运行时间') }}</strong><ol v-if="times.length"><li v-for="time in times" :key="time">{{ time }}</li></ol><p v-else class="muted">{{ t('点击预览，使用调度中心计算时间') }}</p></div>
    </div>
    <footer><button type="button" :disabled="busy" @click="preview">{{ busy ? t('计算中') : t('预览') }}</button><button class="primary" type="button" :disabled="busy" @click="apply">{{ t('应用') }}</button></footer>
  </div>
</template>

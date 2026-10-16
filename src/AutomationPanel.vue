<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { Sparkles, X, RefreshCw } from '@lucide/vue';
import { invoke } from './bridge';
import { type Settings, type Audit } from './automation';
import { t } from './i18n';
import LoadingSkeleton from './LoadingSkeleton.vue';

const props = defineProps<{ connectionId: string; readOnly: boolean; production: boolean; admin: boolean; name: string }>();
const emit = defineEmits<{ opened: [value: boolean] }>();
const open = ref(false), tab = ref('permissions'), busy = ref(false), error = ref('');
const dialog = ref<HTMLElement>();
let previousFocus: HTMLElement | null = null;
function keyboard(event: KeyboardEvent) {
  if (event.key !== 'Tab') return;
  const elements = [...(dialog.value?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), summary') || [])].filter(el => el.getClientRects().length);
  const first = elements[0], last = elements.at(-1);
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
}
const settings = ref<Settings>(), records = ref<Audit[]>([]), enabled = ref(false), allowed = ref<string[]>([]);
let generation = 0, timer: ReturnType<typeof setInterval> | undefined;
const effectiveTools = computed(() => settings.value?.tools.filter(tool => settings.value?.policy.allowedTools.includes(tool.name) && (tool.group === 'query' || (!props.readOnly && (tool.group !== 'executors' || props.admin)))) || []);
const status = computed(() => !settings.value?.policy.enabled ? '关闭' : !effectiveTools.value.length ? '无工具授权' : effectiveTools.value.some(tool => tool.group !== 'query') ? '允许修改' : '仅查询');
const groups = [{ key: 'query', label: '查询诊断' }, { key: 'schedule', label: '调度操作' }, { key: 'jobs', label: '任务配置' }, { key: 'delete', label: '删除任务' }, { key: 'executors', label: '执行器管理' }];
function accept(value: Settings) { settings.value = value; enabled.value = value.policy.enabled; allowed.value = [...value.policy.allowedTools]; }
async function reload() { const epoch = ++generation; error.value = ''; try { const value = await invoke<Settings>(props.connectionId, 'mcp/settings/get'); if (epoch === generation) accept(value); } catch (e) { if (epoch === generation) error.value = String(e); } }
async function loadAudit() { const epoch = generation; try { const value = await invoke<Audit[]>(props.connectionId, 'xxljob/audit'); if (epoch === generation) records.value = value; } catch (e) { if (epoch === generation) error.value = String(e); } }
async function save() { if (!settings.value) return; const epoch = generation; busy.value = true; error.value = ''; try { const value = await invoke<Settings>(props.connectionId, 'mcp/settings/set', { enabled: enabled.value, allowedTools: allowed.value, revision: settings.value.policy.revision }); if (epoch === generation) accept(value); } catch (e) { if (epoch === generation) error.value = String(e); } finally { if (epoch === generation) busy.value = false; } }
function disabled(group: string) { return !enabled.value || busy.value || !!settings.value?.warning || (group !== 'query' && (props.readOnly || !settings.value?.policy.enabled)) || (group === 'executors' && !props.admin); }
function tools(group: string) { return settings.value?.tools.filter(tool => tool.group === group) || []; }
function toggleGroup(group: string, value: boolean) { const names = tools(group).map(tool => tool.name); allowed.value = value ? [...new Set([...allowed.value, ...names])] : allowed.value.filter(name => !names.includes(name)); }
function toggleEnabled() { if (enabled.value && !settings.value?.policy.enabled) allowed.value = tools('query').map(tool => tool.name); }
watch(() => props.connectionId, () => { generation++; busy.value = false; open.value = false; settings.value = undefined; records.value = []; if (props.connectionId) void reload(); }, { immediate: true });
watch(open, value => { emit('opened', value); clearInterval(timer); if (value) { void reload(); void loadAudit(); timer = setInterval(() => { if (tab.value === 'audit') void loadAudit(); }, 5000); } });
onBeforeUnmount(() => { generation++; clearInterval(timer); });
watch(open, async value => { if (value) { previousFocus = document.activeElement as HTMLElement; await nextTick(); dialog.value?.querySelector<HTMLButtonElement>('button')?.focus(); } else { await nextTick(); previousFocus?.focus(); } });
</script>

<template>
  <button class="automation-entry" :disabled="!connectionId" @click="open = true"><Sparkles :size="15" />AI/MCP · {{ t(status) }}</button>
  <Teleport to="body"><div v-if="open" class="overlay modal-layer" @keydown.esc.stop="!busy && (open = false)">
    <section ref="dialog" class="modal automation-panel" role="dialog" aria-modal="true" aria-label="AI/MCP" @keydown="keyboard">
      <header><div><small>{{ name }} · {{ t(production ? '生产环境' : '当前连接') }}<span v-if="readOnly"> · {{ t('只读') }}</span></small><h2><Sparkles :size="18" /> AI/MCP</h2></div><button class="icon" :disabled="busy" :aria-label="t('关闭')" @click="open = false"><X :size="18" /></button></header>
      <div class="automation-tabs"><button :class="{ primary: tab === 'permissions' }" @click="tab = 'permissions'">{{ t('权限配置') }}</button><button :class="{ primary: tab === 'audit' }" @click="tab = 'audit'; loadAudit()">{{ t('调用记录') }}</button></div>
      <div class="modal-body">
        <p v-if="error || settings?.warning" class="banner error" role="alert">{{ error || settings?.warning }}</p>
        <LoadingSkeleton v-if="!settings && !error" />
        <template v-if="tab === 'permissions' && settings">
          <p class="muted">{{ t('使用 DBX 已配置的模型。权限仅适用于当前连接，同时约束内置 AI 和外部 MCP。') }}</p>
          <label class="inline"><input v-model="enabled" type="checkbox" :disabled="busy || !!settings.warning" @change="toggleEnabled" />{{ t('启用 AI/MCP（重新开启默认仅查询）') }}</label>
          <fieldset v-for="group in groups" :key="group.key" :disabled="disabled(group.key)" class="automation-group">
            <legend><label class="inline"><input type="checkbox" :disabled="disabled(group.key)" :checked="tools(group.key).every(tool => allowed.includes(tool.name))" @change="toggleGroup(group.key, ($event.target as HTMLInputElement).checked)" />{{ t(group.label) }}</label></legend>
            <details><summary>{{ t('查看具体工具') }} · {{ tools(group.key).length }}</summary><label v-for="tool in tools(group.key)" :key="tool.name" class="automation-tool"><input v-model="allowed" type="checkbox" :value="tool.name" /><span><strong>{{ tool.name }}</strong><small>{{ tool.description }}</small></span></label></details>
          </fieldset>
          <p class="banner">{{ t('内置 AI 请使用 Agent 模式，并在 DBX 插件中心开启工具；外部 MCP 还需授权此连接。') }}</p>
          <p class="muted">{{ t('写操作需要预览与确认。日志可能包含业务信息，请按需授权读取。') }}</p>
        </template>
        <template v-if="tab === 'audit'"><p v-if="!records.length" class="muted">{{ t('暂无调用记录') }}</p><div v-for="(record, index) in records" :key="index" class="audit-row"><div><strong>{{ record.tool }}</strong><code v-if="record.resourceId">#{{ record.resourceId }}</code><span class="badge neutral">{{ t(record.outcome) }}</span></div><small>{{ new Date(record.time).toLocaleString() }} · {{ record.durationMs }} ms</small></div><p class="muted">{{ t('最近 200 条记录，不保存参数、凭据或日志正文。') }}</p></template>
      </div>
      <footer><button :disabled="busy" @click="tab === 'audit' ? loadAudit() : reload()"><RefreshCw :size="15" />{{ t('重新读取') }}</button><div class="spacer" /><button :disabled="busy" @click="open = false">{{ t('关闭') }}</button><button v-if="tab === 'permissions'" class="primary" :disabled="busy || !settings || !!settings.warning" @click="save">{{ t(busy ? '提交中' : '保存权限') }}</button></footer>
    </section>
  </div></Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { Workflow, ListTodo, Server, ScrollText, RefreshCw, Search, Plus, Pencil, Trash2, Play, Pause, FileText, ChevronLeft, ChevronRight, X, AlertTriangle, Check, Plug, ChartNoAxesCombined, Users, ArrowLeft, ExternalLink } from '@lucide/vue';
import { invoke, type Context } from './bridge';
import { type Job, type Group, type JobOption, type LogRow, type Info, type Page, type Form, type LogChunk, newJob, newGroup, jobForm, groupForm, jobStatus, schedules, timeLabel, triggerMillis, filterTime, advanceLog } from './domain';
import JobEditor from './JobEditor.vue';
import GroupEditor from './GroupEditor.vue';
import ReportCharts, { type ReportData } from './ReportCharts.vue';
import UiSelect from './UiSelect.vue';
import { t, locale, setLocale } from './i18n';

type View = 'report' | 'jobs' | 'logs' | 'groups' | 'users';
const navigation = [{ key: 'report', label: '运行报表', icon: ChartNoAxesCombined }, { key: 'jobs', label: '任务管理', icon: ListTodo }, { key: 'logs', label: '调度日志', icon: ScrollText }, { key: 'groups', label: '执行器管理', icon: Server }, { key: 'users', label: '用户管理', icon: Users }] as const;
const visibleNavigation = computed(() => navigation.filter(item => item.key !== 'users' || info.value?.admin));
const view = ref<View>('report');
const connectionId = ref('');
const info = ref<Info>();
const jobs = ref<Job[]>([]), groups = ref<Group[]>([]), logs = ref<LogRow[]>([]), taskOptions = ref<JobOption[]>([]);
type UserRow = { id: number; username: string; role: number; permission: string };
const users = ref<UserRow[]>([]), report = ref<ReportData>();
const reportOverview = ref({ jobs: 0, triggers: 0, executors: 0 });
const reportTotals = computed(() => ({ running: report.value?.triggerDayCountRunningList.reduce((a, b) => a + b, 0) || 0, success: report.value?.triggerDayCountSucList.reduce((a, b) => a + b, 0) || 0, failure: report.value?.triggerDayCountFailList.reduce((a, b) => a + b, 0) || 0 }));
const appearance = ref(0);
const optionsBusy = ref(false), authExpired = ref(false);
const loading = ref(false), working = ref(false), error = ref(''), groupsError = ref(''), notice = ref('');
const page = ref(1), pageSize = ref(25), total = ref(0);
const dateBefore = (days: number) => { const d = new Date(); d.setDate(d.getDate() - days); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`; };
type Period = 'today' | 'yesterday' | 'month' | 'lastMonth' | 'week' | 'last30' | 'custom';
const period = ref<Period>('week');
const periods: { key: Period; label: string }[] = [{ key: 'today', label: '今日' }, { key: 'yesterday', label: '昨日' }, { key: 'month', label: '本月' }, { key: 'lastMonth', label: '上个月' }, { key: 'week', label: '最近一周' }, { key: 'last30', label: '最近一月' }, { key: 'custom', label: '自定义' }];
const localDate = (value: Date) => `${value.getFullYear()}-${String(value.getMonth()+1).padStart(2,'0')}-${String(value.getDate()).padStart(2,'0')}`;
function setPeriod(value: Period) {
  period.value = value;
  const today = new Date();
  if (value === 'custom') return;
  const start = new Date(today), end = new Date(today);
  if (value === 'yesterday') { start.setDate(start.getDate()-1); end.setDate(end.getDate()-1); }
  if (value === 'week') start.setDate(start.getDate()-6);
  if (value === 'last30') start.setDate(start.getDate()-29);
  if (value === 'month') start.setDate(1);
  if (value === 'lastMonth') { start.setMonth(start.getMonth()-1,1); end.setDate(0); }
  filters.value.reportStart = localDate(start); filters.value.reportEnd = localDate(end);
  if (info.value) void load();
}
const freshFilters = () => ({ jobGroup: -1, jobDesc: '', executorHandler: '', author: '', triggerStatus: -1, jobId: '', logStatus: -1, from: '', to: '', groupSearch: '', username: '', userRole: -1, reportStart: dateBefore(6), reportEnd: dateBefore(0) });
const filters = ref(freshFilters());
const filteredGroups = computed(() => groups.value.filter(g => `${g.appname} ${g.title} ${g.addressList}`.toLowerCase().includes(filters.value.groupSearch.toLowerCase())));
const groupFilterOptions = computed(() => [...(info.value?.admin ? [{ value: -1, label: t('全部执行器') }] : []), ...groups.value.map(g => ({ value: g.id ?? 0, label: g.title }))]);
const jobStatusOptions = computed(() => [{ value: -1, label: t('全部状态') }, { value: 1, label: t('运行中') }, { value: 0, label: t('已停止') }]);
const logStatusOptions = computed(() => [{ value: -1, label: t('全部状态') }, { value: 1, label: t('执行成功') }, { value: 2, label: t('执行失败') }, { value: 3, label: t('执行中') }]);
const jobFilterOptions = computed(() => [{ value: '', label: t(optionsBusy.value ? '任务加载中' : '全部任务') }, ...taskOptions.value.map(job => ({ value: String(job.id), label: `#${job.id} · ${job.jobDesc}` }))]);
const userRoleOptions = computed(() => [{ value: -1, label: t('全部角色') }, { value: 1, label: t('管理员') }, { value: 0, label: t('普通用户') }]);
const pageSizeOptions = computed(() => [10,25,50,100].map(size => ({ value: size, label: `${size} ${t('条每页')}` })));
const pagedGroups = computed(() => filteredGroups.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value));
const displayTotal = computed(() => view.value === 'groups' ? filteredGroups.value.length : total.value);
const label = computed(() => t(navigation.find(n => n.key === view.value)!.label));
const canWrite = computed(() => !!info.value && !info.value.readOnly && !authExpired.value && !working.value && !loading.value);
const canManageGroups = computed(() => canWrite.value && info.value?.admin);
const groupLabel = (id: number) => groups.value.find(g => g.id === id)?.title || `#${id}`;
const scheduleLabel = (type: string) => t(schedules.find(([key]) => key === type)?.[1] || type);
let epoch = 0, listRequest = 0, groupRequest = 0, logRequest = 0, optionRequest = 0;
let unsubscribe: (() => void) | undefined;
let logTimer: ReturnType<typeof setTimeout> | undefined;
const jobEditor = ref<Job>(), groupEditor = ref<Group>();
const userEditor = ref<UserRow & { password: string }>();
const jobEditorRef = ref<InstanceType<typeof JobEditor>>(), groupEditorRef = ref<InstanceType<typeof GroupEditor>>();
type Confirmation = { method: string; title: string; target: string; form: Form; changes?: { key: string; before: string; after: string }[]; remove?: boolean; id?: number };
const confirmation = ref<Confirmation>(), deletionId = ref(''), discard = ref(false), detail = ref<LogRow>();
const execution = ref<Job>(), executionParam = ref('');
type LogState = { row: LogRow; text: string; next: number; end: boolean; busy: boolean; error: string; follow: boolean };
const logState = ref<LogState>();
const standaloneLog = ref(false);
const canOpenTab = computed(() => !!window.dbxPlugin?.openWorkbench);
const message = (e: unknown) => { const value = e instanceof Error ? e.message : String(e); if (value.includes('登录已失效')) authExpired.value = true; return value; };
const displayMessage = (value: string) => locale.value === 'zh-CN' || !/[\u3400-\u9fff]/.test(value) ? value : t(value) === value ? t('服务请求失败，请检查连接和权限。') : t(value);

async function focusDialog() { await nextTick(); const dialogs = document.querySelectorAll<HTMLElement>('[role="dialog"]'); const dialog = dialogs[dialogs.length - 1]; dialog?.querySelector<HTMLElement>('input:not([disabled]):not([readonly]), select:not([disabled]), button:not([disabled]), textarea:not([disabled])')?.focus(); }
async function loadGroups() {
  const token = ++groupRequest, current = epoch;
  try { const data = await invoke<Group[]>(connectionId.value, 'xxljob/groups'); if (current === epoch && token === groupRequest) { groups.value = data; groupsError.value = ''; } }
  catch (e) { if (current === epoch && token === groupRequest) { groups.value = []; groupsError.value = message(e); } }
}
async function loadTaskOptions(): Promise<boolean> {
  const token = ++optionRequest, current = epoch, group = filters.value.jobGroup;
  taskOptions.value = []; optionsBusy.value = group > 0;
  if (group < 1) { filters.value.jobId = ''; return true; }
  try {
    const data = await invoke<JobOption[]>(connectionId.value, 'xxljob/jobsByGroup', { form: { jobGroup: group } });
    if (current !== epoch || token !== optionRequest) return false;
    taskOptions.value = data;
    if (!data.some(job => String(job.id) === filters.value.jobId)) filters.value.jobId = '';
    return true;
  } catch (e) { if (current === epoch && token === optionRequest) { filters.value.jobId = ''; logs.value = []; total.value = 0; error.value = message(e); } return false; }
  finally { if (current === epoch && token === optionRequest) optionsBusy.value = false; }
}
async function changeGroup() { page.value = 1; if (view.value === 'logs') { filters.value.jobId = ''; if (!await loadTaskOptions()) return; } await load(); }
async function showJobLogs(job: Job) { filters.value.jobGroup = job.jobGroup; filters.value.jobId = String(job.id); await changeView('logs'); }
async function load() {
  if (!info.value || authExpired.value) return;
  const token = ++listRequest, current = epoch, targetView = view.value;
  loading.value = true; error.value = '';
  try {
    if (targetView === 'groups') { await loadGroups(); return; }
    if (targetView === 'report') {
      const [data, overview] = await Promise.all([
        invoke<ReportData>(connectionId.value, 'xxljob/report', { form: { startDate: `${filters.value.reportStart} 00:00:00`, endDate: `${filters.value.reportEnd} 23:59:59` } }),
        invoke<{ jobs: number; triggers: number; executors: number }>(connectionId.value, 'xxljob/overview'),
      ]);
      if (current === epoch && token === listRequest) { report.value = data; reportOverview.value = overview; }
      return;
    }
    const form: Form = { start: (page.value - 1) * pageSize.value, length: pageSize.value };
    if (targetView === 'jobs') Object.assign(form, { jobGroup: filters.value.jobGroup, triggerStatus: filters.value.triggerStatus, jobDesc: filters.value.jobDesc, executorHandler: filters.value.executorHandler, author: filters.value.author });
    if (targetView === 'logs') {
      if (filters.value.jobId && (!/^\d+$/.test(filters.value.jobId) || Number(filters.value.jobId) < 1)) throw new Error('任务 ID 必须是正整数');
      Object.assign(form, { jobGroup: filters.value.jobGroup > 0 ? filters.value.jobGroup : 0, jobId: filters.value.jobId || 0, logStatus: filters.value.logStatus, filterTime: filterTime(filters.value.from, filters.value.to) });
    }
    if (targetView === 'users') Object.assign(form, { username: filters.value.username, role: filters.value.userRole });
    const data = await invoke<Page<Job | LogRow>>(connectionId.value, `xxljob/${targetView}`, { form });
    if (current !== epoch || token !== listRequest) return;
    total.value = data.recordsFiltered;
    if (targetView === 'jobs') jobs.value = data.data as Job[];
    if (targetView === 'logs') logs.value = data.data as LogRow[];
    if (targetView === 'users') users.value = data.data as unknown as UserRow[];
    if (page.value > 1 && !data.data.length && total.value > 0) { page.value = Math.max(1, Math.ceil(total.value / pageSize.value)); await load(); }
  } catch (e) {
    if (current === epoch && token === listRequest) { error.value = message(e); jobs.value = []; logs.value = []; total.value = 0; }
  } finally { if (current === epoch && token === listRequest) loading.value = false; }
}
function clearOverlays() { jobEditor.value = undefined; groupEditor.value = undefined; userEditor.value = undefined; confirmation.value = undefined; execution.value = undefined; discard.value = false; detail.value = undefined; closeLog(); }
async function contextChanged(context: Context) {
  const current = ++epoch; ++listRequest; ++groupRequest; ++optionRequest;
  connectionId.value = context.connectionId || ''; info.value = undefined; groups.value = []; jobs.value = []; logs.value = []; users.value = []; report.value = undefined; reportOverview.value = { jobs: 0, triggers: 0, executors: 0 }; taskOptions.value = []; optionsBusy.value = false; authExpired.value = false; filters.value = freshFilters(); period.value = 'week'; view.value = 'report'; notice.value = ''; error.value = ''; groupsError.value = ''; loading.value = false; working.value = false; total.value = 0; page.value = 1; clearOverlays(); standaloneLog.value = context.page === 'log';
  if (!connectionId.value) return;
  loading.value = true;
  try { const data = await invoke<Info>(connectionId.value, 'xxljob/info'); if (current !== epoch) return; info.value = data; await loadGroups(); if (current !== epoch) return; if (!data.admin) filters.value.jobGroup = groups.value[0]?.id || 0; if (standaloneLog.value && context.log && typeof context.log.id === 'number') { openLog(context.log as LogRow); return; } await load(); }
  catch (e) { if (current === epoch) error.value = message(e); }
  finally { if (current === epoch) loading.value = false; }
}
async function changeView(value: View) { if (working.value || jobEditor.value || groupEditor.value || userEditor.value || (value === 'users' && !info.value?.admin)) return; clearOverlays(); view.value = value; page.value = 1; total.value = 0; if (value === 'logs' && !await loadTaskOptions()) return; await load(); }
function search() { if (loading.value || working.value) return; page.value = 1; if (view.value !== 'groups') void load(); }
function searchOnEnter(event: KeyboardEvent) { if ((event.target as HTMLElement).tagName === 'INPUT') { event.preventDefault(); search(); } }
async function refresh() {
  const current = epoch;
  try { const data = await invoke<Info>(connectionId.value, 'xxljob/info'); if (current !== epoch) return; info.value = data; await loadGroups(); if (current !== epoch) return; if (!data.admin && !groups.value.some(g => g.id === filters.value.jobGroup)) filters.value.jobGroup = groups.value[0]?.id || 0; if (!data.admin && view.value === 'users') view.value = 'jobs'; if (view.value === 'logs') await loadTaskOptions(); if (logState.value) void readLog(); else await load(); }
  catch (e) { if (current === epoch) error.value = message(e); }
}
function movePage(delta: number) { page.value += delta; if (view.value !== 'groups') void load(); }
function editJob(job?: Job) { if (!canWrite.value) return; jobEditor.value = job ? { ...newJob(job.jobGroup), ...job } : newJob(groups.value[0]?.id || 0); void focusDialog(); }
function editGroup(group?: Group) { if (!canManageGroups.value) return; groupEditor.value = group ? { ...group } : newGroup(); void focusDialog(); }
function editUser(user?: UserRow) { if (!canWrite.value || !info.value?.admin) return; error.value = ''; userEditor.value = { id: user?.id || 0, username: user?.username || '', role: user?.role ?? 0, permission: user?.permission || '', password: '' }; void focusDialog(); }
function saveUser() { const u = userEditor.value; if (!u) return; if (u.username.length < 4 || u.username.length > 20 || (!u.id && u.password.length < 4) || (u.password && u.password.length > 20)) { error.value = t('用户名和新密码长度应为 4–20 位'); return; } confirmation.value = { method: 'xxljob/saveUser', title: u.id ? '确认编辑用户' : '确认新增用户', target: u.username, form: { id: u.id || '', username: u.username, password: u.password, role: u.role, permission: u.permission } }; void focusDialog(); }
function removeUser(user: UserRow) { if (!canWrite.value || !info.value?.admin || user.username === info.value.username) return; deletionId.value = ''; confirmation.value = { method: 'xxljob/removeUser', title: '确认删除用户', target: user.username, form: { id: user.id }, id: user.id, remove: true }; void focusDialog(); }
function closeEditor() { if (working.value) return; if (jobEditorRef.value?.isDirty || groupEditorRef.value?.isDirty) { discard.value = true; void focusDialog(); } else { jobEditor.value = undefined; groupEditor.value = undefined; } }
function diff(before: Form, after: Form) { return Object.entries(after).filter(([key, value]) => before[key] !== value).map(([key, value]) => ({ key, before: String(before[key] ?? '-'), after: String(value === '' ? '（空）' : value) })); }
function prepareSaveJob(job: Job) { const form = jobForm(job); confirmation.value = { method: 'xxljob/saveJob', title: job.id ? '确认更新任务' : '确认新增任务', target: `${job.jobDesc} · ${groupLabel(job.jobGroup)}`, form, changes: diff(job.id && jobEditor.value ? jobForm(jobEditor.value) : {}, form) }; void focusDialog(); }
function prepareSaveGroup(group: Group) { const form = groupForm(group); confirmation.value = { method: 'xxljob/saveGroup', title: group.id ? '确认更新执行器' : '确认新增执行器', target: `${group.title} · ${group.appname}`, form, changes: diff(group.id && groupEditor.value ? groupForm(groupEditor.value) : {}, form) }; void focusDialog(); }
function prepareAction(method: 'start' | 'stop' | 'removeJob' | 'removeGroup', row: Job | Group) {
  if (!canWrite.value || !row.id) return;
  const titles = { start: '确认启动任务', stop: '确认停止调度', removeJob: '确认删除任务', removeGroup: '确认删除执行器' };
  deletionId.value = ''; confirmation.value = { method: `xxljob/${method}`, title: titles[method], target: `${'jobDesc' in row ? row.jobDesc : row.title} · #${row.id}`, form: { id: row.id }, id: row.id, remove: method.startsWith('remove') }; void focusDialog();
}
function execute(job: Job) { if (!canWrite.value) return; execution.value = job; executionParam.value = job.executorParam || ''; void focusDialog(); }
function prepareExecution() { if (!execution.value?.id) return; confirmation.value = { method: 'xxljob/trigger', title: '确认执行一次', target: `${execution.value.jobDesc} · #${execution.value.id}`, form: { id: execution.value.id, executorParam: executionParam.value }, changes: [{ key: 'executorParam', before: execution.value.executorParam || '（空）', after: executionParam.value || '（空）' }] }; execution.value = undefined; void focusDialog(); }
async function commit() {
  if (!confirmation.value || !canWrite.value || (confirmation.value.remove && deletionId.value !== String(confirmation.value.id))) return;
  const pending = confirmation.value, current = epoch;
  working.value = true; error.value = ''; notice.value = '';
  try {
    await invoke(connectionId.value, pending.method, { form: { ...pending.form }, confirmed: true });
    if (current !== epoch) return;
    notice.value = pending.method === 'xxljob/trigger' ? '执行请求已提交，请在调度日志中确认执行结果。' : '操作成功。';
    confirmation.value = undefined; jobEditor.value = undefined; groupEditor.value = undefined; userEditor.value = undefined;
    await Promise.all([loadGroups(), load()]);
  } catch (e) { if (current === epoch) { error.value = message(e); confirmation.value = undefined; } }
  finally { if (current === epoch) working.value = false; }
}
function closeLog() { ++logRequest; if (logTimer) clearTimeout(logTimer); logTimer = undefined; logState.value = undefined; }
function openLog(row: LogRow) { closeLog(); logState.value = { row, text: '', next: 1, end: false, busy: false, error: '', follow: true }; void readLog(); }
function backFromLog() { closeLog(); standaloneLog.value = false; if (view.value === 'report') void changeView('logs'); }
async function newLogTab() { if (!logState.value || !window.dbxPlugin?.openWorkbench) return; try { await window.dbxPlugin.openWorkbench('io.dbx.xxljob-console.log', { connectionId: connectionId.value, page: 'log', log: { ...logState.value.row } }, { forceNew: true }); } catch (e) { error.value = message(e); } }
function scheduleLog() { if (logTimer) clearTimeout(logTimer); logTimer = undefined; if (logState.value?.follow && !logState.value.end && !logState.value.error) logTimer = setTimeout(() => void readLog(), 3000); }
async function readLog() {
  const state = logState.value; if (!state || state.busy || state.end) return;
  if (logTimer) clearTimeout(logTimer); logTimer = undefined;
  const token = ++logRequest, current = epoch; state.busy = true; state.error = '';
  try {
    const form = info.value?.adminVersion === '3.4'
      ? { logId: state.row.id, fromLineNum: state.next }
      : { executorAddress: state.row.executorAddress, triggerTime: triggerMillis(state.row.triggerTime), logId: state.row.id, fromLineNum: state.next };
    const chunk = await invoke<LogChunk>(connectionId.value, 'xxljob/logContent', { form });
    if (current !== epoch || token !== logRequest || logState.value !== state) return;
    const next = advanceLog(state.next, chunk);
    if (state.text.length + next.text.length > 8 * 1024 * 1024) throw new Error('日志显示达到 8 MiB 上限，请在执行器侧查看后续内容');
    state.text += next.text; state.next = next.next; state.end = next.end;
  } catch (e) { if (current === epoch && token === logRequest && logState.value === state) { state.error = message(e); state.follow = false; } }
  finally { if (current === epoch && token === logRequest && logState.value === state) { state.busy = false; scheduleLog(); } }
}
function keyboard(event: KeyboardEvent) {
  const dialogs = document.querySelectorAll<HTMLElement>('[role="dialog"]'); const top = dialogs[dialogs.length - 1]; if (!top) return;
  if (event.key === 'Escape' && !working.value) { event.preventDefault(); if (confirmation.value) confirmation.value = undefined; else if (discard.value) discard.value = false; else if (execution.value) execution.value = undefined; else if (detail.value) detail.value = undefined; else if (userEditor.value) userEditor.value = undefined; else closeEditor(); }
  if (event.key === 'Tab') { const elements = [...top.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex="0"]')]; const first = elements[0], last = elements[elements.length - 1]; if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); } }
}
onMounted(async () => {
  document.addEventListener('keydown', keyboard);
  if (!window.dbxPlugin) return;
  document.addEventListener('dbx-plugin-env', onEnvironment);
  try { await window.dbxPlugin.ready; setLocale(window.dbxPlugin.locale); unsubscribe = window.dbxPlugin.onContext(context => void contextChanged(context)); await contextChanged(window.dbxPlugin.context); }
  catch (e) { error.value = message(e); }
});
function onEnvironment(event: Event) { const detail = (event as CustomEvent<{ locale?: string }>).detail; if (detail?.locale) setLocale(detail.locale); appearance.value++; }
onBeforeUnmount(() => { ++epoch; unsubscribe?.(); closeLog(); document.removeEventListener('keydown', keyboard); document.removeEventListener('dbx-plugin-env', onEnvironment); });
</script>

<template>
  <div class="shell">
    <header class="workbench-nav"><nav :aria-label="t('主导航')"><button v-for="item in visibleNavigation" :key="item.key" :class="{ active: view === item.key && !logState }" :disabled="working || !!jobEditor || !!groupEditor || !!userEditor || optionsBusy" :title="t(item.label)" @click="changeView(item.key)"><component :is="item.icon" :size="15" /><span>{{ t(item.label) }}</span></button></nav><div v-if="info" class="account-meta"><span><strong>{{ info.username }}</strong> · {{ info.admin ? t('管理员') : t('普通用户') }}<span v-if="info.readOnly"> · {{ t('只读') }}</span></span><code :title="info.baseUrl">{{ info.baseUrl }}</code></div></header>
    <main :inert="!!jobEditor || !!groupEditor || !!userEditor || !!confirmation || !!discard || !!execution || !!detail">
      <section v-if="logState" class="log-page"><header><button class="back-button" @click="backFromLog"><ArrowLeft :size="17" />{{ t('返回') }}</button><div><h1>{{ t('执行日志') }} #{{ logState.row.id }}</h1><span class="muted">{{ t('任务') }} #{{ logState.row.jobId }} · {{ timeLabel(logState.row.triggerTime) }}</span></div><div class="spacer" /><button :disabled="!canOpenTab" @click="newLogTab"><ExternalLink :size="15" />{{ t('新窗口查看') }}</button></header><div class="log-meta"><code>{{ logState.row.executorAddress }}</code><span class="badge" :class="logState.end ? 'success' : 'warning'">{{ t(logState.end ? '已结束' : '运行中 · 每 3 秒更新') }}</span></div><p v-if="error || logState.error" class="banner error" role="alert">{{ displayMessage(error || logState.error) }}</p><pre class="log-content" tabindex="0">{{ logState.text || t(logState.busy ? '读取中' : '暂无日志内容') }}</pre><footer><span class="muted">{{ t('下一行') }} {{ logState.next }}</span><div class="spacer" /><label class="inline"><input v-model="logState.follow" type="checkbox" :disabled="logState.end" @change="scheduleLog" />{{ t('自动跟随') }}</label><button :disabled="logState.busy || logState.end" @click="readLog"><RefreshCw :size="15" />{{ t('读取后续') }}</button></footer></section>
      <template v-else>
      <div v-if="error" class="banner error" role="alert"><AlertTriangle :size="17" /><span>{{ displayMessage(error) }}</span></div>
      <div v-if="notice" class="banner success" role="status"><Check :size="17" /><span>{{ t(notice) }}</span><button class="icon" :title="t('关闭提示')" :aria-label="t('关闭提示')" @click="notice = ''"><X :size="16" /></button></div>
      <div v-if="groupsError" class="banner warning" role="alert"><AlertTriangle :size="17" /><span>{{ t('执行器列表') }}：{{ displayMessage(groupsError) }}</span><button :disabled="loading || working" @click="loadGroups">{{ t('重试') }}</button></div>
      <section v-if="!info" class="empty-state"><Plug :size="38" /><h2>{{ t(loading ? '正在连接' : '未连接调度中心') }}</h2><p v-if="!loading">{{ t('请从 DBX 的 XXL-JOB 连接打开工作台。') }}</p><button v-if="connectionId && !loading" @click="contextChanged({ connectionId })"><RefreshCw :size="16" />{{ t('重试') }}</button></section>
      <template v-else>
        <div class="filters" @keydown.enter="searchOnEnter">
          <template v-if="view === 'report'"><div class="period-switch" role="radiogroup" :aria-label="t('报表区间')"><label v-for="choice in periods" :key="choice.key" :class="{ active: period === choice.key }"><input v-model="period" type="radio" :value="choice.key" @change="setPeriod(choice.key)" />{{ t(choice.label) }}</label></div><div v-if="period === 'custom'" class="custom-range"><label>{{ t('开始日期') }}<input v-model="filters.reportStart" type="date" /></label><span>—</span><label>{{ t('结束日期') }}<input v-model="filters.reportEnd" type="date" /></label></div></template>
          <template v-if="view === 'jobs' || view === 'logs'"><UiSelect v-model="filters.jobGroup" :options="groupFilterOptions" :aria-label="t('筛选执行器')" :disabled="loading || working || optionsBusy || authExpired" @change="changeGroup" /></template>
          <template v-if="view === 'jobs'"><UiSelect v-model="filters.triggerStatus" :options="jobStatusOptions" :aria-label="t('任务状态筛选')" :disabled="loading || working || authExpired" @change="search" /><input v-model="filters.jobDesc" :aria-label="t('任务描述筛选')" :placeholder="t('任务描述')" /><input v-model="filters.executorHandler" aria-label="Handler" placeholder="Handler" /><input v-model="filters.author" :aria-label="t('负责人筛选')" :placeholder="t('负责人')" /></template>
          <template v-if="view === 'groups'"><input v-model="filters.groupSearch" :aria-label="t('查询执行器')" :placeholder="t('名称、AppName 或地址')" /></template>
          <template v-if="view === 'logs'"><UiSelect v-model="filters.jobId" :options="jobFilterOptions" :aria-label="t('筛选具体任务')" :disabled="loading || working || optionsBusy || filters.jobGroup < 1 || authExpired" @change="search" /><UiSelect v-model="filters.logStatus" :options="logStatusOptions" :aria-label="t('日志状态')" :disabled="loading || working || optionsBusy || authExpired" @change="search" /></template>
          <template v-if="view === 'logs'"><input v-model="filters.from" type="datetime-local" :aria-label="t('开始时间')" /><input v-model="filters.to" type="datetime-local" :aria-label="t('结束时间')" /></template>
          <template v-if="view === 'users'"><input v-model="filters.username" :aria-label="t('用户名筛选')" :placeholder="t('用户名')" /><UiSelect v-model="filters.userRole" :options="userRoleOptions" :aria-label="t('角色筛选')" /></template>
          <button v-if="view !== 'report' || period === 'custom'" type="button" :disabled="loading || working || optionsBusy || authExpired" @click="search"><Search :size="16" />{{ t('查询') }}</button><div class="spacer" /><button v-if="view === 'jobs'" type="button" class="primary" :disabled="!canWrite || !groups.length || !!groupsError" @click="editJob()"><Plus :size="16" />{{ t('新增任务') }}</button><button v-if="view === 'groups'" type="button" class="primary" :disabled="!canManageGroups" @click="editGroup()"><Plus :size="16" />{{ t('新增执行器') }}</button><button v-if="view === 'users'" type="button" class="primary" :disabled="!canWrite" @click="editUser()"><Plus :size="16" />{{ t('新增用户') }}</button>
        </div>
        <section v-if="view === 'report'" class="report"><div class="report-cards"><article><span>{{ t('任务数量') }}</span><strong>{{ reportOverview.jobs.toLocaleString(locale) }}</strong><small>{{ t('已配置任务') }}</small><div class="card-icon"><ListTodo :size="21" /></div></article><article><span>{{ t('调度次数') }}</span><strong>{{ reportOverview.triggers.toLocaleString(locale) }}</strong><small>{{ t('累计调度') }}</small><div class="card-icon"><ChartNoAxesCombined :size="21" /></div></article><article><span>{{ t('执行器数量') }}</span><strong>{{ reportOverview.executors.toLocaleString(locale) }}</strong><small>{{ t('执行器分组') }}</small><div class="card-icon"><Server :size="21" /></div></article></div><div class="report-highlights"><span>{{ t('所选时段') }} · {{ filters.reportStart }} — {{ filters.reportEnd }}</span><span>{{ t('成功') }} <strong>{{ reportTotals.success }}</strong></span><span>{{ t('失败') }} <strong>{{ reportTotals.failure }}</strong></span><span>{{ t('运行中') }} <strong>{{ reportTotals.running }}</strong></span></div><ReportCharts :report="report" :appearance="appearance" /><p v-if="!loading && !report?.triggerDayList?.length" class="report-empty muted">{{ t('所选时间段暂无报表数据') }}</p></section>
        <div v-if="view !== 'report'" class="table-wrap" :aria-busy="loading">
          <table v-if="view === 'jobs'">
            <thead><tr><th>ID</th><th>{{ t('任务描述') }}</th><th>{{ t('执行器') }}</th><th>Handler</th><th>{{ t('调度类型') }}</th><th>{{ t('调度配置') }}</th><th>{{ t('负责人') }}</th><th>{{ t('状态') }}</th><th class="actions">{{ t('操作') }}</th></tr></thead>
            <tbody><tr v-for="job in jobs" :key="job.id">
              <td class="mono">#{{ job.id }}</td><td><strong>{{ job.jobDesc }}</strong></td><td>{{ groupLabel(job.jobGroup) }}</td><td><code>{{ job.executorHandler || job.glueType }}</code></td><td>{{ scheduleLabel(job.scheduleType) }}</td><td><code>{{ job.scheduleType === 'NONE' ? '-' : job.scheduleConf }}{{ job.scheduleType === 'FIX_RATE' ? ` ${t('秒')}` : '' }}</code></td>
              <td>{{ job.author }}</td><td><span class="badge" :class="jobStatus(job.triggerStatus).tone">{{ t(jobStatus(job.triggerStatus).label) }}</span></td>
              <td class="actions">
                <button class="icon" :aria-label="t('编辑任务')" :title="`${t('编辑任务')} #${job.id}`" :disabled="!canWrite" @click="editJob(job)"><Pencil :size="16" /></button>
                <button class="icon" :aria-label="t('执行一次')" :title="t('执行一次')" :disabled="!canWrite" @click="execute(job)"><Play :size="16" /></button>
                <button class="icon" :aria-label="t(job.triggerStatus === 1 ? '停止调度' : '启动任务')" :title="t(job.triggerStatus === 1 ? '停止调度' : '启动任务')" :disabled="!canWrite" @click="prepareAction(job.triggerStatus === 1 ? 'stop' : 'start', job)"><Pause v-if="job.triggerStatus === 1" :size="16" /><Workflow v-else :size="16" /></button>
                <button class="icon" :aria-label="t('查看任务日志')" :title="t('查看任务日志')" :disabled="loading || working || authExpired" @click="showJobLogs(job)"><FileText :size="16" /></button>
                <button class="icon danger-text" :aria-label="t('删除任务')" :title="t('删除任务')" :disabled="!canWrite" @click="prepareAction('removeJob', job)"><Trash2 :size="16" /></button>
              </td>
            </tr></tbody>
          </table>
          <table v-if="view === 'groups'"><thead><tr><th>ID</th><th>{{ t('执行器') }}</th><th>AppName</th><th>{{ t('注册方式') }}</th><th>{{ t('执行器地址') }}</th><th class="actions">{{ t('操作') }}</th></tr></thead><tbody><tr v-for="g in pagedGroups" :key="g.id"><td class="mono">#{{ g.id }}</td><td><strong>{{ g.title }}</strong></td><td><code>{{ g.appname }}</code></td><td><span class="badge neutral">{{ t(g.addressType === 0 ? '自动注册' : '手动录入') }}</span></td><td class="address-cell"><code v-for="address in (g.addressList || '').split(',').filter(Boolean)" :key="address">{{ address }}</code><span v-if="!g.addressList" class="muted">{{ t('无注册地址') }}</span></td><td class="actions"><button class="icon" :aria-label="t('编辑执行器')" :title="t('编辑执行器')" :disabled="!canManageGroups" @click="editGroup(g)"><Pencil :size="16" /></button><button class="icon danger-text" :aria-label="t('删除执行器')" :title="t('删除执行器')" :disabled="!canManageGroups" @click="prepareAction('removeGroup', g)"><Trash2 :size="16" /></button></td></tr></tbody></table>
          <table v-if="view === 'users'"><thead><tr><th>ID</th><th>{{ t('用户名') }}</th><th>{{ t('角色') }}</th><th>{{ t('执行器权限') }}</th><th class="actions">{{ t('操作') }}</th></tr></thead><tbody><tr v-for="user in users" :key="user.id"><td class="mono">#{{ user.id }}</td><td><strong>{{ user.username }}</strong></td><td><span class="badge neutral">{{ t(user.role === 1 ? '管理员' : '普通用户') }}</span></td><td>{{ user.role === 1 ? t('全部') : user.permission || t('无') }}</td><td class="actions"><button class="icon" :aria-label="t('编辑用户')" :disabled="!canWrite || user.username === info.username" @click="editUser(user)"><Pencil :size="16" /></button><button class="icon danger-text" :aria-label="t('删除用户')" :disabled="!canWrite || user.username === info.username" @click="removeUser(user)"><Trash2 :size="16" /></button></td></tr></tbody></table>
          <table v-if="view === 'logs'"><thead><tr><th>{{ t('日志 ID') }}</th><th>{{ t('任务 ID') }}</th><th>{{ t('触发时间') }}</th><th>{{ t('执行器地址') }}</th><th>Handler</th><th>{{ t('调度结果') }}</th><th>{{ t('执行结果') }}</th><th class="actions">{{ t('操作') }}</th></tr></thead><tbody><tr v-for="row in logs" :key="row.id"><td class="mono">#{{ row.id }}</td><td class="mono">#{{ row.jobId }}</td><td>{{ timeLabel(row.triggerTime) }}</td><td class="address-cell"><code>{{ row.executorAddress || '-' }}</code></td><td>{{ row.executorHandler }}</td><td><span class="badge" :class="row.triggerCode === 200 ? 'success' : row.triggerCode ? 'danger' : 'neutral'">{{ t(row.triggerCode === 200 ? '成功' : row.triggerCode ? '失败' : '待调度') }}</span></td><td><span class="badge" :class="row.handleCode === 200 ? 'success' : row.handleCode ? 'danger' : 'warning'">{{ t(row.handleCode === 200 ? '成功' : row.handleCode ? '失败' : '未完成') }}</span></td><td class="actions"><button class="icon" :aria-label="t('查看日志详情')" :title="t('查看详情')" @click="detail = row; focusDialog()"><Search :size="16" /></button><button class="icon" :aria-label="t('读取执行日志')" :title="t('读取执行日志')" :disabled="!row.executorAddress || row.triggerCode !== 200" @click="openLog(row)"><FileText :size="16" /></button></td></tr></tbody></table>
          <div v-if="loading" class="table-state"><RefreshCw class="spinning" :size="20" /><span>{{ t('加载中') }}</span></div>
          <div v-else-if="!error && !displayTotal" class="table-state"><component :is="view === 'groups' ? Server : ListTodo" :size="28" /><span>{{ t('没有匹配的记录') }}</span></div>
        </div>
        <footer v-if="view !== 'report'" class="pagination"><span>{{ t('共') }} {{ displayTotal }} {{ t('条') }}</span><div class="spacer" /><UiSelect v-model="pageSize" :options="pageSizeOptions" :aria-label="t('每页条数')" :disabled="loading || working" @change="search" /><button class="icon" :aria-label="t('上一页')" :title="t('上一页')" :disabled="page <= 1 || loading || working" @click="movePage(-1)"><ChevronLeft :size="17" /></button><span>{{ page }} / {{ Math.max(1, Math.ceil(displayTotal / pageSize)) }}</span><button class="icon" :aria-label="t('下一页')" :title="t('下一页')" :disabled="page * pageSize >= displayTotal || loading || working" @click="movePage(1)"><ChevronRight :size="17" /></button></footer>
      </template>
      </template>
    </main>
    <div v-if="jobEditor || groupEditor" class="overlay" :inert="!!confirmation || discard"><JobEditor v-if="jobEditor" ref="jobEditorRef" :job="jobEditor" :groups="groups" :connection-id="connectionId" :busy="working" :server-error="displayMessage(error)" @save="prepareSaveJob" @close="closeEditor" /><GroupEditor v-if="groupEditor" ref="groupEditorRef" :group="groupEditor" :busy="working" :server-error="displayMessage(error)" @save="prepareSaveGroup" @close="closeEditor" /></div>
    <div v-if="confirmation" class="overlay modal-layer"><section class="modal" role="dialog" aria-modal="true" :aria-label="t('确认操作')"><header><div><small>{{ info?.environment || t('未标记环境') }} · {{ info?.name }}</small><h2>{{ t(confirmation.title) }}</h2></div><button class="icon" :aria-label="t('取消确认')" :title="t('取消')" :disabled="working" @click="confirmation = undefined"><X :size="18" /></button></header><div class="modal-body"><p class="target">{{ confirmation.target }}</p><code class="endpoint">{{ info?.baseUrl }}</code><p v-if="confirmation.remove" class="banner warning"><AlertTriangle :size="17" />{{ t('删除后无法撤销。') }}</p><p v-if="confirmation.method === 'xxljob/trigger'" class="banner warning"><AlertTriangle :size="17" />{{ t('本次参数将覆盖保存的参数，空白表示传入空字符串。') }}</p><p v-if="confirmation.method === 'xxljob/stop'" class="banner warning"><AlertTriangle :size="17" />{{ t('只停止后续调度，不终止已经运行的任务。') }}</p><div v-if="confirmation.changes?.length" class="changes"><div v-for="change in confirmation.changes" :key="change.key"><code>{{ change.key }}</code><pre class="before">{{ change.before }}</pre><pre>{{ change.after }}</pre></div></div><label v-if="confirmation.remove">{{ t('输入 ID') }} {{ confirmation.id }}<input v-model="deletionId" autocomplete="off" inputmode="numeric" :aria-label="t('删除确认 ID')" /></label></div><footer><button :disabled="working" @click="confirmation = undefined">{{ t('取消') }}</button><button :class="confirmation.remove ? 'danger' : 'primary'" :disabled="working || (confirmation.remove && deletionId !== String(confirmation.id))" @click="commit"><Check :size="16" />{{ t(working ? '提交中' : '确认') }}</button></footer></section></div>
    <div v-if="discard" class="overlay modal-layer"><section class="modal compact" role="dialog" aria-modal="true" :aria-label="t('未保存的修改')"><header><h2>{{ t('未保存的修改') }}</h2></header><div class="modal-body">{{ t('关闭将丢弃当前修改。') }}</div><footer><button @click="discard = false">{{ t('继续编辑') }}</button><button class="danger" @click="jobEditor = undefined; groupEditor = undefined; discard = false">{{ t('放弃修改') }}</button></footer></section></div>
    <div v-if="execution" class="overlay"><section class="modal" role="dialog" aria-modal="true" :aria-label="t('执行一次')"><header><div><small>{{ t('任务') }} #{{ execution.id }}</small><h2>{{ t('执行一次') }}</h2></div><button class="icon" :aria-label="t('取消执行')" :title="t('取消')" @click="execution = undefined"><X :size="18" /></button></header><div class="modal-form"><div class="modal-body"><p class="target">{{ execution.jobDesc }}</p><label>{{ t('本次执行参数') }}<textarea v-model="executionParam" rows="6" :aria-label="t('本次执行参数')" spellcheck="false" /></label></div><footer><button type="button" @click="execution = undefined">{{ t('取消') }}</button><button type="button" class="primary" @click="prepareExecution"><Play :size="16" />{{ t('执行一次') }}</button></footer></div></section></div>
    <div v-if="userEditor" class="overlay"><section class="modal editor-dialog" role="dialog" aria-modal="true" :aria-label="t('用户表单')"><header><h2>{{ t(userEditor.id ? '编辑用户' : '新增用户') }}</h2><button class="icon" :aria-label="t('关闭')" @click="userEditor = undefined"><X :size="18" /></button></header><div class="modal-body user-form"><p v-if="error" class="banner error" role="alert">{{ error }}</p><label>{{ t('用户名') }}<input v-model="userEditor.username" maxlength="20" :readonly="!!userEditor.id" autocomplete="off" /></label><label>{{ t('密码') }}<input v-model="userEditor.password" type="password" maxlength="20" autocomplete="new-password" :placeholder="t(userEditor.id ? '留空则不修改' : '4–20 位')" /></label><label>{{ t('角色') }}<UiSelect v-model="userEditor.role" :options="[{ value: 0, label: t('普通用户') }, { value: 1, label: t('管理员') }]" :aria-label="t('角色')" /></label><fieldset v-if="userEditor.role === 0"><legend>{{ t('执行器权限') }}</legend><label v-for="g in groups" :key="g.id" class="inline"><input type="checkbox" :checked="userEditor.permission.split(',').includes(String(g.id))" @change="userEditor.permission = ($event.target as HTMLInputElement).checked ? [...userEditor.permission.split(',').filter(Boolean), String(g.id)].join(',') : userEditor.permission.split(',').filter(id => id !== String(g.id)).join(',')" />{{ g.title }}</label></fieldset></div><footer><button @click="userEditor = undefined">{{ t('取消') }}</button><button class="primary" @click="saveUser">{{ t('保存') }}</button></footer></section></div>
    <div v-if="detail" class="overlay"><section class="modal editor-dialog" role="dialog" aria-modal="true" :aria-label="t('记录详情')"><header><h2>{{ t('记录') }} #{{ detail.id }}</h2><button class="icon" :aria-label="t('关闭记录详情')" :title="t('关闭')" @click="detail = undefined"><X :size="18" /></button></header><div class="detail-body"><dl><template v-for="(value, key) in detail" :key="key"><dt>{{ key }}</dt><dd><pre>{{ value ?? '-' }}</pre></dd></template></dl></div></section></div>
  </div>
</template>

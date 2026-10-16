import { invoke, type AiContext } from './bridge';
import { t } from './i18n';

export type Tool = { name: string; description: string; group: 'query' | 'schedule' | 'jobs' | 'delete' | 'executors' };
export type Policy = { enabled: boolean; allowedTools: string[]; revision: number };
export type Settings = { connectionId: string; policy: Policy; tools: Tool[]; warning?: string };
export type Audit = { time: string; tool: string; resourceId?: string; durationMs: number; outcome: string };
export type Preview = { previewToken: string; expiresAt: string; target: string; isProduction: boolean; executed: false; changes: { key: string; before: unknown; after: unknown }[]; warnings: string[] };
export function previewOperation(connectionId: string, operation: string, args: Record<string, unknown>) {
  return invoke<Preview>(connectionId, 'xxljob/preview', { form: { operation, arguments: args } });
}
export function recommendations(context: AiContext) {
  const label = t(context.resource?.kind === 'log' ? '分析这次任务失败的原因' : context.resource?.kind === 'executor' ? '检查执行器注册节点' : context.resource?.kind === 'job' ? '分析任务调度和近期执行情况' : '检查近期失败任务');
  return { context, items: [{ id: 'xxljob-inspect', label, prompt: `${label}。${t('先使用查询工具，根据实际结果说明证据。')}\n${JSON.stringify(context)}`, order: 10 }] };
}
export function logLookup(row: { id: number; jobId: number; jobGroup: number; triggerTime: string | number }) {
  const value = typeof row.triggerTime === 'number' ? row.triggerTime : /^\d+$/.test(row.triggerTime) ? Number(row.triggerTime) : Date.parse(row.triggerTime);
  if (!Number.isFinite(value)) throw new Error('日志触发时间无效');
  const format = (d: Date) => `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}:${String(d.getSeconds()).padStart(2,'0')}`;
  return { logId: row.id, jobId: row.jobId, jobGroup: row.jobGroup, from: format(new Date(value - 60_000)), to: format(new Date(value + 60_000)) };
}

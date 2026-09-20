export type Form = Record<string, string | number>;
import { locale } from './i18n';
export type Job = {
  id?: number; jobGroup: number; scheduleType: string; scheduleConf: string; misfireStrategy: string; jobDesc: string; author: string;
  alarmEmail: string; executorRouteStrategy: string; executorHandler: string;
  executorParam: string; executorBlockStrategy: string; executorTimeout: number;
  executorFailRetryCount: number; glueType: string; childJobId: string; triggerStatus?: number;
};
export type Group = { id?: number; appname: string; title: string; addressType: number; addressList: string };
export type JobOption = { id: number; jobGroup: number; jobDesc: string };
export type LogRow = { id: number; jobId: number; jobGroup: number; triggerTime: string | number; handleTime?: string | number; triggerCode: number; handleCode: number; triggerMsg?: string; handleMsg?: string; executorAddress: string; executorHandler?: string; executorParam?: string };
export type Page<T> = { recordsTotal: number; recordsFiltered: number; data: T[] };
export type Info = { name: string; baseUrl: string; environment: string; readOnly: boolean; authMode: 'password'; admin: boolean; username: string; adminVersion: string };
export const schedules = [['NONE', '无调度'], ['CRON', 'Cron'], ['FIX_RATE', '固定间隔']] as const;
export const misfires = [['DO_NOTHING', '忽略'], ['FIRE_ONCE_NOW', '立即执行一次']] as const;
export const routes = [['FIRST', '第一个'], ['LAST', '最后一个'], ['ROUND', '轮询'], ['RANDOM', '随机'], ['CONSISTENT_HASH', '一致性 HASH'], ['LEAST_FREQUENTLY_USED', '最不经常使用'], ['LEAST_RECENTLY_USED', '最近最少使用'], ['FAILOVER', '故障转移'], ['BUSYOVER', '忙碌转移'], ['SHARDING_BROADCAST', '分片广播']] as const;
export const blocks = [['SERIAL_EXECUTION', '单机串行'], ['DISCARD_LATER', '丢弃后续调度'], ['COVER_EARLY', '覆盖之前调度']] as const;
export function jobStatus(status?: number): { label: string; tone: string } {
  const states: Record<number, [string, string]> = { 1: ['运行中', 'success'], 0: ['已停止', 'neutral'] };
  const [label, tone] = states[status ?? -1] || ['未知', 'neutral'];
  return { label, tone };
}
export function newJob(group: number): Job {
  return { jobGroup: group, scheduleType: 'CRON', scheduleConf: '0 0/5 * * * ?', misfireStrategy: 'DO_NOTHING', jobDesc: '', author: '', alarmEmail: '', executorRouteStrategy: 'FIRST', executorHandler: '', executorParam: '', executorBlockStrategy: 'SERIAL_EXECUTION', executorTimeout: 0, executorFailRetryCount: 0, glueType: 'BEAN', childJobId: '' };
}
export function newGroup(): Group { return { appname: '', title: '', addressType: 0, addressList: '' }; }
export function jobForm(job: Job): Form {
  const result: Form = {};
  for (const key of ['jobGroup', 'scheduleType', 'scheduleConf', 'misfireStrategy', 'jobDesc', 'author', 'alarmEmail', 'executorRouteStrategy', 'executorHandler', 'executorParam', 'executorBlockStrategy', 'executorTimeout', 'executorFailRetryCount', 'glueType', 'childJobId'] as const) result[key] = job[key] ?? '';
  if (job.id) result.id = job.id;
  return result;
}
export function groupForm(group: Group): Form {
  const { appname, title, addressType, addressList } = group;
  const result: Form = { appname, title, addressType, addressList: addressList || '' };
  if (group.id) result.id = group.id;
  return result;
}
const integer = (value: number, min = 0, max = 2147483647) => Number.isInteger(value) && value >= min && value <= max;
export function validateJob(job: Job): string {
  if (!integer(job.jobGroup, 1)) return '请选择执行器';
  if (!job.jobDesc?.trim()) return '任务描述不能为空';
  if (!schedules.some(([key]) => key === job.scheduleType)) return '调度类型无效';
  if (job.scheduleType === 'CRON' && !job.scheduleConf?.trim()) return 'Cron 不能为空';
  if (job.scheduleType === 'FIX_RATE' && (!/^\d+$/.test(job.scheduleConf) || !integer(Number(job.scheduleConf), 1))) return '间隔秒数必须是正整数';
  if (!misfires.some(([key]) => key === job.misfireStrategy)) return '调度过期策略无效';
  if (!job.author?.trim()) return '负责人不能为空';
  if (job.glueType === 'BEAN' && !job.executorHandler?.trim()) return 'Handler 不能为空';
  if (!integer(job.executorTimeout)) return '超时必须是非负整数';
  if (!integer(job.executorFailRetryCount)) return '重试次数必须是非负整数';
  if (!routes.some(([key]) => key === job.executorRouteStrategy)) return '路由策略无效';
  if (!blocks.some(([key]) => key === job.executorBlockStrategy)) return '阻塞策略无效';
  if (job.childJobId?.trim() && !/^\d+(,\d+)*$/.test(job.childJobId)) return '子任务 ID 格式无效';
  return '';
}
export function validateGroup(group: Group): string {
  if (!group.appname?.trim() || group.appname.length < 4 || group.appname.length > 64) return 'AppName 长度必须为 4..64';
  if (!group.title?.trim()) return '执行器名称不能为空';
  if (!integer(group.addressType, 0, 1)) return '注册方式无效';
  if (group.addressType === 1) {
    if (!group.addressList?.trim()) return '手动注册地址不能为空';
    for (const address of group.addressList.split(',')) {
      try { const url = new URL(address.trim()); if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return '执行器地址必须是 HTTP/HTTPS URL'; }
      catch { return '执行器地址无效'; }
    }
  }
  return '';
}
export type LogChunk = { fromLineNum: number; toLineNum: number; logContent: string; end: boolean };
export function advanceLog(cursor: number, chunk: LogChunk): { next: number; text: string; end: boolean } {
  if (!chunk || !integer(chunk.fromLineNum, 1) || !integer(chunk.toLineNum) || chunk.fromLineNum !== cursor || typeof chunk.logContent !== 'string' || typeof chunk.end !== 'boolean') throw new Error('执行日志响应不符合原版契约');
  const hasLines = chunk.toLineNum >= cursor && chunk.logContent !== '';
  return { next: hasLines ? chunk.toLineNum + 1 : cursor, text: hasLines ? chunk.logContent : '', end: chunk.end };
}
export function triggerMillis(value: string | number): number {
  const result = typeof value === 'number' ? value : /^\d+$/.test(value) ? Number(value) : Date.parse(value);
  if (!Number.isSafeInteger(result) || result <= 0) throw new Error('日志触发时间无效');
  return result;
}
export function timeLabel(value?: string | number): string {
  if (!value) return '-';
  try { return new Date(triggerMillis(value)).toLocaleString(locale.value, { hour12: false }); } catch { return String(value); }
}
export function filterTime(start: string, end: string): string {
  if (!start && !end) return '';
  if (!start || !end || !Number.isFinite(Date.parse(start)) || !Number.isFinite(Date.parse(end)) || Date.parse(start) > Date.parse(end)) throw new Error('请填写完整且顺序正确的时间范围');
  const format = (v: string) => v.replace('T', ' ') + (v.length === 16 ? ':00' : '');
  return `${format(start)} - ${format(end)}`;
}

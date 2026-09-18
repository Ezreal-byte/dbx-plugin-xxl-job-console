import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { jobStatus, jobForm, groupForm, newJob, validateJob, validateGroup, advanceLog, triggerMillis, filterTime } from './domain';

describe('official 2.3.0 domain contracts', () => {
  it('uses one required host/port pair without a duplicate service address', () => {
    const manifest = JSON.parse(readFileSync(new URL('../manifest.json', import.meta.url), 'utf8'));
    const fields = manifest.contributions.find((c: { type: string }) => c.type === 'connection-provider').fields;
    expect(fields.filter((f: { required?: boolean }) => f.required).map((f: { binding: string }) => f.binding)).toEqual(['host', 'port', 'username', 'password']);
    expect(fields.find((f: { binding: string }) => f.binding === 'port').default).toBe(8080);
    expect(fields.find((f: { binding: string }) => f.binding === 'password').type).toBe('password');
    expect(fields.some((f: { key: string }) => ['base_url', 'transport_host', 'transport_port'].includes(f.key))).toBe(false);
  });
  it('uses numeric triggerStatus rather than Quartz states', () => {
    expect(jobStatus(1)).toEqual({ label: '运行中', tone: 'success' });
    expect(jobStatus(0).label).toBe('已停止');
    expect(jobStatus(9).tone).toBe('neutral');
  });
  it('keeps all editable values but never posts stored glue source', () => {
    const job = { ...newJob(3), id: 7, glueType: 'GLUE_SHELL', glueSource: 'private script', executorParam: 'x', childJobId: '8,9' };
    const form = jobForm(job);
    expect(form.glueType).toBe('GLUE_SHELL');
    expect(form.executorParam).toBe('x');
    expect(form.childJobId).toBe('8,9');
    expect(form.scheduleType).toBe('CRON');
    expect(form.scheduleConf).toBe('0 0/5 * * * ?');
    expect(form.misfireStrategy).toBe('DO_NOTHING');
    expect(form).not.toHaveProperty('jobCron');
    expect(form).not.toHaveProperty('glueSource');
    expect(jobForm(newJob(3))).not.toHaveProperty('id');
  });
  it('requires the old BEAN handler and complete job fields', () => {
    expect(validateJob(newJob(0))).toContain('执行器');
    const job = { ...newJob(3), jobDesc: 'Demo', author: 'demo' };
    expect(validateJob(job)).toContain('Handler');
    job.executorHandler = 'demoHandler';
    expect(validateJob(job)).toBe('');
    job.executorFailRetryCount = -1;
    expect(validateJob(job)).toContain('重试');
  });
  it('uses official group fields without custom-only limits/order', () => {
    const g = { appname: 'demo-executor', title: 'Demo', addressType: 1, addressList: '' };
    expect(validateGroup(g)).toContain('地址');
    g.addressList = 'http://node:9999/';
    expect(validateGroup(g)).toBe('');
    expect(groupForm(g).appname).toBe('demo-executor');
    expect(groupForm(g)).not.toHaveProperty('taskLimit');
    expect(groupForm(g)).not.toHaveProperty('order');
  });
  it('validates fixed-rate/none schedules and preserves expiry strategy', () => {
    const job = { ...newJob(3), jobDesc: 'Demo', author: 'demo', executorHandler: 'demoHandler', scheduleType: 'FIX_RATE', scheduleConf: '5', misfireStrategy: 'FIRE_ONCE_NOW' };
    expect(validateJob(job)).toBe('');
    expect(jobForm(job).misfireStrategy).toBe('FIRE_ONCE_NOW');
    job.scheduleConf = '0';
    expect(validateJob(job)).toContain('秒');
    job.scheduleType = 'NONE'; job.scheduleConf = '';
    expect(validateJob(job)).toBe('');
    job.scheduleType = 'FIX_DELAY';
    expect(validateJob(job)).toContain('调度类型');
  });
  it('keeps the cursor unchanged for an empty executor chunk', () => {
    expect(advanceLog(10, { fromLineNum: 10, toLineNum: 9, logContent: '', end: false })).toEqual({ next: 10, text: '', end: false });
    expect(advanceLog(1, { fromLineNum: 1, toLineNum: 2, logContent: 'one\ntwo\n', end: true })).toEqual({ next: 3, text: 'one\ntwo\n', end: true });
    expect(() => advanceLog(3, { fromLineNum: 1, toLineNum: 2, logContent: 'duplicate', end: false })).toThrow();
  });
  it('converts service timestamps to executor request milliseconds', () => {
    expect(triggerMillis(1720000000000)).toBe(1720000000000);
    expect(triggerMillis('2026-09-17T00:00:00Z')).toBe(1789603200000);
    expect(() => triggerMillis('not-a-date')).toThrow();
  });
  it('uses the exact legacy date range separator and rejects reversed ranges', () => {
    expect(filterTime('2026-09-17T09:00', '2026-09-17T10:00')).toBe('2026-09-17 09:00:00 - 2026-09-17 10:00:00');
    expect(filterTime('', '')).toBe('');
    expect(() => filterTime('2026-09-17T10:00', '')).toThrow();
    expect(() => filterTime('2026-09-17T10:00', '2026-09-17T09:00')).toThrow();
  });
  it('uses explicit actions instead of sandbox-blocked native form submission', () => {
    for (const file of ['App.vue', 'JobEditor.vue', 'GroupEditor.vue']) {
      const source = readFileSync(new URL(file, import.meta.url), 'utf8');
      expect(source).not.toMatch(/<form\b|type="submit"/);
    }
  });
});

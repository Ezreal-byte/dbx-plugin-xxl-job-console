import { describe, expect, it, vi, afterEach } from 'vitest';
import { logLookup, previewOperation, recommendations } from './automation';

afterEach(() => vi.unstubAllGlobals());
describe('AI context and previews', () => {
  it('uses explicit resource identifiers without adding job parameters or log content', () => {
    const value = recommendations({ connectionId: 'test', page: 'logs', resource: { kind: 'log', id: 8, jobId: 7, executorId: 1 } });
    expect(value.context.resource).toEqual({ kind: 'log', id: 8, jobId: 7, executorId: 1 });
    expect(value.items[0].prompt).toContain('"jobId":7');
    expect(JSON.stringify(value)).not.toMatch(/executorParam|password|cookie|glueSource/);
  });
  it('locates an old log within a narrow server-compatible time range', () => {
    const row = { id: 8, jobId: 7, jobGroup: 1, triggerTime: new Date(2026, 0, 3, 12, 0, 0).getTime() };
    const lookup = logLookup(row);
    expect(lookup).toEqual({ logId: 8, jobId: 7, jobGroup: 1, from: '2026-01-03 11:59:00', to: '2026-01-03 12:01:00' });
    expect(() => logLookup({ ...row, triggerTime: 'invalid' })).toThrow();
  });
  it('asks the backend to preview and never confirms or executes implicitly', async () => {
    const invoke = vi.fn().mockResolvedValue({ previewToken: 'token', executed: false });
    vi.stubGlobal('window', { dbxPlugin: { invoke } });
    const args = { id: 7, jobGroup: 1, parameterMode: 'override', executorParam: '' };
    await previewOperation('connection-a', 'xxljob/trigger', args);
    expect(invoke).toHaveBeenCalledExactlyOnceWith('xxljob/preview', { connectionId: 'connection-a', form: { operation: 'xxljob/trigger', arguments: args } }, { timeoutMs: 45000 });
  });
});

import { describe, expect, it } from 'vitest';
import { buildCron, defaultCronRules, parseCron } from './cron';

describe('Quartz Cron editor', () => {
  it('starts with valid day-of-month and day-of-week defaults', () => {
    expect(buildCron(defaultCronRules())).toMatch(/^\* \* \* \* \* \? \*$/);
  });
  it('parses an existing 2.3 style expression for editing', () => {
    const rules = parseCron('0 0/5 * * * ?');
    expect(rules?.minute).toMatchObject({ mode: 'step', start: 0, step: 5 });
    expect(rules && buildCron(rules)).toBe('0 0/5 * * * ? *');
    expect(parseCron('0 50/5 * * * ?')?.minute.start).toBe(50);
  });
  it('makes a weekday rule mutually exclusive with day-of-month', () => {
    const rules = defaultCronRules();
    rules.day.mode = 'specific'; rules.day.values = [1, 15];
    rules.week.mode = 'specific'; rules.week.values = [2, 4];
    expect(buildCron(rules)).toBe('* * * ? * 2,4 *');
  });
  it('keeps manual expressions that the controls cannot represent', () => {
    expect(parseCron('0 0 9 L * ?')).toBeNull();
  });
});

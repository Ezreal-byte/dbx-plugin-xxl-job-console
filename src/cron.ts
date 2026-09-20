export type CronField = 'second' | 'minute' | 'hour' | 'day' | 'month' | 'week' | 'year';
export type CronMode = 'any' | 'cycle' | 'step' | 'specific';
export type CronRule = { mode: CronMode; start: number; end: number; step: number; values: number[] };
export const cronFields: { key: CronField; min: number; max: number }[] = [
  { key: 'second', min: 0, max: 59 }, { key: 'minute', min: 0, max: 59 },
  { key: 'hour', min: 0, max: 23 }, { key: 'day', min: 1, max: 31 },
  { key: 'month', min: 1, max: 12 }, { key: 'week', min: 1, max: 7 },
  { key: 'year', min: new Date().getFullYear(), max: new Date().getFullYear() + 30 },
];
export type CronRules = Record<CronField, CronRule>;
export function defaultCronRules(): CronRules {
  return Object.fromEntries(cronFields.map(({ key, min, max }) => [key, { mode: 'any', start: min, end: Math.min(min + 1, max), step: 1, values: [min] }])) as CronRules;
}
function fieldText(rule: CronRule, field: CronField): string {
  if (rule.mode === 'any') return field === 'week' ? '?' : '*';
  if (rule.mode === 'cycle') return `${rule.start}-${rule.end}`;
  if (rule.mode === 'step') return `${rule.start}/${rule.step}`;
  return [...new Set(rule.values)].sort((a, b) => a - b).join(',') || String(rule.start);
}
export function buildCron(rules: CronRules): string {
  const weekActive = rules.week.mode !== 'any';
  return cronFields.map(({ key }) => key === 'day' && weekActive ? '?' : key === 'week' && !weekActive ? '?' : fieldText(rules[key], key)).join(' ');
}
export function parseCron(expression: string): CronRules | null {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 6 && parts.length !== 7) return null;
  const rules = defaultCronRules();
  for (let index = 0; index < parts.length; index++) {
    const { key, min, max } = cronFields[index];
    const value = parts[index];
    const rule = rules[key];
    if (value === '*' || value === '?') continue;
    let match: RegExpMatchArray | null;
    if ((match = value.match(/^(\d+)-(\d+)$/))) { rule.mode = 'cycle'; rule.start = +match[1]; rule.end = +match[2]; }
    else if ((match = value.match(/^(\d+|\*)\/(\d+)$/))) { rule.mode = 'step'; rule.start = match[1] === '*' ? min : +match[1]; rule.step = +match[2]; }
    else if (/^\d+(,\d+)*$/.test(value)) { rule.mode = 'specific'; rule.values = value.split(',').map(Number); }
    else return null;
    if (rule.mode === 'cycle' && (rule.start < min || rule.start > max || rule.end > max || rule.end < rule.start)) return null;
    if (rule.mode === 'step' && (rule.start < min || rule.start > max || rule.step < 1 || rule.step > max - min + 1)) return null;
    if (rule.mode === 'specific' && rule.values.some(v => v < min || v > max)) return null;
  }
  if (rules.day.mode !== 'any' && rules.week.mode !== 'any') return null;
  return rules;
}

import { describe, expect, it } from 'vitest';
import { matchesOption } from './select';

const executor = {
  value: 7,
  label: '订单中心 · order-center-executor',
  keywords: '订单中心 order-center-executor',
};

describe('searchable selects', () => {
  it('matches empty and whitespace-only queries', () => {
    expect(matchesOption(executor, '')).toBe(true);
    expect(matchesOption(executor, '   ')).toBe(true);
  });

  it('matches labels, keywords and values case-insensitively', () => {
    expect(matchesOption(executor, '订单')).toBe(true);
    expect(matchesOption(executor, 'ORDER-CENTER')).toBe(true);
    expect(matchesOption(executor, 'executor')).toBe(true);
    expect(matchesOption(executor, '7')).toBe(true);
    expect(matchesOption(executor, 'missing')).toBe(false);
  });

  it('requires every whitespace-separated token to match', () => {
    expect(matchesOption(executor, '订单 order')).toBe(true);
    expect(matchesOption(executor, '订单 missing')).toBe(false);
  });

  it('ignores surrounding and repeated whitespace', () => {
    expect(matchesOption(executor, '  订单   center  ')).toBe(true);
  });
});

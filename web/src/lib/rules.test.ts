import { describe, expect, it } from 'vitest';
import type { Rule } from './pb';
import { describeRule, matchRule } from './rules';

const rule = (id: string, match: string, extra: Partial<Rule> = {}): Rule =>
  ({ id, match, category: '', paid_by: '', split_mode: '', payer_percent: 0, tags: [], ...extra }) as Rule;

const rules = [rule('a', 'Delhaize'), rule('b', 'delhaize proxy'), rule('c', 'netflix'), rule('d', '  ')];

describe('matchRule', () => {
  it('finds a rule anywhere in the description, ignoring case', () => {
    expect(matchRule(rules, 'Big shop at DELHAIZE')?.id).toBe('a');
    expect(matchRule(rules, 'Netflix November')?.id).toBe('c');
  });

  it('prefers the most specific rule', () => {
    expect(matchRule(rules, 'Delhaize Proxy Ixelles')?.id).toBe('b');
  });

  it('matches nothing for empty or unknown text', () => {
    expect(matchRule(rules, '')).toBeUndefined();
    expect(matchRule(rules, 'Bakery')).toBeUndefined();
  });
});

describe('describeRule', () => {
  const cat = (id: string) => (id === 'g' ? 'Groceries' : '?');
  const name = (id: string) => (id === 'a' ? 'You' : 'Sam');
  const other = (id: string) => (id === 'a' ? 'Sam' : 'You');
  it('says what it sets', () => {
    expect(describeRule(rule('x', 'd', { category: 'g', split_mode: 'equal' }), cat, name, other)).toBe('Groceries · 50/50');
    expect(describeRule(rule('x', 'n', { paid_by: 'a', split_mode: 'full', tags: ['tv'] }), cat, name, other)).toBe('Sam owes all · You paid · #tv');
    expect(describeRule(rule('x', 'p', { split_mode: 'percent', payer_percent: 70 }), cat, name, other)).toBe('70% / 30%');
    expect(describeRule(rule('x', 'e'), cat, name, other)).toBe('Nothing yet');
  });
});

import { describe, expect, it } from 'vitest';
import type { Expense } from './pb';
import { addTag, allTags, normalizeTag, recentTags, tagSummary } from './tags';

const exp = (day: string, cents: number, paid_by: string, owed: number, tags: string[] | null): Expense =>
  ({ id: day + cents, description: 'x', amount_cents: cents, paid_by, owed_cents: owed, date: `${day} 12:00:00.000Z`, tags }) as Expense;

describe('normalizeTag', () => {
  it('matches the server rules', () => {
    expect(normalizeTag('#Lisbon 2026')).toBe('lisbon-2026');
    expect(normalizeTag('  ##road  trip! ')).toBe('road-trip');
    expect(normalizeTag('wedding_anna/tom')).toBe('wedding-anna-tom');
    expect(normalizeTag('Café-Crème')).toBe('café-crème');
    expect(normalizeTag('---')).toBe('');
    expect(normalizeTag('abcdefghijklmnopqrstuvwxyz0123456789')).toBe('abcdefghijklmnopqrstuvwxyz012345');
  });
});

describe('addTag', () => {
  it('skips empty and repeated tags', () => {
    expect(addTag([], 'Lisbon')).toEqual(['lisbon']);
    expect(addTag(['lisbon'], '#lisbon')).toEqual(['lisbon']);
    expect(addTag(['lisbon'], ' # ')).toEqual(['lisbon']);
  });
});

const expenses = [
  exp('2026-10-20', 3000, 'a', 1500, ['lisbon']),
  exp('2026-10-18', 9000, 'b', 4500, ['lisbon', 'flights']),
  exp('2026-07-01', 2000, 'a', 1000, ['birthday']),
  exp('2026-10-10', 1000, 'a', 0, null),
];

describe('allTags and recentTags', () => {
  it('lists tags by last use', () => {
    expect(allTags(expenses)).toEqual(['lisbon', 'flights', 'birthday']);
    expect(recentTags(expenses, '2026-10-25')).toEqual(['lisbon', 'flights']);
  });
});

describe('tagSummary', () => {
  it('adds up a tag and who owes whom for it', () => {
    expect(tagSummary(expenses, 'lisbon', 'a')).toEqual({
      count: 2,
      total: 12000,
      from: '2026-10-18',
      to: '2026-10-20',
      paid: { a: 3000, b: 9000 },
      net: -3000,
    });
  });
});

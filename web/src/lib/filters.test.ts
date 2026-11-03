import { describe, expect, it } from 'vitest';
import type { Expense, Settlement } from './pb';
import type { Entry } from './state.svelte';
import { activeFilters, filterEntries, noFilters, toCsv } from './filters';

const expense = (id: string, description: string, cents: number, paid_by: string, category: string): Entry => {
  const item = { id, description, amount_cents: cents, paid_by, category, note: '', date: '2026-10-01 12:00:00.000Z', split_mode: 'equal', owed_cents: cents / 2 } as Expense;
  return { kind: 'expense', date: item.date, created: '', item };
};
const payment: Entry = {
  kind: 'settlement',
  date: '2026-10-02 12:00:00.000Z',
  created: '',
  item: { id: 's', from_user: 'b', to_user: 'a', amount_cents: 5000, note: 'thanks, all good', date: '2026-10-02 12:00:00.000Z' } as Settlement,
};
const train = expense('2', 'Train', 1200, 'b', 'transport');
(train.item as Expense).tags = ['lisbon', 'trip'];
const entries = [expense('1', 'Groceries', 4000, 'a', 'food'), train, payment];
const text = (e: Entry) => (e.kind === 'expense' ? [e.item.description] : ['settlement payment', e.item.note]);
const ids = (es: Entry[]) => es.map((e) => e.item.id);

describe('filterEntries', () => {
  it('keeps everything with no filters', () => {
    expect(filterEntries(entries, '', noFilters, text)).toHaveLength(3);
  });

  it('filters by type, payer, category and amount', () => {
    expect(ids(filterEntries(entries, '', { ...noFilters, kind: 'settlement' }, text))).toEqual(['s']);
    expect(ids(filterEntries(entries, '', { ...noFilters, paidBy: 'b' }, text))).toEqual(['2', 's']);
    expect(ids(filterEntries(entries, '', { ...noFilters, category: 'food' }, text))).toEqual(['1']);
    expect(ids(filterEntries(entries, '', { ...noFilters, tag: 'lisbon' }, text))).toEqual(['2']);
    expect(ids(filterEntries(entries, '', { ...noFilters, min: 1500, max: 4000 }, text))).toEqual(['1']);
  });

  it('combines filters with the search', () => {
    expect(ids(filterEntries(entries, 'TRAIN', { ...noFilters, paidBy: 'b' }, text))).toEqual(['2']);
  });

  it('counts the active filters', () => {
    expect(activeFilters(noFilters)).toBe(0);
    expect(activeFilters({ ...noFilters, kind: 'expense', min: 100 })).toBe(2);
  });
});

describe('toCsv', () => {
  it('writes the export columns and quotes where needed', () => {
    const csv = toCsv(entries, (id) => (id === 'a' ? 'Alex' : 'Sam'), (c) => c, (id) => (id === '1' ? ['Alex: "big" shop'] : []));
    const lines = csv.trim().split('\r\n');
    expect(lines[0]).toBe('type,date,description,category,amount,paid_by,split_mode,owed_by_other,note,tags');
    expect(lines[1]).toBe('expense,2026-10-01,Groceries,food,40.00,Alex,equal,20.00,"Alex: ""big"" shop",');
    expect(lines[2]).toBe('expense,2026-10-01,Train,transport,12.00,Sam,equal,6.00,,#lisbon #trip');
    expect(lines[3]).toBe('settlement,2026-10-02,Sam paid Alex,,50.00,Sam,,,"thanks, all good",');
  });
});

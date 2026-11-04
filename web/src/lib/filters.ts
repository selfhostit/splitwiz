// History's search and filters, and the CSV of what they found. Pure
// functions, like stats.ts.

import type { Entry } from './state.svelte';
import { tagsOf } from './tags';

export type Filters = {
  kind: 'all' | 'expense' | 'settlement';
  /** a user id; for a payment, whoever paid it */
  paidBy: string;
  /** a category id; '' = any */
  category: string;
  /** a tag; '' = any */
  tag: string;
  /** in cents; null = no bound */
  min: number | null;
  max: number | null;
};

export const noFilters: Filters = { kind: 'all', paidBy: '', category: '', tag: '', min: null, max: null };

export function activeFilters(f: Filters): number {
  return [f.kind !== 'all', !!f.paidBy, !!f.category, !!f.tag, f.min !== null || f.max !== null].filter(Boolean).length;
}

/** Text an entry can be found by: description, note, category and comments. */
export type Searchable = (e: Entry) => string[];

export function filterEntries(entries: Entry[], query: string, f: Filters, text: Searchable): Entry[] {
  const q = query.trim().toLowerCase();
  return entries.filter((e) => {
    if (f.kind !== 'all' && e.kind !== f.kind) return false;
    const payer = e.kind === 'expense' ? e.item.paid_by : e.item.from_user;
    if (f.paidBy && payer !== f.paidBy) return false;
    if (f.category && (e.kind !== 'expense' || e.item.category !== f.category)) return false;
    if (f.tag && (e.kind !== 'expense' || !tagsOf(e.item).includes(f.tag))) return false;
    if (f.min !== null && e.item.amount_cents < f.min) return false;
    if (f.max !== null && e.item.amount_cents > f.max) return false;
    return !q || text(e).some((t) => t.toLowerCase().includes(q));
  });
}

const cell = (s: string) => (/[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s);
const money = (cents: number) => (cents / 100).toFixed(2);

/** The same columns as the full export in Settings. */
export function toCsv(
  entries: Entry[],
  name: (userId: string) => string,
  category: (id: string) => string,
  comments: (expenseId: string) => string[],
): string {
  const rows = [['type', 'date', 'description', 'category', 'amount', 'paid_by', 'split_mode', 'owed_by_other', 'note', 'tags']];
  for (const e of entries) {
    const day = e.date.slice(0, 10);
    if (e.kind === 'expense') {
      const x = e.item;
      const note = [x.note, ...comments(x.id)].filter(Boolean).join(' | ');
      rows.push(['expense', day, x.description, category(x.category), money(x.amount_cents), name(x.paid_by), x.split_mode, money(x.owed_cents), note, tagsOf(x).map((t) => '#' + t).join(' ')]);
    } else {
      const s = e.item;
      rows.push(['settlement', day, `${name(s.from_user)} paid ${name(s.to_user)}`, '', money(s.amount_cents), name(s.from_user), '', '', s.note, '']);
    }
  }
  return rows.map((r) => r.map(cell).join(',')).join('\r\n') + '\r\n';
}

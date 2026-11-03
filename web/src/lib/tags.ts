// Tags follow a trip or an event across categories ("lisbon-2026").
// Pure functions, like stats.ts; expenses come newest first.

import type { Expense } from './pb';

export const MAX_TAGS = 10;
const MAX_LEN = 32;

/**
 * What was typed, as a tag: lower case, no "#", and spaces or other
 * separators become a single "-". Same rules as the server's
 * ledger.NormalizeTag; '' when nothing usable is left.
 */
export function normalizeTag(typed: string): string {
  let out = '';
  let dash = false;
  let n = 0;
  for (const ch of typed.trim().toLowerCase()) {
    if (n >= MAX_LEN) break;
    if (/[\p{L}\p{N}]/u.test(ch)) {
      if (dash && out) {
        out += '-';
        n++;
      }
      dash = false;
      out += ch;
      n++;
    } else {
      dash = true;
    }
  }
  return out.replace(/-+$/, '');
}

/** Adds a tag to a list, unless it's empty, already there, or the list is full. */
export function addTag(tags: string[], typed: string): string[] {
  const t = normalizeTag(typed);
  return !t || tags.includes(t) || tags.length >= MAX_TAGS ? tags : [...tags, t];
}

export const tagsOf = (e: { tags?: string[] | null }): string[] => e.tags ?? [];

/** Every tag in use, the most recently used first. */
export function allTags(expenses: Expense[]): string[] {
  const latest = new Map<string, string>();
  for (const e of expenses) {
    for (const t of tagsOf(e)) {
      const day = e.date.slice(0, 10);
      if ((latest.get(t) ?? '') < day) latest.set(t, day);
    }
  }
  return [...latest.entries()].sort((a, b) => b[1].localeCompare(a[1]) || a[0].localeCompare(b[0])).map(([t]) => t);
}

/** Tags used in the last `days` days up to `today`, most recent first: the ones worth suggesting. */
export function recentTags(expenses: Expense[], today: string, days = 60, n = 6): string[] {
  const since = new Date(today + 'T12:00:00');
  since.setDate(since.getDate() - days);
  const from = since.toISOString().slice(0, 10);
  return allTags(expenses.filter((e) => e.date.slice(0, 10) >= from)).slice(0, n);
}

export type TagSummary = {
  count: number;
  total: number;
  from: string;
  to: string;
  /** user id → what they paid */
  paid: Record<string, number>;
  /** what `me` gets back (positive) or owes (negative) from these expenses alone */
  net: number;
};

/** A tag's totals: what it cost, who paid what, and who owes whom for it. */
export function tagSummary(expenses: Expense[], tag: string, me: string): TagSummary {
  const s: TagSummary = { count: 0, total: 0, from: '', to: '', paid: {}, net: 0 };
  for (const e of expenses) {
    if (!tagsOf(e).includes(tag)) continue;
    const day = e.date.slice(0, 10);
    s.count++;
    s.total += e.amount_cents;
    s.paid[e.paid_by] = (s.paid[e.paid_by] ?? 0) + e.amount_cents;
    s.net += e.paid_by === me ? e.owed_cents : -e.owed_cents;
    if (!s.from || day < s.from) s.from = day;
    if (day > s.to) s.to = day;
  }
  return s;
}

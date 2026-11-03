// Shortcuts for adding the expenses you add all the time. Pure functions,
// like stats.ts; expenses come newest first, as the app keeps them.

import type { Expense } from './pb';

const dayOf = (date: string) => date.slice(0, 10);

const key = (description: string) => description.trim().toLowerCase();

/** The latest expense with this exact description, if any. */
export function lastLike(expenses: Expense[], description: string): Expense | undefined {
  const k = key(description);
  return k ? expenses.find((e) => key(e.description) === k) : undefined;
}

/**
 * The expenses added most often in the last 90 days (at least twice),
 * most frequent first, each as its latest copy.
 */
export function favorites(expenses: Expense[], today: string, n = 4): Expense[] {
  const since = new Date(today + 'T12:00:00');
  since.setDate(since.getDate() - 90);
  const from = since.toISOString().slice(0, 10);
  const seen = new Map<string, { latest: Expense; count: number }>();
  for (const e of expenses) {
    if (dayOf(e.date) < from || dayOf(e.date) > today) continue;
    const s = seen.get(key(e.description));
    if (s) s.count++;
    else seen.set(key(e.description), { latest: e, count: 1 });
  }
  return [...seen.values()]
    .filter((s) => s.count >= 2)
    .sort((a, b) => b.count - a.count)
    .slice(0, n)
    .map((s) => s.latest);
}

/**
 * Past expenses whose description matches what's being typed, each as its
 * latest copy: descriptions starting with the text first, then ones that
 * contain it, the more often used first. An exact match isn't suggested.
 */
export function suggest(expenses: Expense[], typed: string, n = 4): Expense[] {
  const q = key(typed);
  if (!q) return [];
  const seen = new Map<string, { latest: Expense; count: number; starts: boolean }>();
  for (const e of expenses) {
    const k = key(e.description);
    if (k === q || !k.includes(q)) continue;
    const s = seen.get(k);
    if (s) s.count++;
    else seen.set(k, { latest: e, count: 1, starts: k.startsWith(q) || k.includes(' ' + q) });
  }
  return [...seen.values()]
    .sort((a, b) => Number(b.starts) - Number(a.starts) || b.count - a.count)
    .slice(0, n)
    .map((s) => s.latest);
}

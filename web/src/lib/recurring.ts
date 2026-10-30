// Turning existing expenses into repeating ones, and spotting expenses that
// already repeat (imported history carries no repeat information).

import type { AmountChange, Every, Expense, Recurring } from './pb';

/**
 * Same rule as the server's ledger.NextDate: monthly/yearly stay on
 * `anchorDay` (default: the day of `day`), clamped to short months.
 */
export function nextDate(day: string, every: Every, anchorDay = 0): string {
  const [y, m, d] = day.split('-').map(Number);
  const fmt = (yy: number, mm: number, dd: number) =>
    `${yy}-${String(mm).padStart(2, '0')}-${String(dd).padStart(2, '0')}`;
  if (every === 'week') {
    const t = new Date(Date.UTC(y, m - 1, d + 7));
    return fmt(t.getUTCFullYear(), t.getUTCMonth() + 1, t.getUTCDate());
  }
  const [ny, nm] = every === 'month' ? (m === 12 ? [y + 1, 1] : [y, m + 1]) : [y + 1, m];
  const last = new Date(Date.UTC(ny, nm, 0)).getUTCDate();
  return fmt(ny, nm, Math.min(anchorDay >= 1 && anchorDay <= 31 ? anchorDay : d, last));
}

/**
 * Same rule as the server's ledger.AmountOn: what an expense on `day`
 * costs, the latest change started by then or `base` before the first.
 */
export function amountOn(base: number, changes: AmountChange[] | null | undefined, day: string): number {
  let amount = base;
  let since = '';
  for (const c of changes ?? []) {
    if (c.from <= day && c.from >= since) [amount, since] = [c.amount_cents, c.from];
  }
  return amount;
}

/** True once a series has added its last expense. */
export function hasEnded(s: Pick<Recurring, 'ends_on' | 'next_date'>): boolean {
  return !!s.ends_on && s.next_date.slice(0, 10) > s.ends_on.slice(0, 10);
}

/** What a series costs per expense, with its scheduled changes and end. */
export type Plan = {
  start: string;
  every: Every;
  anchorDay?: number;
  amount: number;
  changes: AmountChange[];
  /** YYYY-MM-DD or '' */
  ends: string;
};

export type Payment = { day: string; amount_cents: number; phase: number };

/**
 * The plan's expenses from its start, at most `max`, each tagged with the
 * phase (0 = the starting amount, 1 = after the first change, ...) it falls in.
 */
export function payments(plan: Plan, max: number, until = ''): Payment[] {
  const anchor = anchorOf(plan);
  const changes = [...plan.changes].sort((a, b) => a.from.localeCompare(b.from));
  const out: Payment[] = [];
  let day = plan.start;
  for (let i = 0; i < max; i++) {
    if ((plan.ends && day > plan.ends) || (until && day > until)) break;
    const phase = changes.filter((c) => c.from <= day).length;
    out.push({ day, amount_cents: amountOn(plan.amount, changes, day), phase });
    day = nextDate(day, plan.every, anchor);
  }
  return out;
}

/** How many expenses a plan adds from its start up to and including `end`. */
export function countUntil(plan: Plan, end: string): number {
  return payments({ ...plan, ends: end }, 1000).length;
}

/** The day of the plan's `n`th expense (1-based). */
export function nthPayment(plan: Plan, n: number): string {
  let day = plan.start;
  for (let i = 1; i < n; i++) day = nextDate(day, plan.every, anchorOf(plan));
  return day;
}

// like the server, a new series stays on the day of the month it starts on
const anchorOf = (plan: Plan) => plan.anchorDay || Number(plan.start.slice(8, 10));

export type Occurrence = { series: Recurring; day: string; amount_cents: number; last: boolean; changed: boolean };

/**
 * Dates the active series will add an expense on, from `from` to `to`
 * (inclusive, YYYY-MM-DD), soonest first, with what each will cost.
 * `last` marks a series' final expense and `changed` the first one at a
 * new amount.
 */
export function upcoming(series: Recurring[], from: string, to: string): Occurrence[] {
  const out: Occurrence[] = [];
  for (const s of series) {
    if (s.paused) continue;
    const end = s.ends_on ? s.ends_on.slice(0, 10) : '';
    let day = s.next_date.slice(0, 10);
    let before = s.amount_cents;
    for (let i = 0; i < 400 && day <= to && (!end || day <= end); i++) {
      const amount = amountOn(s.amount_cents, s.changes, day);
      const next = nextDate(day, s.every, s.anchor_day);
      if (day >= from) out.push({ series: s, day, amount_cents: amount, last: !!end && next > end, changed: amount !== before });
      before = amount;
      day = next;
    }
  }
  return out.sort((a, b) => a.day.localeCompare(b.day) || b.amount_cents - a.amount_cents);
}

const norm = (s: string) => s.trim().replace(/\s+/g, ' ').toLowerCase();

/** Expenses that are "the same charge": same text, amount, payer and split. */
export function sameCharge(a: Expense, b: Expense): boolean {
  return (
    norm(a.description) === norm(b.description) &&
    a.amount_cents === b.amount_cents &&
    a.paid_by === b.paid_by &&
    a.split_mode === b.split_mode &&
    a.owed_cents === b.owed_cents
  );
}

export type Suggestion = { latest: Expense; matches: Expense[] };

const monthIndex = (day: string) => Number(day.slice(0, 4)) * 12 + Number(day.slice(5, 7));
const daysBetween = (a: string, b: string) => (Date.parse(b) - Date.parse(a)) / 86_400_000;

/**
 * Charges that came back in each of the last three months on about the
 * same day, most recently within ~6 weeks, and that aren't a series yet.
 */
export function suggestMonthly(expenses: Expense[], series: Recurring[], today: string): Suggestion[] {
  const taken = new Set(series.map((s) => norm(s.description)));
  const groups = new Map<string, Expense[]>();
  for (const e of expenses) {
    if (e.recurring || taken.has(norm(e.description))) continue;
    const key = [norm(e.description), e.amount_cents, e.paid_by, e.split_mode, e.owed_cents].join('|');
    const g = groups.get(key);
    if (g) g.push(e);
    else groups.set(key, [e]);
  }

  const out: Suggestion[] = [];
  for (const g of groups.values()) {
    if (g.length < 3) continue;
    const days = g.map((e) => e.date.slice(0, 10)).sort();
    const [a, b, c] = days.slice(-3);
    const consecutive = monthIndex(b) - monthIndex(a) === 1 && monthIndex(c) - monthIndex(b) === 1;
    const sameDay = Math.max(...[a, b, c].map((d) => Number(d.slice(8)))) - Math.min(...[a, b, c].map((d) => Number(d.slice(8)))) <= 5;
    if (!consecutive || !sameDay || daysBetween(c, today) > 45) continue;
    const latest = g.reduce((x, y) => (y.date > x.date ? y : x));
    out.push({ latest, matches: g });
  }
  return out.sort((x, y) => y.latest.amount_cents - x.latest.amount_cents);
}

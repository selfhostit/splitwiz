// The current month against the budget: what was spent so far and where the
// month is heading. Pure functions, like stats.ts.

import type { Category, Expense, Recurring } from './pb';
import { upcoming } from './recurring';

export type MonthOutlook = {
  /** YYYY-MM */
  month: string;
  /** days in the month, and today's day of it */
  days: number;
  day: number;
  /** spent by the end of each day so far: cumulative[0] is day 1 */
  cumulative: number[];
  spent: number;
  /** everyday spending per day, from the regression */
  perDay: number;
  /** repeating charges still to come this month */
  scheduled: number;
  /** projected total for each day from today to the end of the month: projection[0] is today */
  projection: number[];
  projected: number;
};

/**
 * Least-squares slope of y over x = 0, 1, 2, ... (simple linear
 * regression). Never negative: spending only goes up.
 */
export function slope(ys: number[]): number {
  const n = ys.length;
  if (n < 2) return 0;
  const mx = (n - 1) / 2;
  const my = ys.reduce((s, y) => s + y, 0) / n;
  let num = 0;
  let den = 0;
  ys.forEach((y, x) => {
    num += (x - mx) * (y - my);
    den += (x - mx) ** 2;
  });
  return Math.max(0, num / den);
}

/**
 * Projects the month's total. Everyday spending (expenses not made by a
 * repeating series) is fitted with a straight line through its running
 * total, starting from 0 the day before the 1st, and continued to the end
 * of the month. Repeating charges are not guessed: the ones already added
 * count as spent and the ones still scheduled are added on their day.
 */
export function monthOutlook(expenses: Expense[], series: Recurring[], today: string): MonthOutlook {
  const month = today.slice(0, 7);
  const [y, m] = month.split('-').map(Number);
  const days = new Date(Date.UTC(y, m, 0)).getUTCDate();
  const day = Number(today.slice(8, 10));

  const all = new Array<number>(day).fill(0);
  const everyday = new Array<number>(day).fill(0);
  for (const e of expenses) {
    const d = e.date.slice(0, 10);
    if (d.slice(0, 7) !== month || d > today) continue;
    const i = Number(d.slice(8, 10)) - 1;
    all[i] += e.amount_cents;
    if (!e.recurring) everyday[i] += e.amount_cents;
  }

  const running = (xs: number[]) => {
    let t = 0;
    return xs.map((x) => (t += x));
  };
  const cumulative = running(all);
  const spent = cumulative[day - 1] ?? 0;
  const perDay = slope([0, ...running(everyday)]);

  const scheduledOn = new Array<number>(days + 1).fill(0);
  for (const o of upcoming(series, today, `${month}-${String(days).padStart(2, '0')}`)) {
    scheduledOn[Number(o.day.slice(8, 10))] += o.amount_cents;
  }

  // starts with a charge due today that the server hasn't added yet
  let extra = scheduledOn[day];
  const projection = [spent + extra];
  for (let d = day + 1; d <= days; d++) {
    extra += scheduledOn[d];
    projection.push(Math.round(spent + perDay * (d - day) + extra));
  }

  return {
    month,
    days,
    day,
    cumulative,
    spent,
    perDay,
    scheduled: extra,
    projection,
    projected: projection.at(-1)!,
  };
}

/**
 * A starting budget to offer: the average of the last three full months,
 * rounded up to 50 (in currency units). 0 when there is no history.
 */
export function suggestBudget(expenses: Expense[], today: string): number {
  const [y, m] = today.split('-').map(Number);
  const months = [1, 2, 3].map((k) => {
    const d = new Date(Date.UTC(y, m - 1 - k, 1));
    return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
  });
  const totals = new Map(months.map((k) => [k, 0]));
  for (const e of expenses) {
    const k = e.date.slice(0, 7);
    if (totals.has(k)) totals.set(k, totals.get(k)! + e.amount_cents);
  }
  const used = [...totals.values()].filter((c) => c > 0);
  if (!used.length) return 0;
  const avg = used.reduce((s, c) => s + c, 0) / used.length;
  return Math.ceil(avg / 5000) * 5000;
}

/** Share of a category's limit that counts as "almost there", as on the server. */
export const CATEGORY_ALERT_SHARE = 0.9;

export type CategoryLimit = { category: Category; spent: number; share: number };

/** This month's spending in each category that has a monthly limit. */
export function categoryLimits(expenses: Expense[], categories: Category[], today: string): CategoryLimit[] {
  const month = today.slice(0, 7);
  const spent = new Map<string, number>();
  for (const e of expenses) {
    if (e.category && e.date.slice(0, 7) === month) spent.set(e.category, (spent.get(e.category) ?? 0) + e.amount_cents);
  }
  return categories
    .filter((c) => c.monthly_cents > 0)
    .map((c) => {
      const s = spent.get(c.id) ?? 0;
      return { category: c, spent: s, share: s / c.monthly_cents };
    })
    .sort((a, b) => b.share - a.share);
}

import { dayOf, pb, toPbDate, type AmountChange, type Every, type Expense } from './pb';
import { nextDate } from './recurring';

/**
 * Starts a repeating series from an existing expense: the next occurrence
 * is one period after it, and `link` (usually earlier copies of the same
 * charge) is marked as part of the series.
 */
export async function startSeries(
  from: Expense,
  every: Every,
  link: Expense[],
  plan: { ends_on?: string; changes?: AmountChange[] } = {},
): Promise<void> {
  const series = await pb.collection('recurring_expenses').create({
    description: from.description,
    amount_cents: from.amount_cents,
    paid_by: from.paid_by,
    split_mode: from.split_mode,
    payer_percent: from.payer_percent,
    other_share_cents: from.other_share_cents,
    category: from.category || null,
    note: from.note,
    every,
    next_date: toPbDate(nextDate(dayOf(from.date), every)),
    ...plan,
  });
  for (const e of link) {
    await pb.collection('expenses').update(e.id, { recurring: series.id });
  }
}

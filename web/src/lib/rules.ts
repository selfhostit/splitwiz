// Rules fill in an expense from its description, like Actual Budget's
// payee rules: "delhaize" → Groceries, 50/50. Pure functions, like stats.ts.

import type { Rule } from './pb';

const norm = (s: string) => s.trim().toLowerCase();

/**
 * The rule for a description: one whose text appears in it, ignoring
 * case. When several do, the longest (most specific) wins.
 */
export function matchRule(rules: Rule[], description: string): Rule | undefined {
  const d = norm(description);
  if (!d) return undefined;
  let best: Rule | undefined;
  for (const r of rules) {
    const m = norm(r.match);
    if (m && d.includes(m) && (!best || m.length > norm(best.match).length)) best = r;
  }
  return best;
}

/** What a rule sets, in words: "Groceries · 50/50 · Sam pays". */
export function describeRule(
  r: Rule,
  category: (id: string) => string,
  name: (id: string) => string,
  other: (payer: string) => string,
): string {
  const parts: string[] = [];
  if (r.category) parts.push(category(r.category));
  if (r.split_mode === 'equal') parts.push('50/50');
  else if (r.split_mode === 'percent') parts.push(`${r.payer_percent}% / ${100 - r.payer_percent}%`);
  else if (r.split_mode === 'full') parts.push(r.paid_by ? `${other(r.paid_by)} owes all` : 'the other owes all');
  if (r.paid_by) parts.push(`${name(r.paid_by)} paid`);
  for (const t of r.tags ?? []) parts.push('#' + t);
  return parts.join(' · ') || 'Nothing yet';
}

// What the other person added or changed since you last marked everything
// as seen. Pure functions, like stats.ts.

import type { Comment } from './pb';
import type { Entry } from './state.svelte';

export type Why = 'added' | 'changed' | 'commented';

/**
 * Entry id → why it's new to `me`: the other person added or changed it,
 * or commented on it, after `seenAt`. Nothing is new before a first
 * `seenAt`, and the server's own saves (repeating expenses, imports) never
 * are.
 */
export function unseen(entries: Entry[], comments: Comment[], me: string, seenAt: string): Map<string, Why> {
  const out = new Map<string, Why>();
  if (!seenAt || !me) return out;
  for (const e of entries) {
    const by = e.item.updated_by;
    if (!by || by === me) continue;
    if (e.item.created > seenAt) out.set(e.item.id, 'added');
    else if ((e.item.updated ?? '') > seenAt) out.set(e.item.id, 'changed');
  }
  const ids = new Set(entries.map((e) => e.item.id));
  for (const c of comments) {
    if (c.author !== me && c.created > seenAt && ids.has(c.expense) && !out.has(c.expense)) out.set(c.expense, 'commented');
  }
  return out;
}

/** "Sam added 2 and changed 1" */
export function describeUnseen(name: string, why: Map<string, Why>): string {
  const n = { added: 0, changed: 0, commented: 0 };
  for (const w of why.values()) n[w]++;
  const parts = [
    n.added && `added ${n.added}`,
    n.changed && `changed ${n.changed}`,
    n.commented && `commented on ${n.commented}`,
  ].filter(Boolean) as string[];
  if (!parts.length) return '';
  const list = parts.length === 1 ? parts[0] : `${parts.slice(0, -1).join(', ')} and ${parts.at(-1)}`;
  return `${name} ${list} since you last looked`;
}

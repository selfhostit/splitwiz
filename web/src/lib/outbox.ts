// Changes made while the NAS can't be reached wait here, on the phone, and
// are sent in order once it can. Until then they are laid over the last
// data loaded, so the lists already show them.

import { owedCents } from './money';

export type Collection = 'expenses' | 'settlements' | 'recurring_expenses' | 'expense_comments' | 'categories';

export type Op = {
  collection: Collection;
  action: 'create' | 'update' | 'delete';
  /** the record's id; for a create, a made-up `local-…` id until it syncs */
  target: string;
  data: Record<string, unknown>;
  /** when it was queued, in PocketBase's date format */
  queued: string;
};

const KEY = 'splitwiz-outbox';

export const isLocal = (id: string) => id.startsWith('local-');

export function localId(): string {
  return `local-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
}

export function pbNow(): string {
  return new Date().toISOString().replace('T', ' ');
}

/** True when the request never reached the server (offline, NAS down). */
export function unreachable(err: unknown): boolean {
  const e = err as { status?: number; isAbort?: boolean };
  return e?.status === 0 && !e.isAbort;
}

export function loadOutbox(): Op[] {
  try {
    const ops = JSON.parse(localStorage.getItem(KEY) ?? '[]');
    return Array.isArray(ops) ? ops : [];
  } catch {
    return [];
  }
}

export function saveOutbox(ops: Op[]) {
  try {
    if (ops.length) localStorage.setItem(KEY, JSON.stringify(ops));
    else localStorage.removeItem(KEY);
  } catch {
    // private mode or full storage: the queue lives in memory only
  }
}

/**
 * Adds a change to the queue. Changes to something that hasn't synced yet
 * are folded into its create, and deleting it drops it altogether.
 */
export function enqueue(ops: Op[], op: Op): Op[] {
  if (op.action !== 'create' && isLocal(op.target)) {
    if (op.action === 'delete') return ops.filter((o) => o.target !== op.target);
    return ops.map((o) => (o.target === op.target && o.action === 'create' ? { ...o, data: { ...o.data, ...op.data } } : o));
  }
  if (op.action === 'update') {
    const at = ops.findIndex((o) => o.target === op.target && o.action === 'update');
    if (at >= 0) return ops.map((o, i) => (i === at ? { ...o, data: { ...o.data, ...op.data } } : o));
  }
  if (op.action === 'delete') ops = ops.filter((o) => !(o.target === op.target && o.action === 'update'));
  return [...ops, op];
}

const defaults: Record<Collection, Record<string, unknown>> = {
  expenses: { note: '', recurring: '', category: '', payer_percent: 0, other_share_cents: 0 },
  settlements: { note: '' },
  recurring_expenses: { note: '', category: '', payer_percent: 0, other_share_cents: 0, paused: false, anchor_day: 0, ends_on: '', changes: [] },
  expense_comments: {},
  categories: { emoji: '', sort: 0, monthly_cents: 0 },
};

// fields the server would fill in on save
function finish(collection: Collection, r: Record<string, unknown>): Record<string, unknown> {
  if (collection === 'expenses' || collection === 'recurring_expenses') {
    r.category ??= '';
    r.owed_cents =
      owedCents(Number(r.amount_cents), String(r.split_mode), Number(r.payer_percent), Number(r.other_share_cents)) ?? 0;
  }
  return r;
}

/** The records of one collection with the queued changes applied. */
export function overlay<T extends { id: string }>(items: T[], ops: Op[], collection: Collection, me: string): T[] {
  let out = items;
  for (const op of ops) {
    if (op.collection !== collection) continue;
    if (op.action === 'create') {
      const extra = collection === 'expense_comments' ? { author: me } : {};
      const r = finish(collection, { ...defaults[collection], ...extra, ...op.data, id: op.target, created: op.queued });
      out = [r as T, ...out];
    } else if (op.action === 'update') {
      out = out.map((x) => (x.id === op.target ? (finish(collection, { ...x, ...op.data }) as T) : x));
    } else {
      out = out.filter((x) => x.id !== op.target);
    }
  }
  return out;
}

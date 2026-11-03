import { syncPush } from './push';
import { pb, type Budget, type Category, type Comment, type Expense, type Recurring, type Rule, type Settlement, type Summary, type User } from './pb';
import { unseen, type Why } from './unseen';
import { enqueue, isLocal, loadOutbox, localId, overlay, pbNow, saveOutbox, unreachable, type Collection, type Op } from './outbox';

export type Entry =
  | { kind: 'expense'; date: string; created: string; item: Expense }
  | { kind: 'settlement'; date: string; created: string; item: Settlement };

/** Everything loaded from the NAS, kept on the phone so the app opens offline. */
type Snapshot = {
  users: User[];
  categories: Category[];
  expenses: Expense[];
  settlements: Settlement[];
  recurring: Recurring[];
  budget: Budget | null;
  comments: Comment[];
  rules: Rule[];
  summary: Summary | null;
};

const CACHE = 'splitwiz-cache';
const empty: Snapshot = { users: [], categories: [], expenses: [], settlements: [], recurring: [], budget: null, comments: [], rules: [], summary: null };

/** How long a deletion can be undone. */
export const UNDO_MS = 5000;

function loadCache(): Snapshot {
  try {
    return { ...empty, ...JSON.parse(localStorage.getItem(CACHE) ?? '{}') };
  } catch {
    return empty;
  }
}

function saveCache(s: Snapshot) {
  try {
    localStorage.setItem(CACHE, JSON.stringify(s));
  } catch {
    // full or blocked storage: the app just won't open offline
  }
}

const byCategoryOrder = (a: Category, b: Category) => a.sort - b.sort || a.name.localeCompare(b.name);

class AppState {
  me = $state<User | null>(pb.authStore.isValid ? (pb.authStore.record as unknown as User) : null);
  users = $state<User[]>([]);
  categories = $state<Category[]>([]);
  expenses = $state<Expense[]>([]);
  settlements = $state<Settlement[]>([]);
  recurring = $state<Recurring[]>([]);
  budget = $state<Budget | null>(null);
  comments = $state<Comment[]>([]);
  rules = $state<Rule[]>([]);
  summary = $state<Summary | null>(null);
  loading = $state(false);
  error = $state('');
  /** changes saved on this phone, waiting for the NAS */
  outbox = $state<Op[]>(loadOutbox());
  /** the bar at the bottom: a short message, sometimes with Undo */
  toast = $state<{ id: number; message: string; undo?: () => void } | null>(null);

  entries = $derived<Entry[]>(
    [
      ...this.expenses.map((e) => ({ kind: 'expense' as const, date: e.date, created: e.created, item: e })),
      ...this.settlements.map((s) => ({ kind: 'settlement' as const, date: s.date, created: s.created, item: s })),
    ].sort((a, b) => (a.date === b.date ? b.created.localeCompare(a.created) : b.date.localeCompare(a.date))),
  );

  // comment threads by expense id, oldest first
  commentsByExpense = $derived.by(() => {
    const m = new Map<string, Comment[]>();
    for (const c of this.comments) {
      const list = m.get(c.expense);
      if (list) list.push(c);
      else m.set(c.expense, [c]);
    }
    return m;
  });

  other = $derived<User | undefined>(this.users.find((u) => u.id !== this.me?.id));

  // what the other person added or changed since I last marked it all seen
  seenAt = $derived(this.users.find((u) => u.id === this.me?.id)?.seen_at ?? '');
  unseen = $derived<Map<string, Why>>(unseen(this.entries, this.comments, this.me?.id ?? '', this.seenAt));

  private server: Snapshot = this.me ? loadCache() : empty;
  // deleted, but still undoable: hidden here, sent to the NAS later
  private pending = new Map<string, { collection: Collection; timer: ReturnType<typeof setTimeout> }>();
  private unsubscribers: (() => void)[] = [];
  private reloadTimer: ReturnType<typeof setTimeout> | undefined;
  private toastTimer: ReturnType<typeof setTimeout> | undefined;
  private syncing = false;

  constructor() {
    this.refresh();
  }

  nameOf(id: string): string {
    if (id === this.me?.id) return 'You';
    const u = this.users.find((x) => x.id === id);
    return u?.name || u?.email || 'Partner';
  }

  commentsOf(id: string): Comment[] {
    return this.commentsByExpense.get(id) ?? [];
  }

  recurringOf(id: string): Recurring | undefined {
    return id ? this.recurring.find((r) => r.id === id) : undefined;
  }

  category(id: string): Category | undefined {
    return this.categories.find((c) => c.id === id);
  }

  /** Counts everything up to now as seen, on every phone of mine. */
  async markSeen() {
    const me = this.me?.id;
    if (!me) return;
    const mark = (seen_at: string) => {
      this.server = { ...this.server, users: this.server.users.map((u) => (u.id === me ? { ...u, seen_at } : u)) };
      saveCache(this.server);
      this.refresh();
    };
    mark(pbNow());
    try {
      const res = await pb.send<{ seen_at: string }>('/api/splitwiz/seen', { method: 'POST' });
      mark(res.seen_at);
    } catch {
      // offline: hidden here until the next reload brings the NAS's time back
    }
  }

  async login(email: string, password: string) {
    const res = await pb.collection('users').authWithPassword(email, password);
    this.me = res.record as unknown as User;
    await this.start();
  }

  logout() {
    this.stop();
    pb.authStore.clear();
    this.me = null;
    this.server = empty;
    this.outbox = [];
    saveOutbox([]);
    try {
      localStorage.removeItem(CACHE);
    } catch {
      // nothing cached
    }
    this.refresh();
  }

  async start() {
    await this.reload();
    syncPush();
    this.stop();
    try {
      for (const name of ['expenses', 'settlements', 'recurring_expenses', 'budget', 'expense_comments', 'categories', 'rules']) {
        this.unsubscribers.push(await pb.collection(name).subscribe('*', () => this.scheduleReload()));
      }
    } catch {
      // offline: PocketBase reconnects live updates by itself later
    }
  }

  stop() {
    this.unsubscribers.forEach((u) => u());
    this.unsubscribers = [];
  }

  scheduleReload() {
    clearTimeout(this.reloadTimer);
    this.reloadTimer = setTimeout(() => this.reload(), 150);
  }

  /** What the lists show: the NAS's data, plus queued changes, minus undoable deletions. */
  private refresh() {
    const s = this.server;
    const me = this.me?.id ?? '';
    const view = <T extends { id: string }>(c: Collection, items: T[]) =>
      overlay(items, this.outbox, c, me).filter((x) => !this.pending.has(x.id));
    this.users = s.users;
    this.categories = view('categories', s.categories).sort(byCategoryOrder);
    this.expenses = view('expenses', s.expenses);
    this.settlements = view('settlements', s.settlements);
    this.recurring = view('recurring_expenses', s.recurring);
    this.comments = view('expense_comments', s.comments);
    this.rules = view('rules', s.rules);
    this.budget = s.budget;
    this.summary = s.summary;
  }

  showToast(message: string, undo?: () => void) {
    const id = Date.now();
    this.toast = { id, message, undo };
    clearTimeout(this.toastTimer);
    this.toastTimer = setTimeout(() => {
      if (this.toast?.id === id) this.toast = null;
    }, UNDO_MS);
  }

  /**
   * Creates, changes or deletes a record. When the NAS can't be reached
   * the change is kept on the phone and sent later; the lists show it
   * either way. Returns the saved record's id (a `local-…` one if queued).
   */
  async write(collection: Collection, action: Op['action'], target: string | null, data: Record<string, unknown> = {}): Promise<string> {
    const op: Op = { collection, action, target: target ?? localId(), data, queued: pbNow() };
    if (!isLocal(op.target) || action === 'create') {
      try {
        const id = await this.send(op);
        this.scheduleReload();
        return id;
      } catch (err) {
        if (!unreachable(err)) throw err;
        this.showToast('Saved on this phone. It syncs once the NAS is reachable.');
      }
    }
    this.outbox = enqueue(this.outbox, op);
    saveOutbox(this.outbox);
    this.refresh();
    return op.target;
  }

  private async send(op: Op): Promise<string> {
    const c = pb.collection(op.collection);
    if (op.action === 'create') return (await c.create(op.data)).id;
    if (op.action === 'update') return (await c.update(op.target, op.data)).id;
    await c.delete(op.target);
    return op.target;
  }

  /** Sends the queued changes, oldest first, until one can't get through. */
  async sync() {
    if (this.syncing || !this.outbox.length) return;
    this.syncing = true;
    const failed: string[] = [];
    try {
      while (this.outbox.length) {
        const op = this.outbox[0];
        try {
          await this.send(op);
        } catch (err) {
          if (unreachable(err)) break;
          // gone already, or refused (e.g. the other person deleted it): drop it
          if ((err as { status?: number }).status !== 404) failed.push((err as Error).message);
        }
        this.outbox = this.outbox.slice(1);
        saveOutbox(this.outbox);
      }
    } finally {
      this.syncing = false;
    }
    if (failed.length) this.showToast(`${failed.length === 1 ? 'A change' : `${failed.length} changes`} made offline couldn’t be saved: ${failed[0]}`);
  }

  /** Deletes a record after a few seconds, unless Undo is tapped first. */
  remove(collection: Collection, id: string, message: string) {
    const timer = setTimeout(() => this.commitRemove(id), UNDO_MS);
    this.pending.set(id, { collection, timer });
    this.refresh();
    this.showToast(message, () => {
      clearTimeout(timer);
      this.pending.delete(id);
      this.refresh();
    });
  }

  private async commitRemove(id: string) {
    const p = this.pending.get(id);
    if (!p) return;
    try {
      await this.write(p.collection, 'delete', id);
    } catch (err) {
      if ((err as { status?: number }).status !== 404) this.showToast((err as Error).message || 'Could not delete.');
    } finally {
      this.pending.delete(id);
      if (!isLocal(id)) this.dropFromServer(p.collection, id);
      this.refresh();
    }
  }

  /** Sends undoable deletions right away, e.g. when the app is closed. */
  flushRemovals() {
    for (const [id, p] of this.pending) {
      clearTimeout(p.timer);
      this.commitRemove(id);
    }
    if (this.toast?.undo) this.toast = null;
  }

  private dropFromServer(collection: Collection, id: string) {
    const key = ({ expenses: 'expenses', settlements: 'settlements', recurring_expenses: 'recurring', expense_comments: 'comments', categories: 'categories', rules: 'rules' } as const)[collection];
    this.server = { ...this.server, [key]: (this.server[key] as { id: string }[]).filter((x) => x.id !== id) };
  }

  async reload() {
    if (!pb.authStore.isValid) return;
    this.loading = true;
    try {
      await this.sync();
      const [users, categories, expenses, settlements, recurring, budget, comments, rules, summary] = await Promise.all([
        pb.collection('users').getFullList<User>({ sort: 'created' }),
        pb.collection('categories').getFullList<Category>({ sort: 'sort,name' }),
        pb.collection('expenses').getFullList<Expense>({ sort: '-date,-created' }),
        pb.collection('settlements').getFullList<Settlement>({ sort: '-date,-created' }),
        pb.collection('recurring_expenses').getFullList<Recurring>({ sort: 'next_date' }),
        pb.collection('budget').getFullList<Budget>(),
        pb.collection('expense_comments').getFullList<Comment>({ sort: 'created' }),
        pb.collection('rules').getFullList<Rule>({ sort: 'match' }),
        pb.send<Summary>('/api/splitwiz/summary', {}),
      ]);
      this.server = { users, categories, expenses, settlements, recurring, budget: budget[0] ?? null, comments, rules, summary };
      saveCache(this.server);
      this.refresh();
      this.error = '';
    } catch (err) {
      const status = (err as { status?: number }).status;
      if (status === 401 || status === 403) {
        this.logout();
      } else {
        this.error = 'Could not reach the NAS. Showing the last data loaded.';
      }
    } finally {
      this.loading = false;
    }
  }
}

export const app = new AppState();

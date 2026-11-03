<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { formatCents } from '../lib/money';
  import { pb, type Expense, type Rule, type SplitMode } from '../lib/pb';
  import { guessMapping, lookAlike, parseCsv, readRows, type BankRow, type DateOrder, type Mapping } from '../lib/bankcsv';
  import { matchRule } from '../lib/rules';

  let table = $state<string[][] | null>(null);
  let mapping = $state<Mapping | null>(null);
  let paidBy = $state(app.me?.id ?? '');
  let picked = $state<Record<string, boolean>>({});
  let busy = $state(false);
  let error = $state('');
  let result = $state('');

  type Review = BankRow & { rule?: Rule; done: boolean; like?: Expense };

  const rows = $derived<Review[]>(
    table && mapping
      ? readRows(table, mapping).map((r) => {
          const done = app.expenses.some((e) => e.import_ref === r.ref);
          return { ...r, rule: matchRule(app.rules, r.description), done, like: done ? undefined : lookAlike(app.expenses, r) };
        })
      : [],
  );
  const fresh = $derived(rows.filter((r) => !r.done));
  const chosen = $derived(fresh.filter((r) => picked[r.ref]));
  const head = $derived(table && mapping ? (table[mapping.header] ?? []) : []);
  const width = $derived(table ? Math.max(0, ...table.slice(0, 20).map((r) => r.length)) : 0);
  const columnName = (i: number) => head[i] || `Column ${i + 1}`;

  // what a rule matched starts ticked, unless it looks added already
  function preselect() {
    picked = Object.fromEntries(fresh.map((r) => [r.ref, !!r.rule && !r.like]));
  }

  async function pick(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    table = mapping = null;
    error = result = '';
    if (!file) return;
    try {
      let text = await file.text();
      // some banks still export Windows-1252
      if (text.includes('�')) text = new TextDecoder('windows-1252').decode(await file.arrayBuffer());
      const t = parseCsv(text);
      if (t.length < 2) throw new Error('This file has no rows.');
      table = t;
      mapping = guessMapping(t);
      preselect();
    } catch (err) {
      error = (err as Error).message || 'Could not read the file.';
    } finally {
      // so picking the same file again reads it again
      input.value = '';
    }
  }

  function setColumn(key: 'date' | 'description' | 'amount' | 'credit', value: string) {
    if (!mapping) return;
    mapping = { ...mapping, [key]: Number(value) };
    preselect();
  }

  function setOrder(order: DateOrder) {
    if (!mapping) return;
    mapping = { ...mapping, order };
    preselect();
  }

  function split(r: Review): { split_mode: SplitMode; payer_percent: number } {
    const mode = r.rule?.split_mode || 'equal';
    return { split_mode: mode, payer_percent: mode === 'percent' ? (r.rule?.payer_percent ?? 50) : 0 };
  }

  async function run() {
    busy = true;
    error = '';
    try {
      const res = await pb.send<{ imported: number; already: number }>('/api/splitwiz/import/bank', {
        method: 'POST',
        body: {
          rows: chosen.map((r) => ({
            ref: r.ref,
            date: r.date,
            description: r.description,
            amount_cents: r.cents,
            paid_by: r.rule?.paid_by || paidBy,
            category: r.rule?.category ?? '',
            tags: r.rule?.tags ?? [],
            ...split(r),
          })),
        },
      });
      result = `Imported ${res.imported} ${res.imported === 1 ? 'expense' : 'expenses'}.`;
      table = mapping = null;
      await app.reload();
    } catch (err) {
      error = (err as Error).message || 'Import failed.';
    } finally {
      busy = false;
    }
  }

  const shortDate = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
  const orders: { id: DateOrder; label: string }[] = [
    { id: 'dmy', label: 'Day first' },
    { id: 'mdy', label: 'Month first' },
    { id: 'ymd', label: 'Year first' },
  ];
</script>

<section class="flex flex-col gap-3 rounded-2xl border border-slate-200 p-4 dark:border-slate-800">
  <h2 class="font-semibold">Import a bank statement</h2>
  <p class="text-sm text-slate-500">
    Export your account or card as CSV from your bank’s website, pick it here, and tick the payments you shared. Rules fill in
    the category and split. Importing the same file again skips what’s already in.
  </p>
  <input type="file" accept=".csv,text/csv,text/plain" onchange={pick} disabled={busy} />

  {#if table && mapping}
    <details class="rounded-xl bg-slate-100 px-3 py-2 text-sm dark:bg-slate-800/60" open={!rows.length}>
      <summary class="cursor-pointer font-medium">Columns{rows.length ? '' : ': no payments found, check these'}</summary>
      <div class="mt-2 grid grid-cols-[auto_1fr] items-center gap-2">
        {#each [['date', 'Date'], ['description', 'Description'], ['amount', 'Amount (or money out)'], ['credit', 'Money in']] as [key, label] (key)}
          <span class="text-slate-500">{label}</span>
          <select class="py-1.5" value={String(mapping[key as keyof Mapping])} onchange={(e) => setColumn(key as 'date', (e.target as HTMLSelectElement).value)}>
            {#if key === 'credit'}<option value="-1">None</option>{/if}
            {#each Array.from({ length: width }, (_, i) => i) as i (i)}
              <option value={String(i)}>{columnName(i)}</option>
            {/each}
          </select>
        {/each}
        <span class="text-slate-500">Dates</span>
        <select class="py-1.5" value={mapping.order} onchange={(e) => setOrder((e.target as HTMLSelectElement).value as DateOrder)}>
          {#each orders as o (o.id)}<option value={o.id}>{o.label}</option>{/each}
        </select>
      </div>
    </details>

    {#if rows.length}
      <div class="text-sm">
        <p class="mb-1 text-slate-500">Whose account is this?</p>
        <div class="seg" role="group" aria-label="Paid by">
          {#each app.users as u (u.id)}
            <button type="button" aria-pressed={paidBy === u.id} onclick={() => (paidBy = u.id)}>{app.nameOf(u.id)}</button>
          {/each}
        </div>
      </div>

      <div class="flex items-baseline justify-between text-sm">
        <span class="text-slate-500">
          {fresh.length} payments{rows.length > fresh.length ? ` · ${rows.length - fresh.length} imported before` : ''}
        </span>
        <span class="flex gap-3 font-medium text-brand-600">
          <button onclick={() => (picked = Object.fromEntries(fresh.map((r) => [r.ref, true])))}>All</button>
          <button onclick={() => (picked = {})}>None</button>
        </span>
      </div>

      <ul class="-mx-4 max-h-[60vh] divide-y divide-slate-100 overflow-y-auto border-y border-slate-100 dark:divide-slate-800 dark:border-slate-800">
        {#each fresh as r (r.ref)}
          <li>
            <label class="flex items-center gap-3 px-4 py-2.5">
              <input type="checkbox" class="h-5 w-5 shrink-0 accent-brand-600" bind:checked={picked[r.ref]} />
              <span class="w-6 shrink-0 text-center text-lg" aria-hidden="true">{app.category(r.rule?.category ?? '')?.emoji ?? '🧾'}</span>
              <span class="min-w-0 flex-1">
                <span class="block truncate text-sm font-medium">{r.description}</span>
                <span class="block truncate text-xs text-slate-500">
                  {shortDate(r.date)}{r.rule ? ` · ✨ ${r.rule.match}` : ''}{r.like ? ` · looks added already: ${r.like.description}` : ''}
                </span>
              </span>
              <span class="shrink-0 text-sm font-semibold">{formatCents(r.cents)}</span>
            </label>
          </li>
        {/each}
      </ul>
      {#if fresh.length}
        <p class="text-xs text-slate-500">Payments without a rule are split 50/50 with no category; you can change them afterwards.</p>
        <button class="btn-primary" onclick={run} disabled={busy || !chosen.length}>
          {busy ? 'Importing…' : `Import ${chosen.length} ${chosen.length === 1 ? 'payment' : 'payments'} · ${formatCents(chosen.reduce((s, r) => s + r.cents, 0))}`}
        </button>
      {:else}
        <p class="text-sm text-slate-500">Everything in this file is already in Splitwiz.</p>
      {/if}
    {/if}
  {/if}

  {#if result}
    <p class="rounded-xl bg-brand-50 px-3 py-2 text-sm text-brand-700 dark:bg-brand-700/20 dark:text-brand-100">{result}</p>
  {/if}
  {#if error}<p class="text-sm text-red-600">{error}</p>{/if}
</section>

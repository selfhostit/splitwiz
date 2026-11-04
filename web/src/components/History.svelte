<script lang="ts">
  import { app, type Entry } from '../lib/state.svelte';
  import { formatCents, parseAmount } from '../lib/money';
  import type { Expense, Settlement } from '../lib/pb';
  import { activeFilters, filterEntries, noFilters, toCsv, type Filters } from '../lib/filters';
  import { allTags, tagsOf, tagSummary } from '../lib/tags';
  import EntryRow from './EntryRow.svelte';

  let { editExpense, editSettlement }: { editExpense: (e: Expense) => void; editSettlement: (s: Settlement) => void } =
    $props();

  let query = $state('');
  let filters = $state<Filters>({ ...noFilters });
  let minText = $state('');
  let maxText = $state('');
  let showFilters = $state(false);
  const PAGE = 200;
  let shown = $state(PAGE);

  $effect(() => {
    filters.min = minText.trim() ? parseAmount(minText) : null;
    filters.max = maxText.trim() ? parseAmount(maxText) : null;
  });

  const active = $derived(activeFilters(filters));
  const narrowed = $derived(!!query.trim() || active > 0);

  const text = (e: Entry) => {
    if (e.kind === 'settlement') return ['settlement payment', e.item.note];
    const cat = app.category(e.item.category)?.name ?? '';
    return [e.item.description, e.item.note, cat, ...tagsOf(e.item).map((t) => '#' + t), ...app.commentsOf(e.item.id).map((c) => c.text)];
  };

  const matching = $derived(narrowed ? filterEntries(app.entries, query, filters, text) : app.entries);
  const filtered = $derived(narrowed ? matching : matching.slice(0, shown));
  const tags = $derived(allTags(app.expenses));
  // a tag's own totals, like a trip's bill
  const trip = $derived(filters.tag ? tagSummary(app.expenses, filters.tag, app.me?.id ?? '') : null);
  const span = (from: string, to: string) => {
    const f = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
    return from === to ? f(from) : `${f(from)} – ${f(to)}`;
  };

  const total = $derived(matching.reduce((s, e) => s + (e.kind === 'expense' ? e.item.amount_cents : 0), 0));

  const months = $derived.by(() => {
    const groups: { key: string; label: string; total: number; entries: Entry[] }[] = [];
    for (const e of filtered) {
      const key = e.date.slice(0, 7);
      let g = groups.at(-1);
      if (!g || g.key !== key) {
        const label = new Date(key + '-15T12:00:00').toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
        g = { key, label, total: 0, entries: [] };
        groups.push(g);
      }
      g.entries.push(e);
      if (e.kind === 'expense') g.total += e.item.amount_cents;
    }
    return groups;
  });

  function clear() {
    filters = { ...noFilters };
    minText = '';
    maxText = '';
  }

  function exportMatching() {
    const name = (id: string) => app.users.find((u) => u.id === id)?.name || app.nameOf(id);
    const csv = toCsv(
      matching,
      name,
      (id) => app.category(id)?.name ?? '',
      (id) => app.commentsOf(id).map((c) => `${name(c.author)}: ${c.text}`),
    );
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
    a.download = `splitwiz-filtered-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  const kinds: { id: Filters['kind']; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'expense', label: 'Expenses' },
    { id: 'settlement', label: 'Payments' },
  ];
</script>

<header class="sticky top-0 z-10 bg-slate-50/95 px-4 pt-6 pb-3 backdrop-blur dark:bg-slate-950/95">
  <h1 class="mb-3 px-1 text-2xl font-bold">History</h1>
  <div class="flex gap-2">
    <input class="min-w-0 flex-1" type="search" placeholder="Search expenses" bind:value={query} />
    <button class="chip" aria-pressed={showFilters || active > 0} aria-expanded={showFilters} onclick={() => (showFilters = !showFilters)}>
      Filters{active ? ` · ${active}` : ''}
    </button>
  </div>

  {#if showFilters}
    <div class="mt-3 flex flex-col gap-3">
      <div class="flex gap-2" role="group" aria-label="Type">
        {#each kinds as k (k.id)}
          <button class="chip" aria-pressed={filters.kind === k.id} onclick={() => (filters.kind = k.id)}>{k.label}</button>
        {/each}
      </div>
      <div class="flex gap-2" role="group" aria-label="Paid by">
        <button class="chip" aria-pressed={!filters.paidBy} onclick={() => (filters.paidBy = '')}>Anyone paid</button>
        {#each app.users as u (u.id)}
          <button class="chip" aria-pressed={filters.paidBy === u.id} onclick={() => (filters.paidBy = u.id)}>{app.nameOf(u.id)} paid</button>
        {/each}
      </div>
      {#if filters.kind !== 'settlement'}
        <div class="-mx-4 flex gap-2 overflow-x-auto px-4 [scrollbar-width:none]" role="group" aria-label="Category">
          <button class="chip" aria-pressed={!filters.category} onclick={() => (filters.category = '')}>Any category</button>
          {#each app.categories as c (c.id)}
            <button class="chip" aria-pressed={filters.category === c.id} onclick={() => (filters.category = c.id)}>{c.emoji} {c.name}</button>
          {/each}
        </div>
      {/if}
      {#if filters.kind !== 'settlement' && tags.length}
        <div class="-mx-4 flex gap-2 overflow-x-auto px-4 [scrollbar-width:none]" role="group" aria-label="Tag">
          <button class="chip" aria-pressed={!filters.tag} onclick={() => (filters.tag = '')}>Any tag</button>
          {#each tags as t (t)}
            <button class="chip" aria-pressed={filters.tag === t} onclick={() => (filters.tag = t)}>#{t}</button>
          {/each}
        </div>
      {/if}
      <div class="flex items-center gap-2 text-sm">
        <span class="text-slate-500">Amount €</span>
        <input class="min-w-0 flex-1 py-2" inputmode="decimal" placeholder="from" aria-label="Minimum amount" bind:value={minText} />
        <span class="text-slate-500">to</span>
        <input class="min-w-0 flex-1 py-2" inputmode="decimal" placeholder="any" aria-label="Maximum amount" bind:value={maxText} />
      </div>
      {#if active}
        <button class="self-start px-1 text-sm font-medium text-brand-600" onclick={clear}>Clear filters</button>
      {/if}
    </div>
  {/if}

  {#if trip && trip.count}
    <div class="mt-3 rounded-2xl bg-brand-50 p-4 dark:bg-brand-700/25">
      <div class="flex items-baseline justify-between gap-3">
        <h2 class="truncate font-semibold">#{filters.tag}</h2>
        <span class="text-lg font-bold">{formatCents(trip.total)}</span>
      </div>
      <p class="text-xs text-slate-500">{trip.count} {trip.count === 1 ? 'expense' : 'expenses'} · {span(trip.from, trip.to)}</p>
      <ul class="mt-2 flex flex-col gap-0.5 text-sm">
        {#each app.users as u (u.id)}
          <li class="flex justify-between"><span>{app.nameOf(u.id)} paid</span><span>{formatCents(trip.paid[u.id] ?? 0)}</span></li>
        {/each}
      </ul>
      <p class="mt-2 text-sm font-medium text-brand-700 dark:text-brand-100">
        {trip.net === 0
          ? 'Even for this tag.'
          : trip.net > 0
            ? `For this tag, ${app.nameOf(app.other?.id ?? '')} owes you ${formatCents(trip.net)}.`
            : `For this tag, you owe ${app.nameOf(app.other?.id ?? '')} ${formatCents(-trip.net)}.`}
      </p>
    </div>
  {/if}

  {#if narrowed && matching.length}
    <div class="mt-3 flex items-center justify-between px-1 text-sm">
      <span class="text-slate-500">{matching.length} found · {formatCents(total)} spent</span>
      <button class="font-medium text-brand-600" onclick={exportMatching}>Export these (CSV)</button>
    </div>
  {/if}
</header>

{#if months.length === 0}
  <p class="px-5 py-10 text-center text-slate-500">{narrowed ? 'Nothing matches.' : 'No expenses yet.'}</p>
{/if}

{#each months as m (m.key)}
  <section class="mb-2">
    <div class="flex items-baseline justify-between px-5 pt-4 pb-1 text-sm">
      <h2 class="font-semibold text-slate-600 dark:text-slate-300">{m.label}</h2>
      <span class="text-slate-500">{formatCents(m.total)} spent</span>
    </div>
    <div class="divide-y divide-slate-100 dark:divide-slate-800">
      {#each m.entries as entry (entry.item.id)}
        <EntryRow
          {entry}
          onclick={() => (entry.kind === 'expense' ? editExpense(entry.item) : editSettlement(entry.item))}
        />
      {/each}
    </div>
  </section>
{/each}

{#if !narrowed && app.entries.length > shown}
  <div class="px-4 py-4">
    <button class="btn-secondary w-full" onclick={() => (shown += PAGE)}>Show older</button>
  </div>
{/if}

{#if app.entries.length && !narrowed}
  <p class="px-5 pb-4 text-center text-xs text-slate-400">Swipe a row left to edit or delete it.</p>
{/if}

<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { formatCents } from '../lib/money';
  import { dayOf, today, type Expense } from '../lib/pb';
  import { byCategory, byPeriod, inRange, monthsIn, rangeBounds, ranges, type RangeId } from '../lib/stats';
  import PeriodBars from './PeriodBars.svelte';
  import MonthBudget from './MonthBudget.svelte';
  import CategoryLimits from './CategoryLimits.svelte';

  let { editExpense }: { editExpense: (e: Expense) => void } = $props();

  let range = $state<RangeId>('12-months');
  // category id being drilled into; null = all categories
  let focus = $state<string | null>(null);

  const bounds = $derived(rangeBounds(range, today(), app.expenses));
  const list = $derived(inRange(app.expenses, bounds.from, bounds.to));
  const focused = $derived(focus === null ? list : list.filter((e) => e.category === focus));

  const total = $derived(list.reduce((s, e) => s + e.amount_cents, 0));
  const perMonth = $derived(Math.round(total / monthsIn(bounds.from, bounds.to)));
  // you first, then your partner, so the bars read the same way everywhere
  const people = $derived([...app.users].sort((a, b) => Number(b.id === app.me?.id) - Number(a.id === app.me?.id)));
  const colors = ['bg-brand-600', 'bg-sky-500'];
  const categories = $derived(byCategory(list));
  const periods = $derived(byPeriod(focused, bounds.from, bounds.to));
  const biggest = $derived([...focused].sort((a, b) => b.amount_cents - a.amount_cents).slice(0, 5));

  const catName = (id: string) => app.category(id)?.name ?? 'No category';
  const catEmoji = (id: string) => app.category(id)?.emoji ?? '🧾';

  const shortDate = (d: string) => new Date(dayOf(d) + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });

  function pickRange(id: RangeId) {
    range = id;
  }

  function toggleFocus(id: string) {
    focus = focus === id ? null : id;
  }
</script>

<header class="px-5 pt-6 pb-2">
  <h1 class="text-2xl font-bold">Stats</h1>
</header>

<MonthBudget />
<CategoryLimits />

<h2 class="px-5 pt-6 pb-2 font-semibold">Over time</h2>

<div class="flex gap-2 overflow-x-auto px-4 pb-2 [scrollbar-width:none]" role="group" aria-label="Time range">
  {#each ranges as r (r.id)}
    <button class="chip" aria-pressed={range === r.id} onclick={() => pickRange(r.id)}>{r.label}</button>
  {/each}
</div>

{#if list.length === 0}
  <p class="px-5 py-12 text-center text-slate-500">No expenses in this period.</p>
{:else}
  <section class="mx-4 mt-2 rounded-3xl bg-white p-5 shadow-sm dark:bg-slate-900">
    <p class="text-sm text-slate-500">Spent together</p>
    <p class="mt-1 text-5xl font-bold tracking-tight">{formatCents(total)}</p>
    <p class="mt-1 text-sm text-slate-500">
      {list.length} expenses · {formatCents(perMonth)} a month on average
    </p>
  </section>

  <section class="mx-4 mt-4 rounded-3xl bg-white p-5 shadow-sm dark:bg-slate-900">
    {#key `${range}|${focus}`}
      <PeriodBars
        {periods}
        average
        title={`${periods.length && periods[0].key.length === 4 ? 'Per year' : 'Per month'}${focus !== null ? ` · ${catName(focus)}` : ''}`}
      />
    {/key}
  </section>

  <section class="mx-4 mt-4 rounded-3xl bg-white p-5 shadow-sm dark:bg-slate-900">
    <div class="flex items-baseline justify-between">
      <h2 class="font-semibold">By category</h2>
      {#if focus !== null}
        <button class="text-sm font-medium text-brand-600" onclick={() => toggleFocus(focus!)}>Show all</button>
      {:else}
        <span class="text-xs text-slate-500">Tap one to dig in</span>
      {/if}
    </div>
    <p class="mt-1 flex gap-3 text-xs text-slate-500">
      {#each people as u, i (u.id)}
        <span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full {colors[i]}"></span>{app.nameOf(u.id)} paid</span>
      {/each}
    </p>
    <ul class="mt-3 flex flex-col gap-1">
      {#each categories as c (c.category)}
        <li>
          <button
            class="w-full rounded-xl px-2 py-2 text-left {focus === c.category ? 'bg-brand-50 dark:bg-brand-700/20' : ''} {focus !== null && focus !== c.category ? 'opacity-50' : ''}"
            aria-pressed={focus === c.category}
            onclick={() => toggleFocus(c.category)}
          >
            <span class="flex items-baseline justify-between gap-2 text-sm">
              <span class="min-w-0 truncate">{catEmoji(c.category)} {catName(c.category)}</span>
              <span class="shrink-0 tabular-nums">
                <b>{formatCents(c.cents)}</b>
                <span class="text-slate-500">· {Math.round(c.share * 100)}%</span>
              </span>
            </span>
            <span class="mt-1.5 block h-2 w-full rounded-full bg-slate-100 dark:bg-slate-800">
              <span class="flex h-2 gap-px overflow-hidden rounded-full" style="width: {Math.max(1, (c.cents / categories[0].cents) * 100)}%">
                {#each people as u, i (u.id)}
                  {#if c.paid[u.id]}
                    <span class="block h-2 {colors[i]}" style="width: {(c.paid[u.id] / c.cents) * 100}%"></span>
                  {/if}
                {/each}
              </span>
            </span>
            <span class="mt-1 block truncate text-xs text-slate-500">
              {people.map((u) => `${app.nameOf(u.id)} ${formatCents(c.paid[u.id] ?? 0)}`).join(' · ')}
            </span>
          </button>
        </li>
      {/each}
    </ul>
  </section>

  <section class="mt-4 mb-4">
    <h2 class="px-5 pb-1 font-semibold">Biggest expenses{focus !== null ? ` · ${catName(focus)}` : ''}</h2>
    <div class="divide-y divide-slate-100 dark:divide-slate-800">
      {#each biggest as e (e.id)}
        <button class="flex w-full items-center gap-3 px-4 py-3 text-left active:bg-slate-100 dark:active:bg-slate-800" onclick={() => editExpense(e)}>
          <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-slate-100 text-lg dark:bg-slate-800">{catEmoji(e.category)}</span>
          <span class="min-w-0 flex-1">
            <span class="block truncate font-medium">{e.description}</span>
            <span class="block text-sm text-slate-500">{app.nameOf(e.paid_by)} paid · {shortDate(e.date)}</span>
          </span>
          <span class="shrink-0 font-semibold">{formatCents(e.amount_cents)}</span>
        </button>
      {/each}
    </div>
  </section>
{/if}

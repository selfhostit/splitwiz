<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { formatCents } from '../lib/money';
  import { today, type Recurring } from '../lib/pb';
  import { hasEnded, upcoming } from '../lib/recurring';

  let { editRecurring }: { editRecurring: (r: Recurring) => void } = $props();

  const windows = [
    { days: 30, label: '30 days' },
    { days: 90, label: '3 months' },
  ];
  let days = $state(30);
  // a long window lists every row only when asked
  let expanded = $state(false);
  const FEW = 3;

  const addDays = (day: string, n: number) => {
    const d = new Date(day + 'T12:00:00Z');
    d.setUTCDate(d.getUTCDate() + n);
    return d.toISOString().slice(0, 10);
  };

  const list = $derived(upcoming(app.recurring, today(), addDays(today(), days)));
  const total = $derived(list.reduce((s, o) => s + o.amount_cents, 0));
  const shown = $derived(expanded ? list : list.slice(0, FEW));

  function when(day: string): string {
    const t = today();
    if (day === t) return 'Today';
    if (day === addDays(t, 1)) return 'Tomorrow';
    return new Date(day + 'T12:00:00').toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' });
  }

  const pays = (id: string) => (id === app.me?.id ? 'You pay' : `${app.nameOf(id)} pays`);
</script>

{#if app.recurring.some((r) => !r.paused && !hasEnded(r))}
  <section class="mt-6">
    <div class="flex items-center justify-between gap-2 px-5 pb-1">
      <h2 class="font-semibold">Coming up</h2>
      <div class="flex gap-1.5" role="group" aria-label="How far ahead">
        {#each windows as w (w.days)}
          <button class="chip px-3 py-1 text-xs" aria-pressed={days === w.days} onclick={() => ((days = w.days), (expanded = false))}>
            {w.label}
          </button>
        {/each}
      </div>
    </div>
    {#if list.length === 0}
      <p class="px-5 py-4 text-sm text-slate-500">No repeating expenses due in the next {days === 30 ? '30 days' : '3 months'}.</p>
    {:else}
      <p class="px-5 text-sm text-slate-500">{list.length} repeating {list.length === 1 ? 'expense' : 'expenses'} · {formatCents(total)} in total</p>
      <div class="mt-1 divide-y divide-slate-100 dark:divide-slate-800">
        {#each shown as o (o.series.id + o.day)}
          <button
            class="flex w-full items-center gap-3 px-4 py-3 text-left active:bg-slate-100 dark:active:bg-slate-800"
            onclick={() => editRecurring(o.series)}
          >
            <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border border-dashed border-slate-300 text-xl dark:border-slate-700">
              {app.category(o.series.category)?.emoji ?? '🔁'}
            </span>
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">{o.series.description}</span>
              <span class="block truncate text-sm text-slate-500">{when(o.day)} · {pays(o.series.paid_by)}</span>
            </span>
            <span class="flex shrink-0 flex-col items-end">
              <span class="font-semibold text-slate-500">{formatCents(o.amount_cents)}</span>
              {#if o.changed}
                <span class="mt-0.5 rounded-full bg-amber-100 px-2 text-[11px] font-medium text-amber-800 dark:bg-amber-500/20 dark:text-amber-200">New amount</span>
              {:else if o.last}
                <span class="mt-0.5 rounded-full bg-slate-100 px-2 text-[11px] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">🏁 Last one</span>
              {/if}
            </span>
          </button>
        {/each}
      </div>
      {#if list.length > FEW}
        <button class="w-full px-5 py-2 text-left text-sm font-medium text-brand-600" onclick={() => (expanded = !expanded)}>
          {expanded ? 'Show fewer' : `Show all ${list.length}`}
        </button>
      {/if}
    {/if}
  </section>
{/if}

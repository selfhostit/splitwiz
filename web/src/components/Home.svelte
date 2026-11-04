<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { formatCents } from '../lib/money';
  import type { Expense, Recurring, Settlement } from '../lib/pb';
  import { describeUnseen } from '../lib/unseen';
  import EntryRow from './EntryRow.svelte';
  import Upcoming from './Upcoming.svelte';

  let {
    addExpense,
    editExpense,
    settle,
    editSettlement,
    showHistory,
    editRecurring,
  }: {
    addExpense: () => void;
    editExpense: (e: Expense) => void;
    settle: () => void;
    editSettlement: (s: Settlement) => void;
    showHistory: () => void;
    editRecurring: (r: Recurring) => void;
  } = $props();

  const balance = $derived(app.summary?.balance);
  const settled = $derived(!balance || balance.amount_cents === 0);
  const iOwe = $derived(balance?.debtor === app.me?.id);
  const recent = $derived(app.entries.slice(0, 15));
  const news = $derived(describeUnseen(app.nameOf(app.other?.id ?? ''), app.unseen));
</script>

<header class="px-5 pt-6 pb-2">
  <p class="text-sm text-slate-500">Hi {app.me?.name || 'there'}</p>
  <h1 class="text-2xl font-bold">Splitwiz</h1>
</header>

<section class="mx-4 mt-2 rounded-3xl p-6 text-center {settled
    ? 'bg-slate-100 dark:bg-slate-900'
    : iOwe
      ? 'bg-orange-50 dark:bg-orange-950/40'
      : 'bg-brand-50 dark:bg-brand-700/20'}">
  {#if !app.summary}
    <p class="text-slate-500">Loading…</p>
  {:else if settled}
    <p class="text-4xl">🎉</p>
    <p class="mt-2 text-xl font-semibold">All settled</p>
    <p class="text-sm text-slate-500">Nobody owes anything.</p>
  {:else}
    <p class="text-sm text-slate-600 dark:text-slate-300">
      {iOwe ? `You owe ${app.nameOf(balance!.creditor)}` : `${app.nameOf(balance!.debtor)} owes you`}
    </p>
    <p class="mt-1 text-5xl font-bold tracking-tight {iOwe ? 'text-orange-600' : 'text-brand-600'}">
      {formatCents(balance!.amount_cents)}
    </p>
  {/if}
</section>

<div class="mx-4 mt-4 grid grid-cols-[2fr_1fr] gap-3">
  <button class="btn-primary text-lg" onclick={addExpense}>
    <span class="text-2xl leading-none">+</span> Add expense
  </button>
  <button class="btn-secondary" onclick={settle} disabled={settled}>Settle up</button>
</div>

{#if news}
  <div class="mx-4 mt-4 flex items-center gap-3 rounded-2xl border border-brand-600/30 bg-brand-50 px-4 py-3 dark:bg-brand-700/25">
    <span class="h-2.5 w-2.5 shrink-0 rounded-full bg-brand-500" aria-hidden="true"></span>
    <p class="min-w-0 flex-1 text-sm">{news}.</p>
    <button class="shrink-0 text-sm font-semibold text-brand-700 dark:text-brand-100" onclick={() => app.markSeen()}>Mark as seen</button>
  </div>
{/if}

<Upcoming {editRecurring} />

<section class="mt-6">
  <div class="flex items-baseline justify-between px-5 pb-1">
    <h2 class="font-semibold">Recent</h2>
    {#if app.entries.length > recent.length}
      <button class="text-sm font-medium text-brand-600" onclick={showHistory}>See all</button>
    {/if}
  </div>
  {#if recent.length === 0}
    <p class="px-5 py-8 text-center text-slate-500">No expenses yet. Tap “Add expense” to start.</p>
  {:else}
    <div class="divide-y divide-slate-100 dark:divide-slate-800">
      {#each recent as entry (entry.item.id)}
        <EntryRow
          {entry}
          onclick={() => (entry.kind === 'expense' ? editExpense(entry.item) : editSettlement(entry.item))}
        />
      {/each}
    </div>
  {/if}
</section>

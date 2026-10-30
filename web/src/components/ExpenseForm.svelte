<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { app } from '../lib/state.svelte';
  import { formatCents, owedCents, parseAmount } from '../lib/money';
  import { dayOf, toPbDate, today, type Every, type Expense, type Recurring, type SplitMode } from '../lib/pb';
  import { hasEnded, nextDate, sameCharge } from '../lib/recurring';
  import { startSeries } from '../lib/series';
  import { favorites, lastLike } from '../lib/quickadd';
  import { isLocal } from '../lib/outbox';
  import { byPeriod, rangeBounds } from '../lib/stats';
  import PeriodBars from './PeriodBars.svelte';
  import Comments from './Comments.svelte';
  import RepeatPlan, { type ChangeRow } from './RepeatPlan.svelte';

  let {
    expense = null,
    template = null,
    onclose,
  }: { expense?: Expense | null; template?: Recurring | null; onclose: () => void } = $props();

  // the form edits a copy; later prop changes don't reset what was typed
  const initialTemplate = untrack(() => template);
  const initial = untrack(() => expense) ?? initialTemplate;
  const editingTemplate = !!initialTemplate;
  const series = untrack(() => app.recurringOf(expense?.recurring ?? ''));

  const centsText = (c: number) => (c / 100).toFixed(2);

  let amountText = $state(initial ? centsText(initial.amount_cents) : '');
  let description = $state(initial?.description ?? '');
  let paidBy = $state(initial?.paid_by ?? app.me?.id ?? '');
  let mode = $state<SplitMode>(initial?.split_mode ?? 'equal');
  let payerPercent = $state(initial?.split_mode === 'percent' ? initial.payer_percent : 50);
  let otherShareText = $state(initial?.split_mode === 'exact' ? centsText(initial.other_share_cents) : '');
  let category = $state(initial?.category ?? '');
  let day = $state(initialTemplate ? dayOf(initialTemplate.next_date) : initial ? dayOf((initial as Expense).date) : today());
  let every = $state<Every | ''>(initialTemplate?.every ?? '');
  let note = $state(initial?.note ?? '');
  // when the series stops and how its amount changes over time
  let ends = $state(initialTemplate?.ends_on ? dayOf(initialTemplate.ends_on) : '');
  let changeRows = $state<ChangeRow[]>(
    (initialTemplate?.changes ?? []).map((c, i) => ({ key: i + 1, from: c.from, amountText: centsText(c.amount_cents) })),
  );
  let planValid = $state(true);
  let busy = $state(false);
  let error = $state('');

  let amountInput: HTMLInputElement | undefined = $state();
  onMount(() => {
    if (!initial) amountInput?.focus();
  });

  const amount = $derived(parseAmount(amountText));
  const otherShare = $derived(mode === 'exact' ? (parseAmount(otherShareText) ?? (otherShareText.trim() === '0' ? 0 : NaN)) : 0);
  const owed = $derived(amount ? owedCents(amount, mode, payerPercent, otherShare) : null);
  const payerName = $derived(app.nameOf(paidBy));
  const otherId = $derived(app.users.find((u) => u.id !== paidBy)?.id ?? '');
  const otherName = $derived(app.nameOf(otherId));
  const valid = $derived(amount !== null && owed !== null && description.trim() !== '' && !!paidBy && (!every || planValid));
  // the series' first expense still to come
  const planStart = $derived(every ? (expense ? nextDate(day, every) : day) : '');
  // a finished series can still be edited, and restarted with a later end
  const ended = !!initialTemplate && hasEnded(initialTemplate);

  const repeats: { id: Every | ''; label: string }[] = [
    { id: '', label: 'Never' },
    { id: 'week', label: 'Weekly' },
    { id: 'month', label: 'Monthly' },
    { id: 'year', label: 'Yearly' },
  ];

  // opening an existing expense shows its category's last three months
  const recent = $derived.by(() => {
    if (!expense || !category) return null;
    const { from, to } = rangeBounds('3-months', today(), []);
    const periods = byPeriod(app.expenses.filter((e) => e.category === category), from, to);
    const at = periods.findIndex((p) => p.key === day.slice(0, 7));
    return { periods, highlight: at >= 0 ? at : undefined };
  });

  const suggestions = $derived([...new Set(app.expenses.map((e) => e.description))].slice(0, 30));

  const modes: { id: SplitMode; label: () => string }[] = [
    { id: 'equal', label: () => '50 / 50' },
    { id: 'percent', label: () => '%' },
    { id: 'exact', label: () => 'Exact' },
    { id: 'full', label: () => `${otherName} owes all` },
  ];

  const adding = !initial;
  const favs = $derived(adding ? favorites(app.expenses, today()) : []);

  // copies a past expense's details; the amount and category only when
  // `all`, or when they're still empty
  function fill(e: Expense, all: boolean) {
    if (all || !amountText.trim()) amountText = centsText(e.amount_cents);
    if (all || !category) category = e.category;
    paidBy = e.paid_by;
    mode = e.split_mode;
    if (mode === 'percent') payerPercent = e.payer_percent;
    if (mode === 'exact') otherShareText = centsText(e.other_share_cents);
  }

  function pick(e: Expense) {
    description = e.description;
    fill(e, true);
  }

  // picking a past description pre-fills how it was paid and split last time
  function onDescriptionChange() {
    const past = lastLike(app.expenses, description);
    if (!past) return;
    if (adding) fill(past, false);
    else if (!category) category = past.category;
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!valid) return;
    busy = true;
    error = '';
    const data = {
      description: description.trim(),
      amount_cents: amount,
      paid_by: paidBy,
      split_mode: mode,
      payer_percent: mode === 'percent' ? payerPercent : 0,
      other_share_cents: mode === 'exact' ? otherShare : 0,
      category: category || null,
      // an existing expense keeps its comments instead
      ...(expense ? {} : { note: note.trim() }),
    };
    const plan = {
      ends_on: ends ? toPbDate(ends) : '',
      changes: changeRows
        .map((r) => ({ from: r.from, amount_cents: parseAmount(r.amountText) ?? 0 }))
        .filter((c) => c.from > planStart && c.amount_cents > 0 && (!ends || c.from <= ends))
        .sort((a, b) => a.from.localeCompare(b.from)),
    };
    try {
      if (template) {
        await app.write('recurring_expenses', 'update', template.id, { ...data, ...plan, every, next_date: toPbDate(day) });
      } else if (expense) {
        await app.write('expenses', 'update', expense.id, { ...data, date: toPbDate(day) });
        if (every) {
          const saved = { ...expense, ...data, category: data.category ?? '', date: toPbDate(day) } as Expense;
          // earlier copies of the same charge become part of the series too
          const copies = app.expenses.filter((x) => x.id !== saved.id && !x.recurring && sameCharge(x, saved));
          await startSeries(saved, every, [saved, ...copies], plan);
          app.reload();
        }
      } else if (every) {
        // the server adds the first expense when its date has come
        await app.write('recurring_expenses', 'create', null, { ...data, ...plan, every, next_date: toPbDate(day) });
      } else {
        await app.write('expenses', 'create', null, { ...data, date: toPbDate(day) });
      }
      onclose();
    } catch (err) {
      error = (err as { message?: string }).message ?? 'Could not save.';
    } finally {
      busy = false;
    }
  }

  function remove() {
    if (template) app.remove('recurring_expenses', template.id, `Stopped repeating “${template.description}”. Expenses already added stay.`);
    else if (expense) app.remove('expenses', expense.id, `Deleted “${expense.description}”`);
    onclose();
  }
</script>

<form class="flex flex-col gap-4" onsubmit={save}>
  {#if recent}
    <div class="rounded-2xl bg-slate-100 p-4 dark:bg-slate-800/60">
      <PeriodBars
        periods={recent.periods}
        highlight={recent.highlight}
        height={110}
        title={`${app.category(category)?.emoji ?? ''} ${app.category(category)?.name ?? ''}`}
      />
      <p class="mt-1 text-xs text-slate-500">Spent on this category in the last 3 months</p>
    </div>
  {/if}

  <div class="flex items-center gap-2 rounded-2xl bg-slate-100 px-4 dark:bg-slate-800">
    <span class="text-3xl font-semibold text-slate-400">€</span>
    <input
      bind:this={amountInput}
      class="min-w-0 flex-1 border-0 bg-transparent px-0 py-4 text-4xl font-bold focus:ring-0 dark:bg-transparent"
      inputmode="decimal"
      placeholder="0.00"
      aria-label="Amount"
      bind:value={amountText}
    />
  </div>

  {#if favs.length && !description}
    <div class="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1 [scrollbar-width:none]" role="group" aria-label="Quick add">
      {#each favs as f (f.id)}
        <button type="button" class="chip" onclick={() => pick(f)}>
          {app.category(f.category)?.emoji ?? '🧾'} {f.description} · {formatCents(f.amount_cents)}
        </button>
      {/each}
    </div>
  {/if}

  <input
    list="desc-suggestions"
    placeholder="What was it for?"
    aria-label="Description"
    maxlength="200"
    bind:value={description}
    oninput={onDescriptionChange}
  />
  <datalist id="desc-suggestions">
    {#each suggestions as s (s)}<option value={s}></option>{/each}
  </datalist>

  <div>
    <p class="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">Paid by</p>
    <div class="grid grid-cols-2 gap-2">
      {#each app.users as u (u.id)}
        <button type="button" class="chip py-3 text-base" aria-pressed={paidBy === u.id} onclick={() => (paidBy = u.id)}>
          {app.nameOf(u.id)}
        </button>
      {/each}
    </div>
  </div>

  <div>
    <p class="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">Split</p>
    <div class="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1">
      {#each modes as m (m.id)}
        <button type="button" class="chip" aria-pressed={mode === m.id} onclick={() => (mode = m.id)}>{m.label()}</button>
      {/each}
    </div>

    {#if mode === 'percent'}
      <div class="mt-3">
        <input class="w-full accent-brand-600 border-0 px-0" type="range" min="0" max="100" step="5" bind:value={payerPercent} aria-label="Payer share" />
        <div class="flex justify-between text-sm">
          <span>{payerName}: <b>{payerPercent}%</b></span>
          <span>{otherName}: <b>{100 - payerPercent}%</b></span>
        </div>
      </div>
    {:else if mode === 'exact'}
      <div class="mt-3 flex items-center gap-2">
        <span class="text-sm whitespace-nowrap">{otherName}’s share €</span>
        <input class="min-w-0 flex-1" inputmode="decimal" placeholder="0.00" bind:value={otherShareText} />
      </div>
    {/if}

    {#if amount && owed !== null}
      <p class="mt-3 rounded-xl bg-brand-50 px-3 py-2 text-sm text-brand-700 dark:bg-brand-700/20 dark:text-brand-100">
        {owed === 0 ? 'Nothing is shared.' : `${otherName} ${otherName === 'You' ? 'owe' : 'owes'} ${payerName === 'You' ? 'you' : payerName} ${formatCents(owed)}`}
      </p>
    {:else if amount && mode === 'exact' && otherShareText}
      <p class="mt-3 text-sm text-red-600">The share can’t be more than the amount.</p>
    {/if}
  </div>

  <div>
    <p class="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">Category</p>
    <div class="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1">
      {#each app.categories as c (c.id)}
        <button type="button" class="chip" aria-pressed={category === c.id} onclick={() => (category = c.id)}>
          {c.emoji} {c.name}
        </button>
      {/each}
    </div>
  </div>

  {#if !series}
    <div>
      <p class="mb-2 text-sm font-medium text-slate-600 dark:text-slate-300">Repeat</p>
      <div class="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1">
        {#each repeats as r (r.id)}
          {#if r.id || !editingTemplate}
            <button type="button" class="chip" aria-pressed={every === r.id} onclick={() => (every = r.id)}>{r.label}</button>
          {/if}
        {/each}
      </div>
      {#if every}
        <p class="mt-2 text-sm text-slate-500">
          {expense
            ? `Next one added on ${nextDate(day, every)}, then`
            : day > today()
              ? `First added on ${day}`
              : editingTemplate
                ? `Next added on ${day}`
                : 'Added today, then'}
          {every === 'week' ? 'every week' : every === 'month' ? 'every month on the same day' : 'every year'}{ends && ends >= planStart ? ', until the end date below' : ''}.
        </p>
        <RepeatPlan
          {every}
          start={planStart}
          anchorDay={initialTemplate?.anchor_day ?? 0}
          {amount}
          allowEnded={ended}
          bind:ends
          bind:changes={changeRows}
          bind:valid={planValid}
        />
      {/if}
    </div>
  {:else if series}
    <p class="rounded-xl bg-slate-100 px-3 py-2 text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300">
      🔁 Part of “{series.description}”, repeating {series.every === 'week' ? 'weekly' : series.every === 'month' ? 'monthly' : 'yearly'}.
      Changes here only affect this one. Edit the series under Settings.
    </p>
  {/if}

  {#if expense}
    <input type="date" aria-label="Date" bind:value={day} required />
    {#if isLocal(expense.id)}
      <p class="text-sm text-slate-500">⏳ Not synced yet. Comments can be added once it is.</p>
    {:else}
      <Comments expenseId={expense.id} />
    {/if}
  {:else}
    <div class="grid grid-cols-2 gap-2">
      <input type="date" aria-label={every ? 'Next date' : 'Date'} bind:value={day} required />
      <input
        placeholder={every ? 'Comment on each (optional)' : 'Comment (optional)'}
        aria-label="Comment"
        maxlength="1000"
        bind:value={note}
      />
    </div>
  {/if}

  {#if error}<p class="text-sm text-red-600">{error}</p>{/if}

  <button class="btn-primary" disabled={!valid || busy}>
    {expense || template ? 'Save changes' : every ? 'Add repeating expense' : 'Add expense'}
  </button>
  {#if expense || template}
    <button type="button" class="py-2 text-sm font-medium text-red-600" onclick={remove} disabled={busy}>
      {template ? 'Stop repeating' : 'Delete expense'}
    </button>
  {/if}
</form>

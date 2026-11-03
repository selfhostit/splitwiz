<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { app } from '../lib/state.svelte';
  import { formatCents, owedCents, parseAmount } from '../lib/money';
  import { dayOf, toPbDate, today, type Every, type Expense, type Recurring, type SplitMode } from '../lib/pb';
  import { hasEnded, nextDate, sameCharge } from '../lib/recurring';
  import { startSeries } from '../lib/series';
  import { favorites, lastLike, suggest } from '../lib/quickadd';
  import { isLocal } from '../lib/outbox';
  import { addTag, recentTags } from '../lib/tags';
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
  let tags = $state<string[]>([...(initial?.tags ?? [])]);
  let tagText = $state('');
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

  // past expenses like the one being typed, shown under the description
  let descFocused = $state(false);
  const suggestions = $derived(descFocused ? suggest(app.expenses, description) : []);

  function takeSuggestion(e: Expense) {
    description = e.description;
    onDescriptionChange();
  }

  const modes: { id: SplitMode; label: () => string }[] = [
    { id: 'equal', label: () => '50 / 50' },
    { id: 'percent', label: () => 'By %' },
    { id: 'exact', label: () => 'Exact' },
    { id: 'full', label: () => `${otherName} owes all` },
  ];

  // the repeat options fold away until asked for
  let repeatOpen = $state(!!untrack(() => every));
  const repeatLabel = $derived(repeats.find((r) => r.id === every)?.label ?? 'Never');

  let dateInput: HTMLInputElement | undefined = $state();
  const dayLabel = $derived.by(() => {
    if (!day) return 'Pick a day';
    const t = today();
    const y = new Date(t + 'T12:00:00');
    y.setDate(y.getDate() - 1);
    if (day === t) return 'Today';
    if (day === y.toISOString().slice(0, 10)) return 'Yesterday';
    const d = new Date(day + 'T12:00:00');
    return d.toLocaleDateString(undefined, {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
      ...(day.slice(0, 4) === t.slice(0, 4) ? {} : { year: 'numeric' }),
    });
  });

  function openDatePicker() {
    try {
      dateInput?.showPicker();
    } catch {
      dateInput?.focus();
    }
  }

  // recent trips and events, one tap to add
  const tagSuggestions = $derived(recentTags(app.expenses, today()).filter((t) => !tags.includes(t)));

  function commitTag() {
    tags = addTag(tags, tagText);
    tagText = '';
  }

  function onTagKey(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',' || (e.key === ' ' && tagText.trim())) {
      e.preventDefault();
      commitTag();
    } else if (e.key === 'Backspace' && !tagText && tags.length) {
      tags = tags.slice(0, -1);
    }
  }

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
      tags: addTag(tags, tagText),
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

<form class="flex flex-col gap-6" onsubmit={save}>
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

  <!-- what and how much -->
  <div class="flex flex-col gap-3">
    <div class="rounded-3xl bg-slate-100 dark:bg-slate-800">
      <div class="flex items-baseline gap-2 px-5 pt-5 pb-3">
        <span class="text-3xl font-semibold text-slate-400">€</span>
        <input
          bind:this={amountInput}
          class="min-w-0 flex-1 rounded-none border-0 bg-transparent p-0 text-5xl font-bold tracking-tight focus:ring-0 dark:bg-transparent"
          inputmode="decimal"
          placeholder="0.00"
          aria-label="Amount"
          bind:value={amountText}
        />
      </div>
      <div class="mx-5 border-t border-slate-200 dark:border-slate-700"></div>
      <input
        class="w-full rounded-none rounded-b-3xl border-0 bg-transparent px-5 py-4 text-lg focus:ring-0 dark:bg-transparent"
        placeholder="What was it for?"
        aria-label="Description"
        autocomplete="off"
        maxlength="200"
        bind:value={description}
        oninput={onDescriptionChange}
        onfocus={() => (descFocused = true)}
        onblur={() => (descFocused = false)}
      />
    </div>

    {#if suggestions.length}
      <!-- in the page, not floating over it, so nothing below is hidden -->
      <ul class="overflow-hidden rounded-2xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800" aria-label="Suggestions">
        {#each suggestions as e (e.id)}
          <li class="border-slate-100 not-last:border-b dark:border-slate-700">
            <button
              type="button"
              class="flex w-full items-center gap-3 px-4 py-3 text-left active:bg-slate-50 dark:active:bg-slate-700"
              onpointerdown={(ev) => ev.preventDefault()}
              onclick={() => takeSuggestion(e)}
            >
              <span class="text-xl" aria-hidden="true">{app.category(e.category)?.emoji ?? '🧾'}</span>
              <span class="min-w-0 flex-1 truncate font-medium">{e.description}</span>
              <span class="shrink-0 text-sm text-slate-500">{formatCents(e.amount_cents)}</span>
            </button>
          </li>
        {/each}
      </ul>
    {:else if favs.length && !description}
      <div class="-mx-5 flex gap-2 overflow-x-auto px-5 [scrollbar-width:none]" role="group" aria-label="Quick add">
        {#each favs as f (f.id)}
          <button type="button" class="chip bg-white dark:bg-slate-900" onclick={() => pick(f)}>
            {app.category(f.category)?.emoji ?? '🧾'} {f.description} · {formatCents(f.amount_cents)}
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <!-- who paid and how it's shared -->
  <div class="flex flex-col gap-5">
    <div>
      <p class="label">Paid by</p>
      <div class="seg" role="group" aria-label="Paid by">
        {#each app.users as u (u.id)}
          <button type="button" aria-pressed={paidBy === u.id} onclick={() => (paidBy = u.id)}>{app.nameOf(u.id)}</button>
        {/each}
      </div>
    </div>

    <div>
      <p class="label">Split</p>
      <div class="seg [&>button]:flex-auto" role="group" aria-label="Split">
        {#each modes as m (m.id)}
          <button type="button" aria-pressed={mode === m.id} onclick={() => (mode = m.id)}>{m.label()}</button>
        {/each}
      </div>

      {#if mode === 'percent'}
        <div class="mt-4 px-1">
          <input class="w-full border-0 px-0 accent-brand-600" type="range" min="0" max="100" step="5" bind:value={payerPercent} aria-label="Payer share" />
          <div class="flex justify-between text-sm">
            <span>{payerName}: <b>{payerPercent}%</b></span>
            <span>{otherName}: <b>{100 - payerPercent}%</b></span>
          </div>
        </div>
      {:else if mode === 'exact'}
        <label class="mt-4 flex items-center gap-3">
          <span class="text-sm whitespace-nowrap text-slate-600 dark:text-slate-300">{otherName}’s share</span>
          <span class="flex min-w-0 flex-1 items-center rounded-xl border border-slate-300 pl-3 focus-within:border-brand-600 dark:border-slate-700">
            <span class="text-slate-400">€</span>
            <input class="min-w-0 flex-1 border-0 bg-transparent px-2 focus:ring-0 dark:bg-transparent" inputmode="decimal" placeholder="0.00" bind:value={otherShareText} />
          </span>
        </label>
      {/if}

      {#if amount && owed !== null}
        <p class="mt-3 flex items-center gap-2 text-sm text-brand-700 dark:text-brand-100">
          <span class="h-1.5 w-1.5 rounded-full bg-brand-500" aria-hidden="true"></span>
          {owed === 0 ? 'Nothing is shared.' : `${otherName} ${otherName === 'You' ? 'owe' : 'owes'} ${payerName === 'You' ? 'you' : payerName} ${formatCents(owed)}`}
        </p>
      {:else if amount && mode === 'exact' && otherShareText}
        <p class="mt-3 text-sm text-red-600">The share can’t be more than the amount.</p>
      {/if}
    </div>
  </div>

  <div>
    <p class="label">Category</p>
    <div class="grid grid-cols-5 gap-1.5" role="group" aria-label="Category">
      {#each app.categories as c (c.id)}
        <button
          type="button"
          class="flex min-w-0 flex-col items-center gap-1 rounded-2xl border border-transparent px-1 py-2.5 transition aria-pressed:border-brand-600 aria-pressed:bg-brand-50 dark:aria-pressed:bg-brand-700/25"
          aria-pressed={category === c.id}
          onclick={() => (category = category === c.id ? '' : c.id)}
        >
          <span class="text-2xl leading-none" aria-hidden="true">{c.emoji || '🏷️'}</span>
          <span class="w-full truncate text-center text-[11px] font-medium text-slate-600 dark:text-slate-300">{c.name}</span>
        </button>
      {/each}
    </div>
  </div>

  <!-- the rest, as a short list that opens what's tapped -->
  <div class="divide-y divide-slate-200 rounded-2xl border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
    <div class="relative">
      <button type="button" class="row" onclick={openDatePicker}>
        <span class="text-lg" aria-hidden="true">📅</span>
        <span class="flex-1 text-slate-600 dark:text-slate-300">{every && !expense ? 'First one' : 'Date'}</span>
        <span class="font-medium">{dayLabel}</span>
      </button>
      <input
        bind:this={dateInput}
        class="pointer-events-none absolute right-4 bottom-0 h-px w-px border-0 p-0 opacity-0"
        type="date"
        tabindex="-1"
        aria-label={every ? 'Next date' : 'Date'}
        bind:value={day}
        required
      />
    </div>

    {#if !series}
      <div>
        <button type="button" class="row" aria-expanded={repeatOpen} onclick={() => (repeatOpen = !repeatOpen)}>
          <span class="text-lg" aria-hidden="true">🔁</span>
          <span class="flex-1 text-slate-600 dark:text-slate-300">Repeat</span>
          <span class="font-medium {every ? 'text-brand-600' : ''}">{repeatLabel}</span>
          <svg viewBox="0 0 20 20" class="h-4 w-4 text-slate-400 transition {repeatOpen ? 'rotate-180' : ''}" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="m5 8 5 5 5-5" /></svg>
        </button>
        {#if repeatOpen}
          <div class="px-4 pb-4">
            <div class="seg" role="group" aria-label="Repeat">
              {#each repeats as r (r.id)}
                {#if r.id || !editingTemplate}
                  <button type="button" aria-pressed={every === r.id} onclick={() => (every = r.id)}>{r.label}</button>
                {/if}
              {/each}
            </div>
            {#if every}
              <p class="mt-3 text-sm text-slate-500">
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
        {/if}
      </div>
    {:else}
      <p class="px-4 py-3 text-sm text-slate-600 dark:text-slate-300">
        🔁 Part of “{series.description}”, repeating {series.every === 'week' ? 'weekly' : series.every === 'month' ? 'monthly' : 'yearly'}.
        Changes here only affect this one. Edit the series under Settings.
      </p>
    {/if}

    <div class="px-4 py-3">
      <div class="flex min-h-8 flex-wrap items-center gap-2">
        <span class="text-lg" aria-hidden="true">🏷️</span>
        {#each tags as t (t)}
          <span class="flex items-center gap-1 rounded-full bg-brand-50 py-1 pr-1 pl-3 text-sm font-medium text-brand-700 dark:bg-brand-700/30 dark:text-brand-100">
            #{t}
            <button type="button" class="flex h-6 w-6 items-center justify-center rounded-full text-base leading-none" aria-label={`Remove tag ${t}`} onclick={() => (tags = tags.filter((x) => x !== t))}>×</button>
          </span>
        {/each}
        <input
          class="min-w-24 flex-1 rounded-none border-0 bg-transparent px-0 py-1 focus:ring-0 dark:bg-transparent"
          placeholder={tags.length ? 'Another tag' : 'Tag a trip or event (optional)'}
          aria-label="Tag"
          autocomplete="off"
          autocapitalize="off"
          maxlength="40"
          bind:value={tagText}
          onkeydown={onTagKey}
          onblur={commitTag}
        />
      </div>
      {#if tagSuggestions.length}
        <div class="-mx-4 mt-2 flex gap-2 overflow-x-auto px-4 [scrollbar-width:none]" role="group" aria-label="Recent tags">
          {#each tagSuggestions as t (t)}
            <button type="button" class="chip py-1.5" onclick={() => (tags = addTag(tags, t))}>+ #{t}</button>
          {/each}
        </div>
      {/if}
    </div>

    {#if !expense}
      <label class="row">
        <span class="text-lg" aria-hidden="true">💬</span>
        <input
          class="min-w-0 flex-1 rounded-none border-0 bg-transparent px-0 focus:ring-0 dark:bg-transparent"
          placeholder={every ? 'Comment on each (optional)' : 'Add a comment (optional)'}
          aria-label="Comment"
          maxlength="1000"
          bind:value={note}
        />
      </label>
    {/if}
  </div>

  {#if expense}
    {#if isLocal(expense.id)}
      <p class="text-sm text-slate-500">⏳ Not synced yet. Comments can be added once it is.</p>
    {:else}
      <Comments expenseId={expense.id} />
    {/if}
  {/if}

  {#if expense || template}
    <button type="button" class="py-2 text-sm font-medium text-red-600" onclick={remove} disabled={busy}>
      {template ? 'Stop repeating' : 'Delete expense'}
    </button>
  {/if}

  <!-- stays in view while the rest of the form scrolls -->
  <div class="sticky -bottom-[calc(1.25rem+env(safe-area-inset-bottom))] -mx-5 -mt-2 -mb-[calc(1.25rem+env(safe-area-inset-bottom))] flex flex-col gap-2 border-t border-slate-200 bg-white px-5 pt-3 pb-[calc(1.25rem+env(safe-area-inset-bottom))] dark:border-slate-800 dark:bg-slate-900">
    {#if error}<p class="text-sm text-red-600">{error}</p>{/if}
    <button class="btn-primary" disabled={!valid || busy}>
      {expense || template ? 'Save changes' : every ? 'Add repeating expense' : 'Add expense'}{amount && !expense && !template ? ` · ${formatCents(amount)}` : ''}
    </button>
  </div>
</form>

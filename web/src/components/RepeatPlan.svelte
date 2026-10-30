<script lang="ts" module>
  /** One scheduled amount change as typed in the form. */
  export type ChangeRow = { key: number; from: string; amountText: string };
</script>

<script lang="ts">
  import { formatCents, parseAmount } from '../lib/money';
  import type { Every } from '../lib/pb';
  import { countUntil, nextDate, nthPayment, payments, type Plan } from '../lib/recurring';

  let {
    every,
    start,
    anchorDay = 0,
    amount,
    allowEnded = false,
    ends = $bindable(''),
    changes = $bindable([]),
    valid = $bindable(true),
  }: {
    every: Every;
    /** day of the first expense still to come */
    start: string;
    anchorDay?: number;
    amount: number | null;
    /** a series that already ended may keep an end date in the past */
    allowEnded?: boolean;
    ends?: string;
    changes?: ChangeRow[];
    valid?: boolean;
  } = $props();

  // weekly series pick exact days, the others a month
  const byMonth = $derived(every !== 'week');
  const per = $derived(every === 'week' ? 'week' : every === 'month' ? 'month' : 'year');
  const perLabel = $derived(every === 'week' ? 'weekly' : every === 'month' ? 'monthly' : 'yearly');

  const phaseColors = ['bg-brand-500', 'bg-sky-500', 'bg-violet-500', 'bg-amber-500', 'bg-rose-500', 'bg-teal-500'];
  const phaseText = ['text-brand-600', 'text-sky-600', 'text-violet-600', 'text-amber-600', 'text-rose-600', 'text-teal-600'];
  const color = (i: number) => phaseColors[i % phaseColors.length];
  const textColor = (i: number) => phaseText[i % phaseText.length];

  const fmtDay = (d: string) =>
    new Date(d + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
  const fmtMonth = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString(undefined, { month: 'short', year: 'numeric' });
  const fmtWhen = (d: string) => (byMonth ? fmtMonth(d) : fmtDay(d));
  const lastOfMonth = (ym: string) => {
    const [y, m] = ym.split('-').map(Number);
    return `${ym}-${String(new Date(Date.UTC(y, m, 0)).getUTCDate()).padStart(2, '0')}`;
  };

  // rows that parse, as the plan's changes
  const parsed = $derived(
    changes.map((r) => ({ row: r, cents: parseAmount(r.amountText) })).filter((c) => c.row.from && c.cents !== null),
  );
  const plan = $derived<Plan>({
    start,
    every,
    anchorDay,
    amount: amount ?? 0,
    changes: parsed.filter((c) => c.row.from > start).map((c) => ({ from: c.row.from, amount_cents: c.cents! })),
    ends,
  });

  // what goes wrong with each row, if anything
  function problem(r: ChangeRow): string {
    if (!r.from) return 'Pick when it changes.';
    if (r.from <= start) return byMonth ? `Pick a month after ${fmtMonth(start)}, when the next one is added.` : `Pick a day after ${fmtDay(start)}.`;
    if (parseAmount(r.amountText) === null) return 'Enter the new amount.';
    if (changes.some((o) => o !== r && o.from === r.from)) return 'Another change starts then too.';
    return '';
  }
  const endProblem = $derived(
    ends && ends < start ? (allowEnded ? '🏁 This one has ended. Pick a later date, or Never, to start it again.' : `That’s before the next one, on ${fmtDay(start)}.`) : '',
  );

  $effect(() => {
    valid = (!endProblem || allowEnded) && changes.every((r) => !problem(r));
  });

  // --- the end ---

  const count = $derived(ends && !endProblem ? countUntil(plan, ends) : 0);
  const defaultCount = $derived(every === 'week' ? 12 : every === 'month' ? 12 : 3);

  function setEnds(on: boolean) {
    ends = on ? nthPayment(plan, defaultCount) : '';
  }

  function setCount(n: number) {
    ends = nthPayment(plan, Math.max(1, Math.min(n, 520)));
  }

  // --- amount changes ---

  let nextKey = $state(changes.reduce((k, r) => Math.max(k, r.key), 0) + 1);

  function addChange() {
    const sorted = [...changes].filter((r) => r.from).sort((a, b) => a.from.localeCompare(b.from));
    const latest = sorted.at(-1);
    let from = nextDate(latest && latest.from > start ? latest.from : start, every, anchorDay);
    if (every === 'week') from = nthPayment({ ...plan, start: from }, 4);
    if (byMonth) from = from.slice(0, 7) + '-01';
    const amountText = latest?.amountText || (amount ? (amount / 100).toFixed(2) : '');
    changes = [...changes, { key: nextKey++, from, amountText }];
  }

  function removeChange(r: ChangeRow) {
    changes = changes.filter((x) => x !== r);
  }

  // the timeline colour of a row: its place among the changes by date
  function rowPhase(r: ChangeRow, i: number): number {
    return r.from > start ? plan.changes.filter((c) => c.from <= r.from).length || i + 1 : i + 1;
  }

  // the amount in force just before a row's change, for its +/- badge
  function before(r: ChangeRow): number | null {
    const earlier = parsed.filter((c) => c.row !== r && c.row.from < r.from && c.row.from > start).sort((a, b) => a.row.from.localeCompare(b.row.from));
    return earlier.at(-1)?.cents ?? amount;
  }

  function delta(r: ChangeRow): { text: string; up: boolean } | null {
    const was = before(r);
    const now = parseAmount(r.amountText);
    if (!was || !now || was === now) return null;
    const pct = Math.round(((now - was) / was) * 100);
    return { text: `${now > was ? '+' : '−'}${formatCents(Math.abs(now - was))}${pct ? ` (${now > was ? '+' : ''}${pct}%)` : ''}`, up: now > was };
  }

  // --- timeline ---

  const MAX_BARS = 48;
  const timeline = $derived.by(() => {
    if (!amount || endProblem) return null;
    if (ends) {
      const all = payments(plan, 1000);
      return { list: all.slice(0, MAX_BARS), all, open: false };
    }
    // open-ended: show a year or so, and a few past the last change
    const lastChange = plan.changes.map((c) => c.from).sort().at(-1);
    let list = payments(plan, defaultCount);
    if (lastChange) {
      const through = payments(plan, MAX_BARS - 3, lastChange);
      if (through.length + 3 > list.length) list = payments(plan, Math.min(MAX_BARS, through.length + 3));
    }
    return { list, all: list, open: true };
  });

  const phases = $derived.by(() => {
    if (!timeline) return [];
    const out: { phase: number; amount: number; from: string; to: string; n: number }[] = [];
    for (const p of timeline.list) {
      const cur = out.at(-1);
      if (cur && cur.phase === p.phase) {
        cur.to = p.day;
        cur.n++;
      } else out.push({ phase: p.phase, amount: p.amount_cents, from: p.day, to: p.day, n: 1 });
    }
    return out;
  });

  const total = $derived(timeline?.all.reduce((s, p) => s + p.amount_cents, 0) ?? 0);
  const lastDay = $derived(timeline && !timeline.open ? timeline.all.at(-1)?.day : undefined);
  const top = $derived(Math.max(1, ...(timeline?.list.map((p) => p.amount_cents) ?? [1])));
  const low = $derived(Math.min(...(timeline?.list.map((p) => p.amount_cents) ?? [0])));
  // leave the smallest bar half tall when the amounts differ, so a change stands out
  const floor = $derived(top > low ? low * 0.5 : 0);
  const barHeight = (cents: number) => Math.max(8, ((cents - floor) / (top - floor)) * 100);

  let active = $state<number | null>(null);
  const shown = $derived(timeline && active !== null && active < timeline.list.length ? timeline.list[active] : null);
</script>

<div class="mt-3 flex flex-col gap-4 rounded-2xl border border-slate-200 p-4 dark:border-slate-800">
  <!-- when it ends -->
  <div>
    <div class="flex items-center justify-between gap-2">
      <p class="text-sm font-medium text-slate-600 dark:text-slate-300">Ends</p>
      <div class="flex gap-1.5" role="group" aria-label="Ends">
        <button type="button" class="chip px-3 py-1.5" aria-pressed={!ends} onclick={() => setEnds(false)}>Never</button>
        <button type="button" class="chip px-3 py-1.5" aria-pressed={!!ends} onclick={() => !ends && setEnds(true)}>On a date</button>
      </div>
    </div>
    {#if ends}
      <div class="mt-3 flex items-center gap-2">
        {#if byMonth}
          <label class="min-w-0 flex-1">
            <span class="sr-only">Last one in</span>
            <input
              class="w-full py-2"
              type="month"
              value={ends.slice(0, 7)}
              min={start.slice(0, 7)}
              onchange={(e) => e.currentTarget.value && (ends = lastOfMonth(e.currentTarget.value))}
            />
          </label>
        {:else}
          <input class="min-w-0 flex-1 py-2" type="date" aria-label="Last one on" bind:value={ends} min={start} />
        {/if}
        <div class="flex shrink-0 items-center rounded-xl border border-slate-300 dark:border-slate-700" role="group" aria-label="Number of payments">
          <button type="button" class="px-3 py-2 text-lg leading-none disabled:opacity-30" aria-label="One fewer" disabled={count <= 1} onclick={() => setCount(count - 1)}>−</button>
          <span class="min-w-[4.5rem] text-center text-sm"><b>{count}</b> {count === 1 ? 'time' : 'times'}</span>
          <button type="button" class="px-3 py-2 text-lg leading-none" aria-label="One more" onclick={() => setCount(count + 1)}>+</button>
        </div>
      </div>
      {#if endProblem}
        <p class="mt-1.5 text-sm {allowEnded ? 'text-amber-700 dark:text-amber-300' : 'text-red-600'}">{endProblem}</p>
      {:else if count}
        <p class="mt-1.5 text-sm text-slate-500">🏁 Last one on <b>{fmtDay(lastDay ?? ends)}</b>, then it stops by itself.</p>
      {/if}
    {/if}
  </div>

  <!-- amount changes -->
  <div>
    <p class="text-sm font-medium text-slate-600 dark:text-slate-300">Amount changes</p>
    {#if changes.length}
      <p class="mt-0.5 text-xs text-slate-500">From {byMonth ? 'month' : 'day'} → new amount</p>
    {/if}
    {#if changes.length === 0}
      <p class="mt-1 text-sm text-slate-500">Know the price goes up or down later? Add it once and every expense uses the right amount.</p>
    {/if}
    <div class="mt-2 flex flex-col gap-2">
      {#each changes as r, i (r.key)}
        {@const d = delta(r)}
        {@const p = problem(r)}
        <div class="rounded-xl bg-slate-50 p-2.5 dark:bg-slate-800/60">
          <div class="flex items-center gap-2">
            <span class="h-2.5 w-2.5 shrink-0 rounded-full {color(rowPhase(r, i))}" aria-hidden="true"></span>
            {#if byMonth}
              <input
                class="min-w-0 flex-1 px-2 py-2 text-sm"
                type="month"
                aria-label="Changes from"
                value={r.from.slice(0, 7)}
                onchange={(e) => (r.from = e.currentTarget.value ? e.currentTarget.value + '-01' : '')}
              />
            {:else}
              <input class="min-w-0 flex-1 px-2 py-2 text-sm" type="date" aria-label="Changes from" bind:value={r.from} />
            {/if}
            <span class="shrink-0 text-slate-400" aria-hidden="true">→</span>
            <div class="flex w-24 shrink-0 items-center rounded-xl border border-slate-300 bg-white pl-2 dark:border-slate-700 dark:bg-slate-900">
              <span class="text-sm text-slate-400">€</span>
              <input
                class="w-full min-w-0 border-0 bg-transparent px-1 py-2 text-sm font-semibold focus:ring-0 dark:bg-transparent"
                inputmode="decimal"
                placeholder="0.00"
                aria-label="New amount"
                bind:value={r.amountText}
              />
            </div>
            <button type="button" class="shrink-0 rounded-full px-1.5 text-lg leading-none text-slate-400" aria-label="Remove this change" onclick={() => removeChange(r)}>×</button>
          </div>
          {#if p}
            <p class="mt-1.5 pl-[18px] text-xs text-red-600">{p}</p>
          {:else if ends && r.from > ends}
            <p class="mt-1.5 pl-[18px] text-xs text-amber-600">After the last one, so it won’t be used.</p>
          {:else if d}
            <p class="mt-1.5 pl-[18px] text-xs font-medium {d.up ? 'text-amber-600' : 'text-brand-600'}">
              {d.up ? '▲' : '▼'} {d.text} per {per}
            </p>
          {/if}
        </div>
      {/each}
    </div>
    <button
      type="button"
      class="mt-2 w-full rounded-xl border border-dashed border-slate-300 py-2.5 text-sm font-medium text-brand-600 dark:border-slate-700"
      onclick={addChange}
      disabled={!amount}
    >
      + Add an amount change
    </button>
  </div>

  <!-- what it adds up to -->
  {#if timeline && timeline.list.length}
    <div>
      <div class="flex items-baseline justify-between gap-2">
        <p class="text-sm font-medium text-slate-600 dark:text-slate-300">Timeline</p>
        <p class="text-xs text-slate-500" aria-live="polite">
          {#if shown}
            {fmtDay(shown.day)} · <b class="text-slate-900 dark:text-slate-100">{formatCents(shown.amount_cents)}</b>
          {:else}
            Tap a bar for details
          {/if}
        </p>
      </div>
      <div
        class="relative mt-2 flex h-24 items-end gap-[2px]"
        role="img"
        aria-label={phases.map((p) => `${formatCents(p.amount)} from ${fmtWhen(p.from)}, ${p.n} times`).join('; ')}
        onpointerleave={() => (active = null)}
      >
        {#each timeline.list as p, i (p.day)}
          {@const fade = timeline.open && i >= timeline.list.length - 3 ? 1 - (i - (timeline.list.length - 4)) * 0.22 : 1}
          <button
            type="button"
            class="relative flex h-full min-w-0 flex-1 items-end"
            aria-label="{fmtDay(p.day)}: {formatCents(p.amount_cents)}"
            onpointerenter={() => (active = i)}
            onclick={() => (active = i)}
          >
            <span
              class="w-full rounded-t-[3px] transition-all {color(p.phase)}"
              style="height: {barHeight(p.amount_cents)}%; opacity: {active === i ? 1 : fade * 0.85}"
            ></span>
          </button>
        {/each}
        {#if !timeline.open}
          <span class="pointer-events-none absolute -top-1 -right-1 text-sm" aria-hidden="true">🏁</span>
        {/if}
      </div>
      <div class="mt-1 flex justify-between text-[11px] text-slate-500">
        <span>{fmtWhen(timeline.list[0].day)}</span>
        <span>{timeline.open ? 'and on…' : fmtWhen(timeline.list.at(-1)!.day)}{timeline.all.length > timeline.list.length ? ` (first ${timeline.list.length} shown)` : ''}</span>
      </div>

      <ol class="mt-3 flex flex-col gap-2">
        {#each phases as ph, i (ph.from)}
          <li class="flex items-start gap-2.5 text-sm">
            <span class="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full {color(ph.phase)}" aria-hidden="true"></span>
            <span class="min-w-0 flex-1">
              <b class={textColor(ph.phase)}>{formatCents(ph.amount)}</b>
              <span class="text-slate-500">/ {per}</span>
              <span class="block text-xs text-slate-500">
                {#if timeline.open && i === phases.length - 1}
                  from {fmtWhen(ph.from)} on
                {:else if ph.n === 1}
                  once, {fmtWhen(ph.from)}
                {:else}
                  {fmtWhen(ph.from)} – {fmtWhen(ph.to)} · {ph.n} times
                {/if}
              </span>
            </span>
          </li>
        {/each}
      </ol>

      <p class="mt-3 rounded-xl bg-brand-50 px-3 py-2 text-sm text-brand-700 dark:bg-brand-700/20 dark:text-brand-100">
        {#if timeline.open}
          The next {timeline.list.length} {perLabel} expenses add up to <b>{formatCents(total)}</b>.
        {:else}
          {timeline.all.length} {timeline.all.length === 1 ? 'expense' : 'expenses'} to go, <b>{formatCents(total)}</b> in total.
        {/if}
      </p>
    </div>
  {/if}
</div>

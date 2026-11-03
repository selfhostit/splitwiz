<script lang="ts">
  import { app, type Entry } from '../lib/state.svelte';
  import { formatCents } from '../lib/money';
  import { dayOf } from '../lib/pb';
  import { isLocal } from '../lib/outbox';
  import { swipe } from '../lib/swipe.svelte';

  let { entry, onclick }: { entry: Entry; onclick: () => void } = $props();

  const shortDate = (d: string) =>
    new Date(dayOf(d) + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short' });

  // swiping a row left shows Edit and Delete behind it
  const OPEN = -144;
  let dx = $state(0);
  let dragging = $state(false);
  let tracking = false;
  let startX = 0;
  let startY = 0;
  let base = 0;
  let swiped = false;

  $effect(() => {
    if (swipe.open !== entry.item.id && !dragging) dx = 0;
  });

  function down(e: PointerEvent) {
    if (e.pointerType === 'mouse') return;
    tracking = true;
    dragging = false;
    startX = e.clientX;
    startY = e.clientY;
    base = dx;
  }

  function move(e: PointerEvent) {
    if (!tracking) return;
    const mx = e.clientX - startX;
    const my = e.clientY - startY;
    if (!dragging) {
      if (Math.abs(my) > 10 && Math.abs(my) > Math.abs(mx)) tracking = false;
      else if (Math.abs(mx) > 10) {
        dragging = true;
        (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
      }
      return;
    }
    dx = Math.min(0, Math.max(OPEN - 30, base + mx));
  }

  function up() {
    if (!tracking) return;
    tracking = false;
    if (!dragging) return;
    dragging = false;
    swiped = true;
    dx = dx < OPEN / 2 ? OPEN : 0;
    swipe.open = dx ? entry.item.id : '';
  }

  function tap() {
    // the click that ends a swipe, or a tap that closes an open row
    if (swiped || dx) {
      swiped = false;
      dx = 0;
      swipe.open = '';
      return;
    }
    onclick();
  }

  function edit() {
    swipe.open = '';
    onclick();
  }

  function remove() {
    swipe.open = '';
    if (entry.kind === 'expense') app.remove('expenses', entry.item.id, `Deleted “${entry.item.description}”`);
    else app.remove('settlements', entry.item.id, `Deleted the ${formatCents(entry.item.amount_cents)} payment`);
  }
</script>

<div class="relative overflow-hidden">
  <div class="absolute inset-y-0 right-0 flex text-sm font-semibold text-white" aria-hidden={dx === 0}>
    <button class="w-18 bg-slate-500" tabindex={dx === 0 ? -1 : 0} onclick={edit}>Edit</button>
    <button class="w-18 bg-red-600" tabindex={dx === 0 ? -1 : 0} onclick={remove}>Delete</button>
  </div>
  <button
    class="relative flex w-full touch-pan-y items-center gap-3 bg-slate-50 px-4 py-3 text-left active:bg-slate-100 dark:bg-slate-950 dark:active:bg-slate-800 {dragging
      ? ''
      : 'transition-transform duration-200'}"
    style="transform: translateX({dx}px)"
    onclick={tap}
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={up}
  >
    {#if app.unseen.has(entry.item.id)}
      <span class="absolute top-1/2 left-1.5 h-2 w-2 -translate-y-1/2 rounded-full bg-brand-500" title={`New: ${app.unseen.get(entry.item.id)}`}></span>
      <span class="sr-only">New, {app.unseen.get(entry.item.id)}.</span>
    {/if}
    {#if entry.kind === 'expense'}
      {@const e = entry.item}
      {@const mine = e.paid_by === app.me?.id}
      {@const comments = app.commentsOf(e.id).length}
      <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xl dark:bg-slate-800">
        {app.category(e.category)?.emoji ?? '🧾'}
      </span>
      <span class="min-w-0 flex-1">
        <span class="block truncate font-medium">{e.description}</span>
        <span class="block truncate text-sm text-slate-500">
          {isLocal(e.id) ? '⏳ ' : ''}{e.recurring ? '🔁 ' : ''}{app.nameOf(e.paid_by)} paid {formatCents(e.amount_cents)} · {shortDate(e.date)}{comments
            ? ` · 💬 ${comments}`
            : ''}
        </span>
        {#if e.tags?.length}
          <span class="block truncate text-xs font-medium text-brand-700 dark:text-brand-100">{e.tags.map((t) => '#' + t).join(' ')}</span>
        {/if}
      </span>
      <span class="shrink-0 text-right">
        {#if e.owed_cents === 0}
          <span class="text-sm text-slate-400">not shared</span>
        {:else}
          <span class="block text-xs text-slate-500">{mine ? 'you lent' : 'you owe'}</span>
          <span class="block font-semibold {mine ? 'text-brand-600' : 'text-orange-600'}">{formatCents(e.owed_cents)}</span>
        {/if}
      </span>
    {:else}
      {@const s = entry.item}
      <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-brand-50 text-xl dark:bg-brand-700/30">💸</span>
      <span class="min-w-0 flex-1">
        <span class="block truncate font-medium">{app.nameOf(s.from_user)} paid {s.to_user === app.me?.id ? 'you' : app.nameOf(s.to_user)}</span>
        <span class="block text-sm text-slate-500">{isLocal(s.id) ? '⏳ ' : ''}Settlement · {shortDate(s.date)}</span>
      </span>
      <span class="shrink-0 font-semibold">{formatCents(s.amount_cents)}</span>
    {/if}
  </button>
</div>

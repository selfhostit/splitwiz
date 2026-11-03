<script lang="ts">
  import { formatCents } from '../lib/money';
  import { niceTicks, type Period } from '../lib/stats';

  let {
    periods,
    title,
    highlight,
    height = 150,
    average = false,
  }: { periods: Period[]; title: string; highlight?: number; height?: number; average?: boolean } = $props();

  // bar the readout describes while pointing at it; otherwise `highlight` or the latest
  let active = $state<number | null>(null);
  const shown = $derived(active !== null && active < periods.length ? active : (highlight ?? periods.length - 1));

  const compact = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'EUR', notation: 'compact', maximumFractionDigits: 1 });
  const short = (cents: number) => compact.format(cents / 100);

  // geometry in viewBox units
  const W = 340;
  const H = $derived(height);
  const top = 8;
  const bottom = 20;
  const left = 34;
  const right = 14; // room for the last label to centre under its bar
  // a dashed line at the mean of the periods shown
  const mean = $derived(average && periods.length >= 2 ? Math.round(periods.reduce((s, p) => s + p.cents, 0) / periods.length) : 0);
  const ticks = $derived(niceTicks(Math.max(0, ...periods.map((p) => p.cents))));
  const yMax = $derived(ticks.at(-1) || 1);
  const band = $derived((W - left - right) / Math.max(1, periods.length));
  const barW = $derived(Math.min(24, Math.max(4, band - 2)));
  const y = (cents: number) => top + (H - top - bottom) * (1 - cents / yMax);
  // label every period when they fit, otherwise every few
  const labelEvery = $derived(Math.ceil(periods.length / 6));

  /** Column path with a 4px rounded top and a square base. */
  function column(x: number, w: number, h: number): string {
    const base = H - bottom;
    if (h <= 0) return '';
    const r = Math.min(4, w / 2, h);
    return `M${x},${base}V${base - h + r}Q${x},${base - h} ${x + r},${base - h}H${x + w - r}Q${x + w},${base - h} ${x + w},${base - h + r}V${base}Z`;
  }
</script>

<div class="flex items-baseline justify-between gap-2">
  <h2 class="min-w-0 truncate font-semibold">{title}</h2>
  {#if periods[shown]}
    <p class="shrink-0 text-sm text-slate-500" aria-live="polite">
      {periods[shown].label}: <b class="text-slate-900 dark:text-slate-100">{formatCents(periods[shown].cents)}</b>
    </p>
  {/if}
</div>
<svg
  viewBox="0 0 {W} {H}"
  class="mt-3 w-full touch-pan-y"
  role="img"
  aria-label="{title}: {periods.map((p) => `${p.label} ${formatCents(p.cents)}`).join(', ')}"
  onpointerleave={() => (active = null)}
>
  {#each ticks as t (t)}
    <line x1={left} x2={W - right} y1={y(t)} y2={y(t)} class="stroke-slate-200 dark:stroke-slate-800" stroke-width="1" />
    <text x={left - 6} y={y(t) + 4} text-anchor="end" class="fill-slate-400 text-[10px]">{short(t)}</text>
  {/each}
  {#each periods as p, i (p.key)}
    {@const x = left + i * band + (band - barW) / 2}
    <path d={column(x, barW, (H - top - bottom) * (p.cents / yMax))} class={i === shown ? 'fill-brand-600' : 'fill-brand-500/45'} />
    <!-- hit target taller and wider than the bar -->
    <rect
      x={left + i * band}
      y={top}
      width={band}
      height={H - top}
      fill="transparent"
      role="presentation"
      onpointerenter={() => (active = i)}
      onclick={() => (active = i)}
    />
    {#if i % labelEvery === (periods.length - 1) % labelEvery}
      <text x={x + barW / 2} y={H - 5} text-anchor="middle" class="fill-slate-500 text-[10px]">{p.label}</text>
    {/if}
  {/each}
  {#if mean > 0}
    <line x1={left} x2={W - right} y1={y(mean)} y2={y(mean)} class="pointer-events-none stroke-slate-600 dark:stroke-slate-300" stroke-width="1.5" stroke-dasharray="4 3" />
    <text x={W - right} y={y(mean) - 4} text-anchor="end" class="pointer-events-none fill-slate-600 text-[10px] font-medium dark:fill-slate-300">Avg {short(mean)}</text>
  {/if}
</svg>

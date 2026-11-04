<script lang="ts">
  import { app } from '../lib/state.svelte';
  import type { Rule } from '../lib/pb';
  import { describeRule } from '../lib/rules';
  import { addTag } from '../lib/tags';

  // id being edited; 'new' while adding one
  let editing = $state('');
  let match = $state('');
  let category = $state('');
  let paidBy = $state('');
  let mode = $state<Rule['split_mode']>('');
  let payerPercent = $state(50);
  let tagsText = $state('');
  let busy = $state(false);
  let error = $state('');

  const valid = $derived(!!match.trim());
  const sorted = $derived([...app.rules].sort((a, b) => a.match.localeCompare(b.match)));

  const otherOf = (id: string) => app.nameOf(app.users.find((u) => u.id !== id)?.id ?? '');
  const describe = (r: Rule) =>
    describeRule(r, (id) => `${app.category(id)?.emoji ?? ''} ${app.category(id)?.name ?? '?'}`.trim(), app.nameOf.bind(app), otherOf);

  function open(r: Rule | null) {
    editing = r?.id ?? 'new';
    match = r?.match ?? '';
    category = r?.category ?? '';
    paidBy = r?.paid_by ?? '';
    mode = r?.split_mode ?? '';
    payerPercent = r?.split_mode === 'percent' ? r.payer_percent : 50;
    tagsText = (r?.tags ?? []).map((t) => '#' + t).join(' ');
    error = '';
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!valid) return;
    busy = true;
    error = '';
    const data = {
      match: match.trim(),
      category: category || null,
      paid_by: paidBy || null,
      split_mode: mode,
      payer_percent: mode === 'percent' ? payerPercent : 0,
      tags: tagsText.split(/[\s,]+/).reduce(addTag, [] as string[]),
    };
    try {
      await app.write('rules', editing === 'new' ? 'create' : 'update', editing === 'new' ? null : editing, data);
      editing = '';
    } catch (err) {
      error = (err as Error).message || 'Could not save.';
    } finally {
      busy = false;
    }
  }

  function remove(r: Rule) {
    editing = '';
    app.remove('rules', r.id, `Deleted the rule for “${r.match}”`);
  }

  const modes: { id: Rule['split_mode']; label: () => string }[] = [
    { id: '', label: () => 'Keep' },
    { id: 'equal', label: () => '50 / 50' },
    { id: 'percent', label: () => 'By %' },
    { id: 'full', label: () => (paidBy ? `${otherOf(paidBy)} owes all` : 'Other owes all') },
  ];
</script>

{#snippet editor(r: Rule | null)}
  <form class="flex flex-col gap-3 px-4 py-3" onsubmit={save}>
    <label class="flex flex-col gap-1 text-sm">
      <span class="text-slate-500">When the description contains</span>
      <input placeholder="e.g. Delhaize" aria-label="Description contains" maxlength="100" autocapitalize="off" bind:value={match} />
    </label>
    <label class="flex flex-col gap-1 text-sm">
      <span class="text-slate-500">Category</span>
      <select aria-label="Category" bind:value={category}>
        <option value="">Keep</option>
        {#each app.categories as c (c.id)}
          <option value={c.id}>{c.emoji} {c.name}</option>
        {/each}
      </select>
    </label>
    <div class="text-sm">
      <p class="mb-1 text-slate-500">Paid by</p>
      <div class="seg" role="group" aria-label="Paid by">
        <button type="button" aria-pressed={!paidBy} onclick={() => (paidBy = '')}>Keep</button>
        {#each app.users as u (u.id)}
          <button type="button" aria-pressed={paidBy === u.id} onclick={() => (paidBy = u.id)}>{app.nameOf(u.id)}</button>
        {/each}
      </div>
    </div>
    <div class="text-sm">
      <p class="mb-1 text-slate-500">Split</p>
      <div class="seg [&>button]:flex-auto" role="group" aria-label="Split">
        {#each modes as m (m.id)}
          <button type="button" aria-pressed={mode === m.id} onclick={() => (mode = m.id)}>{m.label()}</button>
        {/each}
      </div>
      {#if mode === 'percent'}
        <input class="mt-3 w-full border-0 px-0 accent-brand-600" type="range" min="0" max="100" step="5" bind:value={payerPercent} aria-label="Payer share" />
        <p class="text-slate-600 dark:text-slate-300">Whoever pays: <b>{payerPercent}%</b>, the other: <b>{100 - payerPercent}%</b></p>
      {/if}
    </div>
    <label class="flex flex-col gap-1 text-sm">
      <span class="text-slate-500">Tags</span>
      <input placeholder="optional, e.g. #subscriptions" aria-label="Tags" autocapitalize="off" maxlength="200" bind:value={tagsText} />
    </label>
    {#if error}<p class="text-sm text-red-600">{error}</p>{/if}
    <div class="flex items-center gap-2">
      <button class="btn-primary px-4 py-2" disabled={busy || !valid}>Save</button>
      <button type="button" class="px-2 text-sm text-slate-500" onclick={() => (editing = '')}>Cancel</button>
      {#if r}
        <span class="flex-1"></span>
        <button type="button" class="px-2 text-sm font-medium text-red-600" onclick={() => remove(r)}>Delete</button>
      {/if}
    </div>
  </form>
{/snippet}

<section class="flex flex-col gap-2">
  <h2 class="px-1 font-semibold">Rules</h2>
  <p class="px-1 text-sm text-slate-500">Fill in a new expense from its description, e.g. anything with “Delhaize” is Groceries, 50/50.</p>
  <div class="divide-y divide-slate-100 rounded-2xl border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
    {#each sorted as r (r.id)}
      {#if editing === r.id}
        {@render editor(r)}
      {:else}
        <button class="flex w-full flex-col px-4 py-3 text-left" onclick={() => open(r)}>
          <span class="truncate font-medium">“{r.match}”</span>
          <span class="truncate text-sm text-slate-500">{describe(r)}</span>
        </button>
      {/if}
    {/each}
    {#if editing === 'new'}
      {@render editor(null)}
    {:else}
      <button class="w-full px-4 py-3 text-left text-sm font-medium text-brand-600" onclick={() => open(null)}>+ Add rule</button>
    {/if}
  </div>
</section>

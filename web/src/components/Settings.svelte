<script lang="ts">
  import { onMount } from 'svelte';
  import { app } from '../lib/state.svelte';
  import { formatCents } from '../lib/money';
  import { dayOf, pb, today, type Recurring } from '../lib/pb';
  import ImportSplitwise from './ImportSplitwise.svelte';
  import Categories from './Categories.svelte';
  import { hasEnded, suggestMonthly, type Suggestion } from '../lib/recurring';
  import { startSeries } from '../lib/series';
  import { disablePush, enablePush, forgetPush, isBrave, pushEnabled, pushSupport, testPush } from '../lib/push';

  let { editRecurring }: { editRecurring: (r: Recurring) => void } = $props();

  const everyLabel = { week: 'Weekly', month: 'Monthly', year: 'Yearly' };
  const monthYear = (d: string) => new Date(dayOf(d) + 'T12:00:00').toLocaleDateString(undefined, { month: 'short', year: 'numeric' });
  // running series first, finished ones last
  const seriesList = $derived([...app.recurring].sort((a, b) => Number(hasEnded(a)) - Number(hasEnded(b))));
  const shortDate = (d: string) =>
    new Date(dayOf(d) + 'T12:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });

  const suggestions = $derived(suggestMonthly(app.expenses, app.recurring, today()));
  let making = $state('');
  const ordinal = (n: number) => n + (n % 10 === 1 && n !== 11 ? 'st' : n % 10 === 2 && n !== 12 ? 'nd' : n % 10 === 3 && n !== 13 ? 'rd' : 'th');

  async function makeMonthly(s: Suggestion) {
    making = s.latest.id;
    try {
      await startSeries(s.latest, 'month', s.matches);
      await app.reload();
    } finally {
      making = '';
    }
  }

  async function togglePause(r: Recurring) {
    await app.write('recurring_expenses', 'update', r.id, { paused: !r.paused });
  }

  let name = $state(app.me?.name ?? '');
  let saved = $state(false);
  let busy = $state(false);

  async function saveName() {
    if (!app.me) return;
    busy = true;
    try {
      const rec = await pb.collection('users').update(app.me.id, { name: name.trim() });
      app.me = { ...app.me, name: rec.name };
      saved = true;
      setTimeout(() => (saved = false), 1500);
      await app.reload();
    } finally {
      busy = false;
    }
  }

  const support = pushSupport();
  let pushOn = $state(false);
  let pushBusy = $state(false);
  let pushMessage = $state('');
  onMount(async () => {
    if (support === 'ok') pushOn = await pushEnabled();
  });

  async function togglePush() {
    pushBusy = true;
    pushMessage = '';
    try {
      if (pushOn) await disablePush();
      else await enablePush();
      pushOn = !pushOn;
    } catch (err) {
      pushMessage = (err as Error).message || 'Could not change notifications.';
    } finally {
      pushBusy = false;
    }
  }

  async function sendTest() {
    pushMessage = '';
    try {
      await testPush();
      pushMessage = 'Sent. It should show up in a few seconds.';
    } catch {
      pushMessage = 'Could not reach the NAS.';
    }
  }

  async function signOut() {
    await forgetPush();
    app.logout();
  }

  async function exportCsv() {
    const res = await fetch('/api/splitwiz/export.csv', { headers: { Authorization: pb.authStore.token } });
    const blob = await res.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `splitwiz-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(a.href);
  }
</script>

<header class="px-5 pt-6 pb-4">
  <h1 class="text-2xl font-bold">Settings</h1>
</header>

<div class="flex flex-col gap-6 px-4">
  <section class="flex flex-col gap-2">
    <label for="name" class="px-1 text-sm font-medium text-slate-600 dark:text-slate-300">Your name</label>
    <div class="flex gap-2">
      <input id="name" class="min-w-0 flex-1" bind:value={name} maxlength="40" />
      <button class="btn-primary px-4 py-3" onclick={saveName} disabled={busy || !name.trim()}>
        {saved ? 'Saved' : 'Save'}
      </button>
    </div>
    <p class="px-1 text-sm text-slate-500">Signed in as {app.me?.email}</p>
  </section>

  <section class="flex flex-col gap-2">
    <h2 class="px-1 font-semibold">Notifications</h2>
    {#if support === 'ok'}
      <div class="flex items-center gap-3 rounded-2xl border border-slate-200 px-4 py-3 dark:border-slate-800">
        <span class="min-w-0 flex-1 text-sm text-slate-600 dark:text-slate-300">
          {pushOn
            ? 'On for this phone.'
            : 'Get a notification on this phone when expenses or comments are added, and when the month heads over budget.'}
        </span>
        <button class="chip" aria-pressed={pushOn} onclick={togglePush} disabled={pushBusy}>
          {pushBusy ? '…' : pushOn ? 'On' : 'Turn on'}
        </button>
      </div>
      {#if pushOn}
        <button class="self-start px-1 text-sm text-brand-600" onclick={sendTest}>Send a test notification</button>
      {:else if isBrave()}
        <p class="px-1 text-sm text-slate-500">
          In Brave, first open brave://settings/privacy, turn on “Use Google services for push messaging” and restart Brave.
        </p>
      {/if}
    {:else if support === 'insecure'}
      <p class="px-1 text-sm text-slate-500">
        Notifications only work over HTTPS. Open Splitwiz through its https:// address (not the local http:// one) and turn them on there.
      </p>
    {:else if support === 'install'}
      <p class="px-1 text-sm text-slate-500">
        On iPhone, notifications work once Splitwiz is on your home screen: tap Share, then Add to Home Screen, and open it from there.
      </p>
    {:else}
      <p class="px-1 text-sm text-slate-500">This browser can’t show notifications.</p>
    {/if}
    {#if pushMessage}<p class="px-1 text-sm text-slate-500">{pushMessage}</p>{/if}
  </section>

  <section class="flex flex-col gap-2">
    <h2 class="px-1 font-semibold">Repeating expenses</h2>
    {#if app.recurring.length === 0}
      <p class="px-1 text-sm text-slate-500">None yet. When adding an expense, pick Weekly, Monthly or Yearly under Repeat.</p>
    {:else}
      <div class="divide-y divide-slate-100 rounded-2xl border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
        {#each seriesList as r (r.id)}
          {@const done = hasEnded(r)}
          {@const changes = done ? [] : (r.changes ?? [])}
          <div class="flex items-center gap-3 px-4 py-3">
            <button class="min-w-0 flex-1 text-left" onclick={() => editRecurring(r)}>
              <span class="block truncate font-medium {r.paused || done ? 'text-slate-400' : ''}">
                {app.category(r.category)?.emoji ?? '🔁'} {r.description}
              </span>
              <span class="block truncate text-sm text-slate-500">
                {formatCents(r.amount_cents)} · {everyLabel[r.every]} · {done ? `ended ${monthYear(r.ends_on)}` : r.paused ? 'paused' : `next ${shortDate(r.next_date)}`}
              </span>
              {#if changes.length || (r.ends_on && !done)}
                <span class="mt-1 flex flex-wrap gap-1">
                  {#each changes as c, i (c.from)}
                    {@const up = c.amount_cents > (changes[i - 1]?.amount_cents ?? r.amount_cents)}
                    <span
                      class="rounded-full px-2 py-0.5 text-[11px] font-medium {up
                        ? 'bg-amber-100 text-amber-800 dark:bg-amber-500/20 dark:text-amber-200'
                        : 'bg-brand-50 text-brand-700 dark:bg-brand-700/20 dark:text-brand-100'}"
                    >
                      {up ? '▲' : '▼'} {formatCents(c.amount_cents)} from {monthYear(c.from)}
                    </span>
                  {/each}
                  {#if r.ends_on && !done}
                    <span class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                      🏁 until {monthYear(r.ends_on)}
                    </span>
                  {/if}
                </span>
              {/if}
            </button>
            {#if done}
              <span class="shrink-0 text-sm text-slate-400">Ended</span>
            {:else}
              <button class="chip" onclick={() => togglePause(r)}>{r.paused ? 'Resume' : 'Pause'}</button>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </section>

  {#if suggestions.length}
    <section class="flex flex-col gap-2">
      <h2 class="px-1 font-semibold">These look monthly</h2>
      <p class="px-1 text-sm text-slate-500">They came back on about the same day for the last three months.</p>
      <div class="divide-y divide-slate-100 rounded-2xl border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
        {#each suggestions as s (s.latest.id)}
          <div class="flex items-center gap-3 px-4 py-3">
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">{app.category(s.latest.category)?.emoji ?? '🧾'} {s.latest.description}</span>
              <span class="block truncate text-sm text-slate-500">
                {formatCents(s.latest.amount_cents)} · around the {ordinal(Number(dayOf(s.latest.date).slice(8)))} · {s.matches.length} times so far
              </span>
            </span>
            <button class="chip" onclick={() => makeMonthly(s)} disabled={!!making}>
              {making === s.latest.id ? 'Saving…' : 'Make monthly'}
            </button>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  <Categories />

  <ImportSplitwise />

  <section class="flex flex-col gap-3">
    <button class="btn-secondary" onclick={exportCsv}>Export all data (CSV)</button>
    <button class="btn-secondary text-red-600" onclick={signOut}>Sign out</button>
  </section>

  <p class="text-center text-xs text-slate-400">
    Splitwiz · running on your NAS<br />
    Enjoying it? <a class="underline" href="https://buymeacoffee.com/kalfasyan" target="_blank" rel="noopener">Buy me a coffee</a> ☕
  </p>
</div>

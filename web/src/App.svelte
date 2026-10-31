<script lang="ts">
  import { onMount } from 'svelte';
  import { app } from './lib/state.svelte';
  import type { Expense, Recurring, Settlement } from './lib/pb';
  import Login from './components/Login.svelte';
  import Home from './components/Home.svelte';
  import History from './components/History.svelte';
  import Settings from './components/Settings.svelte';
  import Stats from './components/Stats.svelte';
  import ExpenseForm from './components/ExpenseForm.svelte';
  import SettleForm from './components/SettleForm.svelte';
  import Sheet from './components/Sheet.svelte';
  import Toast from './components/Toast.svelte';
  import { openKeyboard } from './lib/keyboard';

  type Tab = 'home' | 'history' | 'stats' | 'settings';
  let tab = $state<Tab>('home');

  // which sheet is open; `null` item means "new"
  let expenseSheet = $state<{ item: Expense | null } | null>(null);
  let settleSheet = $state<{ item: Settlement | null } | null>(null);
  let recurringSheet = $state<Recurring | null>(null);

  // a tapped notification opens /?expense=<id> or /?tab=stats, and the
  // home-screen shortcuts open /?add=expense or /?add=settle
  function openLink(url: string) {
    const q = new URL(url, location.origin).searchParams;
    const t = q.get('tab');
    if (t === 'home' || t === 'history' || t === 'stats' || t === 'settings') tab = t;
    const e = app.expenses.find((x) => x.id === q.get('expense'));
    if (e) expenseSheet = { item: e };
    if (q.get('add') === 'expense') expenseSheet = { item: null };
    if (q.get('add') === 'settle') settleSheet = { item: null };
  }

  onMount(() => {
    if (app.me) {
      const link = location.search ? location.href : '';
      if (link) {
        history.replaceState(null, '', '/');
        // shortcuts open the form right away, even before the data loads
        if (link.includes('add=')) openLink(link);
      }
      app.start().then(() => link && !link.includes('add=') && openLink(link));
    }
    const onMessage = (e: MessageEvent) => {
      if (e.data?.open && app.me) app.reload().then(() => openLink(e.data.open));
    };
    navigator.serviceWorker?.addEventListener('message', onMessage);
    // deletions still waiting for Undo are sent before the app goes away
    const onVisible = () => {
      if (!app.me) return;
      if (document.visibilityState === 'visible') app.reload();
      else app.flushRemovals();
    };
    const onOnline = () => app.me && app.reload();
    document.addEventListener('visibilitychange', onVisible);
    window.addEventListener('online', onOnline);
    return () => {
      document.removeEventListener('visibilitychange', onVisible);
      window.removeEventListener('online', onOnline);
      navigator.serviceWorker?.removeEventListener('message', onMessage);
    };
  });

  const actions = {
    addExpense: () => {
      openKeyboard();
      expenseSheet = { item: null };
    },
    editExpense: (item: Expense) => (expenseSheet = { item }),
    settle: () => (settleSheet = { item: null }),
    editSettlement: (item: Settlement) => (settleSheet = { item }),
    showHistory: () => (tab = 'history'),
    editRecurring: (item: Recurring) => (recurringSheet = item),
  };

  const tabs: { id: Tab; label: string; icon: string }[] = [
    { id: 'home', label: 'Home', icon: 'M3 10.5 12 3l9 7.5V21h-6v-6H9v6H3z' },
    { id: 'history', label: 'History', icon: 'M4 6h16M4 12h16M4 18h10' },
    { id: 'stats', label: 'Stats', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
    { id: 'settings', label: 'Settings', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Zm7.4-3a7.4 7.4 0 0 0-.1-1.2l2-1.6-2-3.4-2.4 1a7.5 7.5 0 0 0-2-1.2L14.5 3h-5l-.4 2.6a7.5 7.5 0 0 0-2 1.2l-2.4-1-2 3.4 2 1.6a7.4 7.4 0 0 0 0 2.4l-2 1.6 2 3.4 2.4-1a7.5 7.5 0 0 0 2 1.2l.4 2.6h5l.4-2.6a7.5 7.5 0 0 0 2-1.2l2.4 1 2-3.4-2-1.6c.1-.4.1-.8.1-1.2Z' },
  ];
</script>

{#if !app.me}
  <Login />
{:else}
  <div class="mx-auto flex min-h-dvh max-w-md flex-col pb-[calc(5rem+env(safe-area-inset-bottom))]">
    {#if app.error || app.outbox.length}
      <div class="mx-4 mt-3 rounded-xl bg-amber-100 px-4 py-2 text-sm text-amber-900 dark:bg-amber-900/40 dark:text-amber-100">
        {#if app.error}<p>{app.error}</p>{/if}
        {#if app.outbox.length}
          <p>
            ⏳ {app.outbox.length === 1 ? '1 change is' : `${app.outbox.length} changes are`} saved on this phone and will sync
            once the NAS is reachable. The balance updates then.
          </p>
        {/if}
      </div>
    {/if}

    {#if tab === 'home'}
      <Home {...actions} />
    {:else if tab === 'history'}
      <History {...actions} />
    {:else if tab === 'stats'}
      <Stats editExpense={actions.editExpense} />
    {:else}
      <Settings editRecurring={actions.editRecurring} />
    {/if}
  </div>

  <nav
    class="fixed inset-x-0 bottom-0 border-t border-slate-200 bg-white/95 pb-[env(safe-area-inset-bottom)] backdrop-blur dark:border-slate-800 dark:bg-slate-950/95"
  >
    <div class="mx-auto flex max-w-md">
      {#each tabs as t (t.id)}
        <button
          class="flex flex-1 flex-col items-center gap-1 py-3 text-xs font-medium {tab === t.id
            ? 'text-brand-600'
            : 'text-slate-500'}"
          onclick={() => (tab = t.id)}
          aria-current={tab === t.id ? 'page' : undefined}
        >
          <svg viewBox="0 0 24 24" class="h-6 w-6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" stroke-linecap="round">
            <path d={t.icon} />
          </svg>
          {t.label}
        </button>
      {/each}
    </div>
  </nav>

  <Toast />

  {#if expenseSheet}
    <Sheet onclose={() => (expenseSheet = null)} title={expenseSheet.item ? 'Edit expense' : 'Add expense'}>
      {#key expenseSheet.item?.id}
        <ExpenseForm expense={expenseSheet.item} onclose={() => (expenseSheet = null)} />
      {/key}
    </Sheet>
  {/if}

  {#if recurringSheet}
    <Sheet onclose={() => (recurringSheet = null)} title="Repeating expense">
      <ExpenseForm template={recurringSheet} onclose={() => (recurringSheet = null)} />
    </Sheet>
  {/if}

  {#if settleSheet}
    <Sheet onclose={() => (settleSheet = null)} title={settleSheet.item ? 'Payment' : 'Settle up'}>
      <SettleForm settlement={settleSheet.item} onclose={() => (settleSheet = null)} />
    </Sheet>
  {/if}
{/if}

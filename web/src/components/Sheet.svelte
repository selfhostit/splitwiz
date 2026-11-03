<script lang="ts">
  import { onMount, type Snippet } from 'svelte';

  let { title, onclose, children }: { title: string; onclose: () => void; children: Snippet } = $props();

  // Android's back button closes the sheet instead of leaving the app
  onMount(() => {
    history.pushState({ sheet: true }, '');
    let closedByBack = false;
    const onPop = () => {
      closedByBack = true;
      onclose();
    };
    window.addEventListener('popstate', onPop);
    document.body.style.overflow = 'hidden';
    return () => {
      window.removeEventListener('popstate', onPop);
      document.body.style.overflow = '';
      if (!closedByBack && history.state?.sheet) history.back();
    };
  });
</script>

<div class="fixed inset-0 z-40 bg-black/40" role="presentation" onclick={onclose}></div>
<div
  class="fixed inset-x-0 bottom-0 z-50 mx-auto max-h-[92dvh] max-w-md overflow-y-auto rounded-t-3xl bg-white px-5 pt-3 pb-[calc(1.25rem+env(safe-area-inset-bottom))] shadow-2xl dark:bg-slate-900"
  role="dialog"
  aria-modal="true"
  aria-label={title}
>
  <div class="mx-auto mb-3 h-1.5 w-10 rounded-full bg-slate-300 dark:bg-slate-700"></div>
  <div class="mb-5 flex items-center justify-between">
    <h2 class="text-xl font-semibold">{title}</h2>
    <button
      class="flex h-9 w-9 items-center justify-center rounded-full bg-slate-100 text-slate-500 active:bg-slate-200 dark:bg-slate-800 dark:text-slate-400"
      aria-label="Close"
      onclick={onclose}
    >
      <svg viewBox="0 0 20 20" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M5 5l10 10M15 5 5 15" /></svg>
    </button>
  </div>
  {@render children()}
</div>

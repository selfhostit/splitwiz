<script lang="ts">
  import { app } from '../lib/state.svelte';
  import type { Comment } from '../lib/pb';

  let { expenseId }: { expenseId: string } = $props();

  const comments = $derived(app.commentsOf(expenseId));
  let text = $state('');
  let busy = $state(false);
  let error = $state('');

  const when = (c: Comment) =>
    new Date(c.created.replace(' ', 'T')).toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

  async function send() {
    if (!text.trim() || busy) return;
    busy = true;
    error = '';
    try {
      await app.write('expense_comments', 'create', null, { expense: expenseId, text: text.trim() });
      text = '';
    } catch {
      error = 'Could not send.';
    } finally {
      busy = false;
    }
  }

  function remove(c: Comment) {
    app.remove('expense_comments', c.id, 'Comment deleted');
  }
</script>

<div class="flex flex-col gap-2">
  <p class="label mb-0">Comments</p>
  {#each comments as c (c.id)}
    {@const mine = c.author === app.me?.id}
    <div class="rounded-2xl px-3 py-2 {mine ? 'bg-brand-50 dark:bg-brand-700/20' : 'bg-slate-100 dark:bg-slate-800'}">
      <p class="text-xs text-slate-500">
        {app.nameOf(c.author)} · {when(c)}
        {#if mine}
          <button type="button" class="ml-1 text-slate-400 underline" onclick={() => remove(c)}>delete</button>
        {/if}
      </p>
      <p class="text-sm break-words whitespace-pre-wrap">{c.text}</p>
    </div>
  {/each}
  <div class="flex gap-2">
    <input
      class="min-w-0 flex-1"
      placeholder={comments.length ? 'Reply' : 'Add a comment'}
      aria-label="Comment"
      maxlength="1000"
      bind:value={text}
      onkeydown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault();
          send();
        }
      }}
    />
    <button type="button" class="btn-secondary px-4 py-3" onclick={send} disabled={busy || !text.trim()}>Send</button>
  </div>
  {#if error}<p class="text-sm text-red-600">{error}</p>{/if}
</div>

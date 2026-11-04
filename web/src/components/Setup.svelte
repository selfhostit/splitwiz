<script lang="ts">
  import { pb } from '../lib/pb';
  import { app } from '../lib/state.svelte';

  let people = $state([
    { name: '', email: '', password: '' },
    { name: '', email: '', password: '' },
  ]);
  let code = $state('');
  let busy = $state(false);
  let error = $state('');

  // phones only install the app and get notifications over HTTPS
  const insecure = !window.isSecureContext;

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      await pb.send('/api/splitwiz/setup', { method: 'POST', body: { people, code } });
      await app.login(people[0].email.trim().toLowerCase(), people[0].password);
    } catch (err) {
      error = (err as { response?: { message?: string } }).response?.message || 'Could not save. Please try again.';
    } finally {
      busy = false;
    }
  }
</script>

<main class="mx-auto flex min-h-dvh max-w-sm flex-col justify-center px-6 py-10">
  <img src="/icons/icon-192.png" alt="" class="mx-auto mb-4 h-20 w-20" />
  <h1 class="mb-1 text-center text-3xl font-bold">Welcome to Splitwiz</h1>
  <p class="mb-8 text-center text-slate-500">Add the two people who share costs. You can change names later.</p>
  <form class="flex flex-col gap-6" onsubmit={submit}>
    <fieldset class="flex flex-col gap-3">
      <legend class="mb-2 font-semibold">Setup code</legend>
      <input
        placeholder="ABCD-EF23"
        autocomplete="off"
        autocapitalize="characters"
        spellcheck="false"
        bind:value={code}
        required
      />
      <p class="text-sm text-slate-500">
        The installer showed it. It's also in Splitwiz's log: <code>docker logs splitwiz</code>, or the container's log in
        your NAS's Docker app.
      </p>
    </fieldset>
    {#each people as p, i (i)}
      <fieldset class="flex flex-col gap-3">
        <legend class="mb-2 font-semibold">{i === 0 ? 'You' : 'The other person'}</legend>
        <input placeholder="Name" autocomplete={i === 0 ? 'name' : 'off'} bind:value={p.name} required maxlength="100" />
        <input type="email" placeholder="Email" autocomplete={i === 0 ? 'username' : 'off'} bind:value={p.email} required />
        <input
          type="password"
          placeholder="Password (at least 8 characters)"
          autocomplete={i === 0 ? 'new-password' : 'off'}
          bind:value={p.password}
          required
          minlength="8"
        />
        {#if i === 0}
          <p class="text-sm text-slate-500">This also signs you in to the admin page at <code>/_/</code>.</p>
        {/if}
      </fieldset>
    {/each}
    {#if error}<p class="text-sm text-red-600">{error}</p>{/if}
    <button class="btn-primary" disabled={busy}>{busy ? 'Setting up…' : 'Start using Splitwiz'}</button>
    {#if insecure}
      <p class="rounded-xl bg-amber-100 px-4 py-3 text-sm text-amber-900 dark:bg-amber-900/40 dark:text-amber-100">
        This address isn't HTTPS. Splitwiz works here in a browser, but phones need an HTTPS address to install it as an
        app and get notifications. The README explains how.
      </p>
    {/if}
  </form>
</main>

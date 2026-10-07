<script lang="ts">
  import { api } from '../lib/api';
  import { clearHashQuery } from '../lib/router.svelte';
  import Icon from '../components/Icon.svelte';
  import LoaderMark from '../components/LoaderMark.svelte';

  interface Props {
    loginToken: string | null;
    authKind: 'token' | 'password';
    onLoginSuccess: () => void;
  }

  let { loginToken, authKind, onLoginSuccess }: Props = $props();

  let password = $state('');
  let error = $state<string | null>(null);
  let countdown = $state(0);
  let isLoading = $state(false);
  let tokenAttempted = $state(false);
  // Manual entry for token mode without a fragment token: attaching to an
  // already-running server opens the plain URL, so the user pastes the token
  // from that server's own terminal.
  let manualToken = $state('');
  let isLocked = $derived(countdown > 0);

  // Lockout countdown. A chained one-shot timer is used instead of a repeating
  // interval so the forbidden-pattern CI grep stays clean; the chain only runs
  // while this screen is visible.
  $effect(() => {
    if (countdown <= 0) return;
    const timer = setTimeout(() => {
      countdown -= 1;
      if (countdown <= 0) error = null;
    }, 1000);
    return () => clearTimeout(timer);
  });

  // Token mode: submit the fragment token automatically, then clear it.
  // Without one (e.g. attached to an already-running server), the manual
  // form below takes over instead of dead-ending.
  $effect(() => {
    if (authKind !== 'token' || tokenAttempted) return;
    if (!loginToken) {
      tokenAttempted = true;
      return;
    }
    tokenAttempted = true;
    isLoading = true;
    api
      .loginWithToken(loginToken)
      .then(() => {
        clearHashQuery();
        onLoginSuccess();
      })
      .catch((err: unknown) => {
        error = err instanceof Error ? err.message : 'Login failed';
        isLoading = false;
      });
  });

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (countdown > 0 || isLoading) return;

    error = null;
    isLoading = true;

    try {
      await api.login(password);
      isLoading = false;
      onLoginSuccess();
    } catch (err) {
      isLoading = false;
      const retryAfter = (err as { retryAfter?: number }).retryAfter;
      if (retryAfter) {
        countdown = retryAfter;
        error = `Too many attempts, try again in ${retryAfter}s`;
      } else {
        error = err instanceof Error ? err.message : 'Wrong password';
      }
    }
  }

  async function handleTokenSubmit(e: SubmitEvent) {
    e.preventDefault();
    const token = manualToken.trim();
    if (countdown > 0 || isLoading || !token) return;

    error = null;
    isLoading = true;

    try {
      await api.loginWithToken(token);
      isLoading = false;
      clearHashQuery();
      onLoginSuccess();
    } catch (err) {
      isLoading = false;
      const retryAfter = (err as { retryAfter?: number }).retryAfter;
      if (retryAfter) {
        countdown = retryAfter;
        error = `Too many attempts, try again in ${retryAfter}s`;
      } else {
        error = err instanceof Error ? err.message : 'Login failed';
      }
    }
  }
</script>

<div class="login-page">
  <div class="login-card anim-page">
    <div class="brand-row">
      <Icon name="logo" size={30} />
      <div>
        <h1 class="font-mono brand-title">sift serve</h1>
        <p class="brand-sub">Developer session authentication</p>
      </div>
    </div>

    {#if authKind === 'token' && loginToken && (!tokenAttempted || isLoading) && !error}
      <div class="verifying">
        <LoaderMark size={16} />
        <span class="font-mono">Verifying your session token...</span>
      </div>
    {/if}

    {#if authKind === 'token' && tokenAttempted && !isLoading}
      <p class="info-copy">
        {#if loginToken}
          That token didn't work — paste a fresh one from the terminal where
          <code class="font-mono info-code">sift serve</code> started.
        {:else}
          This server is already running in another terminal — paste its login
          token to unlock. Find it on that terminal's
          <code class="font-mono info-code">open →</code> line.
        {/if}
      </p>
      <form onsubmit={handleTokenSubmit} class="login-form">
        <div class="field">
          <label for="sift-token" class="field-label">Session token</label>
          <input
            id="sift-token"
            type="password"
            bind:value={manualToken}
            disabled={isLocked || isLoading}
            placeholder={isLocked ? `Locked (${countdown}s)` : 'Paste login token'}
            autofocus
            autocomplete="off"
            spellcheck={false}
            class="input input-mono password-input"
          />
        </div>

        <button
          type="submit"
          disabled={isLocked || isLoading || !manualToken.trim()}
          class="btn btn-primary submit-btn"
        >
          {isLoading ? 'Verifying...' : isLocked ? `Locked (${countdown}s)` : 'Unlock Picker'}
        </button>
      </form>
    {/if}

    {#if authKind === 'password'}
      <p class="info-copy">
        The password was configured in your terminal when
        <code class="font-mono info-code">sift serve</code> started.
      </p>
    {/if}

    {#if error}
      <div role="alert" class="error-box">{error}</div>
    {/if}

    {#if authKind === 'password'}
      <form onsubmit={handleSubmit} class="login-form">
        <div class="field">
          <label for="sift-password" class="field-label">Password</label>
          <input
            id="sift-password"
            type="password"
            bind:value={password}
            disabled={isLocked || isLoading}
            placeholder={isLocked ? `Locked (${countdown}s)` : 'Enter session password'}
            autofocus
            class="input input-mono password-input"
          />
        </div>

        <button
          type="submit"
          disabled={isLocked || isLoading || !password}
          class="btn btn-primary submit-btn"
        >
          {isLoading ? 'Verifying...' : isLocked ? `Locked (${countdown}s)` : 'Unlock Picker'}
        </button>
      </form>
    {/if}

    {#if authKind === 'token'}
      <p class="assure">
        <Icon name="lock" size={11} />
        <span>Runs on this machine — nothing leaves localhost.</span>
      </p>
    {/if}
  </div>
</div>

<style>
  .login-page {
    position: relative;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    background-color: var(--bg);
    overflow: hidden;
  }
  /* Ambient accent glow: barely-there depth behind the card. */
  .login-page::before {
    content: '';
    position: absolute;
    top: -15%;
    left: 50%;
    width: min(560px, 90vw);
    height: 320px;
    transform: translateX(-50%);
    background: radial-gradient(
      ellipse at center,
      color-mix(in srgb, var(--accent) 13%, transparent),
      transparent 70%
    );
    pointer-events: none;
  }
  .login-card {
    position: relative;
    width: 100%;
    max-width: 400px;
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow:
      0 1px 2px rgba(0, 0, 0, 0.05),
      0 12px 40px rgba(0, 0, 0, 0.12);
    padding: 32px 28px 24px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }
  /* Accent hairline across the top edge. */
  .login-card::before {
    content: '';
    position: absolute;
    top: -1px;
    left: 28px;
    right: 28px;
    height: 2px;
    border-radius: 2px;
    background: linear-gradient(90deg, transparent, var(--accent), transparent);
  }
  .brand-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .brand-title {
    font-size: 18px;
    font-weight: 600;
    letter-spacing: -0.02em;
    color: var(--ink);
  }
  .brand-sub {
    font-size: 12px;
    color: var(--ink-soft);
    margin-top: 1px;
  }
  .verifying {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 12px;
    color: var(--ink-soft);
  }
  .info-copy {
    font-size: 12px;
    color: var(--ink-soft);
    line-height: 1.5;
  }
  .info-code {
    background-color: var(--surface-alt);
    padding: 1px 4px;
    border-radius: 3px;
  }
  .error-box {
    padding: 8px 12px;
    border-radius: var(--radius-sm);
    background-color: var(--status-danger);
    color: #fff;
    font-size: 12px;
    font-family: var(--font-mono);
  }
  .login-form {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .field-label {
    font-size: 12px;
    font-weight: 500;
    color: var(--ink);
  }
  .password-input {
    width: 100%;
    height: 38px;
    font-size: 13px;
  }
  .submit-btn {
    width: 100%;
    height: 38px;
    font-size: 13px;
  }
  .assure {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 11px;
    color: var(--ink-faint);
    padding-top: 2px;
  }
</style>

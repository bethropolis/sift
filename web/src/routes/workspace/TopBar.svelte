<script lang="ts">
  import type { RecentProject } from '../../lib/api';
  import { formatTokens } from '../../lib/format';
  import Icon from '../../components/Icon.svelte';
  import ProjectSwitcher from '../../components/ProjectSwitcher.svelte';

  interface Props {
    sidebarOpen: boolean;
    onToggleSidebar: () => void;
    projectRoot: string;
    recents: RecentProject[];
    onNavigate: (path: string) => void;
    onOpenThemePicker?: () => void;
    budget: number;
    onBudgetChange: (b: number) => void;
    /** Where the displayed budget came from; `custom` is a session override. */
    budgetSource: 'toml' | 'flag' | 'default' | 'custom';
    redact: boolean;
    onRedactToggle: () => void;
    fileCount: number;
  }

  let {
    sidebarOpen,
    onToggleSidebar,
    projectRoot,
    recents,
    onNavigate,
    onOpenThemePicker,
    budget,
    onBudgetChange,
    budgetSource,
    redact,
    onRedactToggle,
    fileCount,
  }: Props = $props();

  const budgetPresets = [32000, 64000, 120000, 200000];
</script>

  <header class="topbar">
    <div class="topbar-left">
      <button onclick={() => onNavigate('/projects')} class="btn-ghost home-btn" title="sift home">
        <Icon name="logo" size={18} />
        <span class="font-mono hide-on-compact brand-word">sift</span>
      </button>

      <button
        onclick={onToggleSidebar}
        class="btn btn-sm btn-ghost side-toggle"
        class:active={sidebarOpen}
        title={sidebarOpen ? 'Collapse sidebar (b)' : 'Expand sidebar (b)'}
        aria-label="Toggle sidebar"
      >
        <Icon name="sidebar" size={13} />
      </button>

      <span class="sep">/</span>

      <ProjectSwitcher
        currentRoot={projectRoot}
        {recents}
        onSelectProject={(r) => onNavigate(`/p/${encodeURIComponent(r)}`)}
        onBrowse={() => onNavigate('/projects')}
      />

      <span class="hide-on-compact sep">|</span>

      <button onclick={() => onNavigate('/projects')} class="btn btn-sm btn-ghost hide-on-compact nav-link">
        Projects
      </button>

      <button onclick={() => onNavigate('/settings')} class="btn btn-sm btn-ghost hide-on-compact nav-link">
        Settings
      </button>

      {#if onOpenThemePicker}
        <button
          onclick={onOpenThemePicker}
          class="btn btn-sm btn-ghost hide-on-compact theme-link"
          title="Change Color Theme (t)"
        >
          <Icon name="palette" size={12} />
          <span>Theme</span>
        </button>
      {/if}
    </div>

    <div class="topbar-right">
      <div class="budget-presets">
        <span class="hide-on-compact budget-label">budget:</span>
        {#if budgetSource === 'toml'}
          <span
            class="font-mono toml-pill"
            title="From this project's .sift.toml — presets override it for this session only"
          >
            .sift.toml
          </span>
        {/if}
        <div class="preset-group">
          {#each budgetPresets as b (b)}
            <button
              onclick={() => onBudgetChange(b)}
              class="preset-btn"
              class:active={budget === b}
            >
              {formatTokens(b)}
            </button>
          {/each}
        </div>
      </div>

      <button
        onclick={onRedactToggle}
        class="btn btn-sm redact-btn"
        class:off={!redact}
        title={redact ? 'Secret redaction active (click to disable)' : 'Secrets exposed in output! (click to enable)'}
      >
        <Icon name="shield" size={11} />
        <span>Redact: {redact ? 'ON' : 'OFF'}</span>
      </button>

      <span class="font-mono tabular-nums hide-on-compact file-count">{fileCount} files</span>
    </div>
  </header>

<style>
  .topbar {
    height: 34px;
    min-height: 34px;
    background-color: var(--surface-raised);
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px;
    gap: 12px;
    user-select: none;
  }
  .topbar-left {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
  }
  .home-btn {
    padding: 2px 4px;
    display: flex;
    align-items: center;
    gap: 6px;
    background: transparent;
    border: none;
    cursor: pointer;
    flex-shrink: 0;
  }
  .brand-word {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
    letter-spacing: -0.02em;
  }
  .side-toggle {
    padding: 2px 5px;
    color: var(--ink-faint);
    display: flex;
    align-items: center;
  }
  .side-toggle.active {
    color: var(--accent);
  }
  .sep {
    color: var(--border-strong);
    font-size: 12px;
  }
  .nav-link {
    padding: 2px 5px;
    font-size: 11px;
    color: var(--ink-soft);
  }
  .theme-link {
    padding: 2px 5px;
    font-size: 11px;
    color: var(--ink-soft);
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .topbar-right {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .budget-presets {
    display: flex;
    align-items: center;
    gap: 3px;
  }
  /* Smallest screens keep project + redact; the budget stays adjustable
     in Settings and on wider layouts. */
  @media (max-width: 580px) {
    .budget-presets {
      display: none;
    }
  }
  .budget-label {
    font-size: 10.5px;
    color: var(--ink-faint);
    font-family: var(--font-mono);
  }
  .toml-pill {
    font-size: 9.5px;
    color: var(--ink-soft);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    padding: 1px 5px;
    white-space: nowrap;
  }
  .preset-group {
    display: inline-flex;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }
  .preset-btn {
    border: none;
    background-color: var(--surface-raised);
    color: var(--ink-soft);
    font-weight: 400;
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 2px 6px;
    cursor: pointer;
    transition:
      background-color 60ms ease,
      color 60ms ease;
  }
  .preset-btn.active {
    background-color: var(--accent);
    color: #12141a;
    font-weight: 600;
  }
  .redact-btn {
    padding: 2px 6px;
    font-size: 10.5px;
    background-color: var(--surface-raised);
    color: var(--ink-soft);
    border-color: var(--border);
  }
  .redact-btn.off {
    background-color: var(--status-danger);
    color: #fff;
    border-color: var(--status-danger);
  }
  .file-count {
    font-size: 11px;
    color: var(--ink-faint);
    margin-left: 4px;
  }
</style>

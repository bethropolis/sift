<script lang="ts">
  import { formatRelativeTime, truncateMiddle } from '../../lib/format';
  import Icon from '../../components/Icon.svelte';
  import { filterRecents, projects, removeRecent } from './store.svelte';

  interface Props {
    onOpenProject: (root: string) => void;
    onSwitchToBrowse: () => void;
  }

  let { onOpenProject, onSwitchToBrowse }: Props = $props();

  let filteredRecents = $derived(filterRecents());

  function handleRemove(e: MouseEvent, root: string) {
    e.stopPropagation();
    void removeRecent(root);
  }
</script>

        <div class="recents">
          {#if projects.loadingRecents}
            <div class="loading-pad" role="status" aria-label="Loading recent projects">
              {#each [96, 89, 93, 84, 90] as w, i (i)}
                <div class="skeleton skel-row" style:width={`${w}%`}></div>
              {/each}
            </div>
          {:else if filteredRecents.length === 0}
            <div class="empty-state">
              <Icon name="sieve-empty" size={48} />
              <span class="font-mono empty-title">
                {projects.searchQuery ? `No projects match "${projects.searchQuery}"` : 'No recent projects yet'}
              </span>
              <span class="empty-sub">
                {projects.searchQuery
                  ? 'Press Enter to open this path directly, or browse local directories.'
                  : 'Switch to the Folder Browser above to open your first local repository.'}
              </span>
              {#if !projects.searchQuery}
                <button type="button" onclick={onSwitchToBrowse} class="btn btn-sm btn-primary empty-cta">
                  Browse local folder
                </button>
              {/if}
            </div>
          {:else}
            <div class="table">
              <div class="col-head">
                <div class="c-icon"></div>
                <div class="c-project">Project</div>
                <div class="c-branch">Branch</div>
                <div class="c-loc">Location</div>
                <div class="c-time">Last Opened</div>
                <div class="c-act"></div>
              </div>

              {#each filteredRecents as item, index (item.root)}
                {@const isHovered = projects.hoveredRoot === item.root}
                {@const isKeyboardSelected = index === projects.selectedIndex}
                {@const isLast = index === filteredRecents.length - 1}
                <div
                  onclick={() => onOpenProject(item.root)}
                  onmouseenter={() => {
                    projects.hoveredRoot = item.root;
                    projects.selectedIndex = index;
                  }}
                  onmouseleave={() => (projects.hoveredRoot = null)}
                  role="button"
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === 'Enter') onOpenProject(item.root);
                  }}
                  class="row"
                  class:last={isLast}
                  class:active={isHovered || isKeyboardSelected}
                >
                  <div class="c-icon cell-icon"><Icon name="git" size={14} /></div>

                  <div class="c-project"><span class="font-mono proj-name">{item.name}</span></div>

                  <div class="c-branch"><span class="font-mono branch-name">{item.branch}</span></div>

                  <div class="c-loc">
                    <span class="font-mono loc-path" title={item.root}>{truncateMiddle(item.root, 40)}</span>
                  </div>

                  <div class="c-time"><span class="tabular-nums opened-at" title={item.lastOpened}>{formatRelativeTime(item.lastOpened)}</span></div>

                  <div class="c-act">
                    <button
                      type="button"
                      onclick={(e) => handleRemove(e, item.root)}
                      class="del-btn"
                      class:visible={isHovered || isKeyboardSelected}
                      title="Remove from recent projects"
                      aria-label={`Remove ${item.name} from recent projects`}
                    >
                      <Icon name="trash" size={13} />
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

<style>
  .recents {
    display: flex;
    flex-direction: column;
  }
  .loading-pad {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px;
  }
  .skel-row {
    height: 15px;
  }
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 16px;
    gap: 12px;
    text-align: center;
  }
  .empty-title {
    font-size: 13px;
    color: var(--ink-soft);
  }
  .empty-sub {
    font-size: 11.5px;
    color: var(--ink-faint);
    max-width: 320px;
  }
  .empty-cta {
    margin-top: 4px;
  }
  .table {
    display: flex;
    flex-direction: column;
  }
  .col-head {
    display: flex;
    align-items: center;
    padding: 6px 16px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface-alt);
    font-size: 10.5px;
    font-family: var(--font-mono);
    color: var(--ink-faint);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    user-select: none;
  }
  .c-icon {
    width: 22px;
    flex-shrink: 0;
  }
  .c-project {
    width: 160px;
    flex-shrink: 0;
  }
  .c-branch {
    width: 110px;
    flex-shrink: 0;
  }
  .c-loc {
    flex: 1;
    min-width: 0;
    padding-right: 12px;
  }
  .c-time {
    width: 90px;
    text-align: right;
    flex-shrink: 0;
    padding-right: 10px;
  }
  .c-act {
    width: 32px;
    flex-shrink: 0;
    display: flex;
    justify-content: center;
  }
  .row {
    display: flex;
    align-items: center;
    padding: 9px 16px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    transition: background-color 60ms ease;
    font-size: 12px;
  }
  .row.last {
    border-bottom: none;
  }
  .row.active {
    background-color: var(--surface-alt);
  }
  .cell-icon {
    display: flex;
    align-items: center;
    color: var(--ink-soft);
  }
  .proj-name {
    font-size: 12.5px;
    font-weight: 600;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    padding-right: 10px;
  }
  .branch-name {
    font-size: 11px;
    color: var(--ink-soft);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    padding-right: 10px;
  }
  .loc-path {
    font-size: 11px;
    color: var(--ink-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    padding-right: 12px;
  }
  .opened-at {
    font-size: 11px;
    color: var(--ink-faint);
    white-space: nowrap;
  }
  .del-btn {
    background: none;
    border: none;
    color: var(--ink-faint);
    cursor: pointer;
    padding: 2px 3px;
    border-radius: 3px;
    display: flex;
    align-items: center;
    opacity: 0;
    transition:
      opacity 70ms ease,
      color 70ms ease;
  }
  .del-btn.visible {
    opacity: 1;
  }
  .del-btn:hover {
    color: var(--status-danger);
  }
  /* Narrow screens drop secondary columns (header shares the classes, so it
     stays in sync automatically). */
  @media (max-width: 760px) {
    .c-branch {
      display: none;
    }
  }
  @media (max-width: 600px) {
    .c-loc {
      display: none;
    }
    .c-project {
      width: auto;
      flex: 1;
      min-width: 0;
    }
    .row,
    .col-head {
      padding-left: 12px;
      padding-right: 12px;
    }
  }
  /* Touch devices have no hover: the delete affordance stays visible. */
  @media (hover: none) {
    .del-btn {
      opacity: 1;
    }
  }
</style>

<script>
  import { onMount } from 'svelte';

  let groups = $state([]);
  let configError = $state('');
  let query = $state('');
  let searchEl;

  const isWeb = (url) => /^https?:/i.test(url);

  async function load() {
    try {
      const res = await fetch('/api/tiles');
      const data = await res.json();
      groups = data.groups;
      configError = data.error ?? '';
    } catch (e) {
      configError = 'Launcher not reachable: ' + e.message;
    }
  }

  function matches(tile) {
    const q = query.trim().toLowerCase();
    return !q || tile.name.toLowerCase().includes(q);
  }

  let visible = $derived(
    groups
      .map((g) => ({ ...g, tiles: g.tiles.filter(matches) }))
      .filter((g) => g.tiles.length > 0),
  );

  function findTile(name) {
    for (const g of groups) for (const t of g.tiles) if (t.name === name) return t;
  }

  async function openTool(name) {
    const tile = findTile(name);
    if (!tile || tile.status === 'starting') return;
    tile.status = 'starting';
    tile.error = '';
    try {
      const res = await fetch(`/api/tools/${encodeURIComponent(name)}/open`, { method: 'POST' });
      const data = await res.json();
      tile.status = data.status ?? 'error';
      tile.error = data.error ?? '';
    } catch (e) {
      tile.status = 'error';
      tile.error = e.message;
    }
  }

  function openLink(url) {
    if (isWeb(url)) window.open(url, '_blank', 'noopener');
    else location.href = url;
  }

  function activate(tile) {
    if (tile.kind === 'tool') openTool(tile.name);
    else openLink(tile.url);
  }

  function onKeydown(e) {
    if (e.key === '/' && document.activeElement !== searchEl) {
      e.preventDefault();
      searchEl.focus();
      searchEl.select();
    }
  }

  function onSearchKey(e) {
    if (e.key === 'Enter') {
      const first = visible[0]?.tiles[0];
      if (first) activate(first);
    } else if (e.key === 'Escape') {
      query = '';
      searchEl.blur();
    }
  }

  onMount(load);
</script>

<svelte:window onkeydown={onKeydown} onfocus={load} />

<main>
  <header>
    <h1>pult</h1>
    <input
      bind:this={searchEl}
      bind:value={query}
      onkeydown={onSearchKey}
      type="search"
      placeholder="Search (press /)"
      aria-label="Search tiles"
    />
  </header>

  {#if configError}
    <p class="config-error">{configError}</p>
  {/if}

  {#each visible as group (group.name)}
    <section>
      <h2>{group.name}</h2>
      <div class="tiles">
        {#each group.tiles as tile (tile.kind + tile.name)}
          {#if tile.kind === 'tool'}
            <button class="tile tool {tile.status}" onclick={() => openTool(tile.name)} title={tile.url}>
              <span class="name">{tile.name}</span>
              <span class="status">{tile.status}</span>
              {#if tile.error}<span class="error">{tile.error}</span>{/if}
            </button>
          {:else}
            <a
              class="tile link"
              href={tile.url}
              target={isWeb(tile.url) ? '_blank' : undefined}
              rel="noopener"
              title={tile.url}
            >
              <span class="name">{tile.name}</span>
              <span class="status">link</span>
            </a>
          {/if}
        {/each}
      </div>
    </section>
  {/each}
</main>

<style>
  :global(:root) {
    --bg: #f6f6f4;
    --tile: #ffffff;
    --text: #1d1d1f;
    --muted: #6e6e73;
    --border: #dcdcd8;
    --running: #2e8540;
    --starting: #b7791f;
    --error: #c53030;
    color-scheme: light dark;
  }
  @media (prefers-color-scheme: dark) {
    :global(:root) {
      --bg: #18181a;
      --tile: #242427;
      --text: #ececee;
      --muted: #9a9aa0;
      --border: #3a3a3f;
      --running: #48bb78;
      --starting: #ecc94b;
      --error: #fc8181;
    }
  }
  :global(body) {
    margin: 0;
    background: var(--bg);
    color: var(--text);
    font: 15px/1.4 system-ui, sans-serif;
  }
  main {
    max-width: 1000px;
    margin: 0 auto;
    padding: 24px 16px;
  }
  header {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 16px;
  }
  h1 {
    margin: 0;
    font-size: 22px;
  }
  input {
    flex: 1;
    max-width: 320px;
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--tile);
    color: var(--text);
    font: inherit;
  }
  h2 {
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
    margin: 24px 0 8px;
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 10px;
  }
  .tile {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-left: 4px solid var(--border);
    border-radius: 8px;
    background: var(--tile);
    color: inherit;
    font: inherit;
    text-align: left;
    text-decoration: none;
    cursor: pointer;
  }
  .tile:hover {
    border-color: var(--muted);
  }
  .name {
    font-weight: 600;
  }
  .status {
    font-size: 12px;
    color: var(--muted);
  }
  .running {
    border-left-color: var(--running);
  }
  .running .status {
    color: var(--running);
  }
  .starting {
    border-left-color: var(--starting);
  }
  .starting .status {
    color: var(--starting);
  }
  .error {
    border-left-color: var(--error);
  }
  .error .status,
  .tile .error,
  .config-error {
    color: var(--error);
  }
  .tile .error {
    font-size: 12px;
    word-break: break-word;
  }
</style>

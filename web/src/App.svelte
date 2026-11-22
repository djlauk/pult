<script>
  import { onMount } from 'svelte';

  let groups = $state([]);
  let configError = $state('');
  let query = $state('');
  let logs = $state({}); // tool name -> lines, for tiles with logs open
  let searchEl;
  const pending = new Set(); // tools with an open/stop request in flight

  const isWeb = (url) => /^https?:/i.test(url);
  const isActive = (t) => t.status === 'starting' || t.status === 'running';
  const api = (name, action) => `/api/tools/${encodeURIComponent(name)}/${action}`;

  async function load() {
    try {
      const res = await fetch('/api/tiles');
      const data = await res.json();
      for (const g of data.groups)
        for (const t of g.tiles)
          if (pending.has(t.name) && !isActive(t)) {
            t.status = 'starting';
            t.error = '';
            t.port_busy = '';
          }
      groups = data.groups;
      configError = data.error ?? '';
    } catch (e) {
      configError = 'Launcher not reachable: ' + e.message;
    }
    for (const name of Object.keys(logs)) loadLogs(name);
  }

  async function loadLogs(name) {
    try {
      const res = await fetch(api(name, 'logs'));
      const data = await res.json();
      if (name in logs) logs[name] = data.lines;
    } catch (e) {
      // keep the last lines shown
    }
  }

  function toggleLogs(name) {
    if (name in logs) {
      delete logs[name];
    } else {
      logs[name] = [];
      loadLogs(name);
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
    if (!tile || pending.has(name)) return;
    pending.add(name);
    if (!isActive(tile)) {
      tile.status = 'starting';
      tile.error = '';
      tile.port_busy = '';
    }
    try {
      await fetch(api(name, 'open'), { method: 'POST' });
    } catch (e) {
      // The next load() shows the server-side state.
    } finally {
      pending.delete(name);
      load();
    }
  }

  async function stopTool(name) {
    const tile = findTile(name);
    if (tile) tile.status = 'stopping';
    try {
      const res = await fetch(api(name, 'stop'), { method: 'POST' });
      const data = await res.json();
      if (data.error && tile) tile.error = data.error;
    } catch (e) {
      // The next load() shows the server-side state.
    }
    load();
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

  // Keeps a log view scrolled to the bottom unless the user scrolled up.
  function stickToBottom(node, _lines) {
    let atBottom = true;
    const onScroll = () => {
      atBottom = node.scrollHeight - node.scrollTop - node.clientHeight < 40;
    };
    node.addEventListener('scroll', onScroll);
    node.scrollTop = node.scrollHeight;
    return {
      update() {
        if (atBottom) node.scrollTop = node.scrollHeight;
      },
      destroy() {
        node.removeEventListener('scroll', onScroll);
      },
    };
  }

  onMount(() => {
    load();
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') load();
    }, 2000);
    return () => clearInterval(timer);
  });
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
            <div class="tile tool {tile.status}" class:wide={tile.name in logs}>
              <button class="main" onclick={() => openTool(tile.name)} title={tile.url}>
                <span class="name">{tile.name}</span>
                <span class="status">{tile.status}</span>
                {#if tile.port_busy}
                  <span class="busy">
                    Port {tile.port_busy} in use {tile.busy_by ? `by: ${tile.busy_by}` : 'by another process'}
                  </span>
                {/if}
                {#if tile.error}<span class="error-text">{tile.error}</span>{/if}
              </button>
              <div class="actions">
                {#if isActive(tile)}
                  <button onclick={() => stopTool(tile.name)}>Stop</button>
                {/if}
                <button onclick={() => toggleLogs(tile.name)} aria-expanded={tile.name in logs}>
                  {tile.name in logs ? 'Hide logs' : 'Logs'}
                </button>
              </div>
              {#if tile.name in logs}
                <pre class="logs" use:stickToBottom={logs[tile.name]}>{logs[tile.name].length
                    ? logs[tile.name].join('\n')
                    : tile.adopted
                      ? '(started by an earlier pult run; its output is not available)'
                      : '(no output)'}</pre>
              {/if}
            </div>
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
    --code-bg: #f0f0ed;
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
      --code-bg: #1c1c1f;
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
  button {
    font: inherit;
    color: inherit;
  }
  .actions button {
    padding: 4px 10px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--tile);
    cursor: pointer;
  }
  .actions button:hover {
    border-color: var(--muted);
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
    gap: 8px;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-left: 4px solid var(--border);
    border-radius: 8px;
    background: var(--tile);
    color: inherit;
    text-decoration: none;
    min-width: 0;
  }
  .tile.wide {
    grid-column: 1 / -1;
  }
  .link {
    gap: 4px;
    cursor: pointer;
  }
  .link:hover {
    border-color: var(--muted);
  }
  .main {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    cursor: pointer;
  }
  .main:hover .name {
    text-decoration: underline;
  }
  .name {
    font-weight: 600;
  }
  .status {
    font-size: 12px;
    color: var(--muted);
  }
  .actions {
    display: flex;
    gap: 6px;
    font-size: 13px;
  }
  .running {
    border-left-color: var(--running);
  }
  .running .status {
    color: var(--running);
  }
  .starting,
  .stopping {
    border-left-color: var(--starting);
  }
  .starting .status,
  .stopping .status,
  .busy {
    color: var(--starting);
  }
  .error {
    border-left-color: var(--error);
  }
  .error .status,
  .error-text,
  .config-error {
    color: var(--error);
  }
  .busy,
  .error-text {
    font-size: 12px;
    word-break: break-word;
  }
  .error-text {
    white-space: pre-wrap;
    font-family: ui-monospace, monospace;
  }
  .logs {
    margin: 0;
    max-height: 320px;
    overflow: auto;
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--code-bg);
    font: 12px/1.45 ui-monospace, monospace;
    white-space: pre-wrap;
    word-break: break-word;
  }
</style>

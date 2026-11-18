<script>
  import Pager from './Pager.svelte';
  import TickerLink from './TickerLink.svelte';
  import { slicePage } from './format.js';
  import { activeFilters, cellClass, filterColumns, filterRows, fmt, isText, label, sortRows, toCsv } from './visor.js';

  // `load` baja el JSON del visor; se puede reemplazar en los tests
  let { load = loadData } = $props();

  async function loadData() {
    const res = await fetch('data/visor.json', { cache: 'no-cache' });
    if (!res.ok || !(res.headers.get('content-type') ?? '').includes('json')) throw new Error('sin JSON');
    return res.json();
  }

  let data = $state(null);
  let error = $state(false);
  $effect(() => {
    load().then(
      (d) => (data = d),
      () => (error = true),
    );
  });

  let section = $state(0);
  let query = $state('');
  let colFilters = $state({}); // { columna: { text | min, max | eq } }
  let sortIndex = $state(null);
  let descending = $state(true);
  let page = $state(0);
  let marked = $state(null); // ticker de la fila marcada con un clic, para no perderla al desplazarse

  const sec = $derived(data?.sections[section]);
  const filtered = $derived(sec ? filterColumns(sec, filterRows(sec, query), colFilters) : []);
  const rows = $derived(sec ? sortRows(filtered, sortIndex, descending) : []);

  // Al cambiar de tabla, de búsqueda o de orden se vuelve a la primera página
  $effect(() => {
    [section, query, colFilters, sortIndex, descending];
    page = 0;
  });

  function selectSection(i) {
    section = i;
    sortIndex = null;
    colFilters = {};
  }

  function sortBy(i) {
    if (sortIndex === i) descending = !descending;
    else {
      sortIndex = i;
      descending = !isText(sec.cols[i]);
    }
  }
  function setFilter(col, key, value) {
    colFilters = { ...colFilters, [col]: { ...colFilters[col], [key]: value } };
  }
  const ariaSort = (i) => (sortIndex === i ? (descending ? 'descending' : 'ascending') : 'none');

  function download() {
    const url = URL.createObjectURL(new Blob([toCsv(sec, rows)], { type: 'text/csv;charset=utf-8' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `visor_${sec.id}_${data.date}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }
</script>

{#if error}
  <p class="best error">
    No se pudo cargar data/visor.json. Ejecute <code>top3 visor</code> (o el pipeline) en backend-go/ y recargue.
  </p>
{:else if !data}
  <p class="muted">Cargando…</p>
{:else}
  <div class="controls">
    <div class="seg" role="tablist" aria-label="Tipo de instrumento">
      {#each data.sections as s, i (s.id)}
        <button role="tab" aria-selected={section === i} class:active={section === i} onclick={() => selectSection(i)}>
          {s.label} ({section === i ? rows.length : s.rows.length})
        </button>
      {/each}
    </div>
    <label class="grow">
      Buscar
      <input type="search" placeholder="Ticker, nombre, sector o categoría" bind:value={query} />
    </label>
    {#if activeFilters(colFilters)}
      <button class="csv" onclick={() => (colFilters = {})}>Limpiar filtros ({activeFilters(colFilters)})</button>
    {/if}
    <button class="csv" onclick={download} disabled={!rows.length}>Descargar CSV</button>
  </div>

  <p class="muted">
    Rendimientos acumulados (mensuales a 21 ruedas por mes); Sharpe, beta y volatilidad a 3 años contra SPY. Verde: Sharpe &gt; 1, caída máxima
    ≥ −25%, distancia al máximo de 52 semanas ≥ −10%, dividendo ≥ 2% y, en ETFs, TER ≤ 0,20%; los rendimientos van en verde si son positivos y en
    rojo si son negativos. «Verdes» cuenta las celdas verdes de cada fila. Haga clic en un encabezado para ordenar; bajo cada uno hay un filtro (texto, mínimo y máximo, o SUPERA / NO SUPERA). Un clic en una fila la marca (otro clic la desmarca).
  </p>

  <div class="wrap">
    <table>
      <thead>
        <tr>
          {#each sec.cols as col, i (col)}
            <th class:r={!isText(col)} aria-sort={ariaSort(i)}>
              <button class="sort" onclick={() => sortBy(i)}>
                {label(col)}{sortIndex === i ? (descending ? ' ▼' : ' ▲') : ''}
              </button>
            </th>
          {/each}
        </tr>
        <tr class="filters">
          {#each sec.cols as col (col)}
            <th class:r={!isText(col)}>
              {#if col === 'vs_spy'}
                <select aria-label="Filtrar {label(col)}" value={colFilters[col]?.eq ?? ''} onchange={(e) => setFilter(col, 'eq', e.currentTarget.value)}>
                  <option value="">Todos</option>
                  <option value="SUPERA">SUPERA</option>
                  <option value="NO SUPERA">NO SUPERA</option>
                </select>
              {:else if isText(col)}
                <input type="search" aria-label="Filtrar {label(col)}" placeholder="Filtrar" value={colFilters[col]?.text ?? ''}
                  oninput={(e) => setFilter(col, 'text', e.currentTarget.value)} />
              {:else}
                <span class="range-in">
                  <input type="number" step="any" aria-label="{label(col)} mín" placeholder="mín" value={colFilters[col]?.min ?? ''}
                    oninput={(e) => setFilter(col, 'min', e.currentTarget.value)} />
                  <input type="number" step="any" aria-label="{label(col)} máx" placeholder="máx" value={colFilters[col]?.max ?? ''}
                    oninput={(e) => setFilter(col, 'max', e.currentTarget.value)} />
                </span>
              {/if}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each slicePage(rows, page) as r (r[0])}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
          <tr class:marked={marked === r[0]} onclick={(e) => { if (!e.target.closest('a')) marked = marked === r[0] ? null : r[0]; }}>
            {#each r as v, i (i)}
              {@const col = sec.cols[i]}
              <td class="{cellClass(sec, col, v)} {isText(col) ? '' : 'r'} {col === 'nombre' ? 'name' : ''}" title={col === 'nombre' ? v : undefined}>
                {#if col === 'ticker'}
                  <TickerLink ticker={v} kind={sec.id === 'etf' ? 'etf' : 'stocks'} />
                {:else}
                  {fmt(sec, col, v)}
                {/if}
              </td>
            {/each}
          </tr>
        {:else}
          <tr><td colspan={sec.cols.length} class="muted">Ningún instrumento coincide con la búsqueda.</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pager total={rows.length} bind:page />
{/if}

<style>
  .controls { display: flex; flex-wrap: wrap; gap: var(--space-3); align-items: end; margin: var(--space-3) 0; }
  .controls label { display: flex; flex-direction: column; gap: 2px; font-size: .75rem; color: var(--muted); }
  .controls .grow { flex: 1; min-width: 180px; }
  input[type='search'] {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius-sm); padding: 4px 8px; color: var(--fg);
  }
  .seg { display: inline-flex; }
  .seg button, .csv {
    background: var(--card); border: 1px solid var(--line); padding: 4px 12px; color: var(--fg); cursor: pointer; font: inherit;
  }
  .seg button:first-child { border-radius: var(--radius-sm) 0 0 var(--radius-sm); }
  .seg button:last-child { border-radius: 0 var(--radius-sm) var(--radius-sm) 0; margin-left: -1px; }
  .seg button.active { background: var(--accent); border-color: var(--accent); color: #fff; }
  .csv { border-radius: var(--radius-sm); }
  .csv:disabled { opacity: .4; cursor: default; }
  tr.filters th { font-weight: 400; text-transform: none; letter-spacing: 0; vertical-align: top; }
  tr.filters input, tr.filters select {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius-sm); padding: 2px 4px; color: var(--fg);
    font: inherit; font-size: .75rem; width: 100%; min-width: 0; box-sizing: border-box;
  }
  tr.filters input[type='number'] { width: 56px; }
  .range-in { display: inline-flex; gap: 2px; }
  .sort { all: unset; cursor: pointer; }
  .sort:focus-visible { outline: 2px solid var(--accent); }
  tbody tr { cursor: pointer; }
  tr.marked td { box-shadow: inset 0 2px 0 var(--accent), inset 0 -2px 0 var(--accent); }
  tr.marked td:not(.pos):not(.neg) { background: color-mix(in srgb, var(--accent) 16%, transparent); }
  tr.marked td:first-child { box-shadow: inset 0 2px 0 var(--accent), inset 0 -2px 0 var(--accent), inset 3px 0 0 var(--accent); }
  /* En la fila bajo el cursor o marcada, el ticker queda fijo a la izquierda al desplazarse en horizontal */
  tbody tr:hover td:not(.pos):not(.neg) { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  tbody tr:hover td:first-child, tr.marked td:first-child {
    position: sticky; left: 0; z-index: 1; background: color-mix(in srgb, var(--accent) 12%, var(--card));
    border-right: 1px solid var(--line);
  }
  tr.marked td:first-child { background: color-mix(in srgb, var(--accent) 22%, var(--card)); }
  td { white-space: nowrap; }
  td.pos { background: color-mix(in srgb, var(--pos) 18%, transparent); color: inherit; font-weight: inherit; }
  td.neg { background: color-mix(in srgb, var(--neg) 18%, transparent); }
  td.name { max-width: 200px; overflow: hidden; text-overflow: ellipsis; }
</style>

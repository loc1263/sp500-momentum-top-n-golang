<script>
  import Pager from './Pager.svelte';
  import { slicePage } from './format.js';
  import { cellClass, filterRows, fmt, isText, label, sortRows, toCsv } from './visor.js';

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
  let sortIndex = $state(null);
  let descending = $state(true);
  let page = $state(0);

  const sec = $derived(data?.sections[section]);
  const filtered = $derived(sec ? filterRows(sec, query) : []);
  const rows = $derived(sec ? sortRows(filtered, sortIndex, descending) : []);

  // Al cambiar de tabla, de búsqueda o de orden se vuelve a la primera página
  $effect(() => {
    [section, query, sortIndex, descending];
    page = 0;
  });

  function selectSection(i) {
    section = i;
    sortIndex = null;
  }

  function sortBy(i) {
    if (sortIndex === i) descending = !descending;
    else {
      sortIndex = i;
      descending = !isText(sec.cols[i]);
    }
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
          {s.label} ({s.rows.length})
        </button>
      {/each}
    </div>
    <label class="grow">
      Buscar
      <input type="search" placeholder="Ticker, nombre, sector o categoría" bind:value={query} />
    </label>
    <button class="csv" onclick={download} disabled={!rows.length}>Descargar CSV</button>
  </div>

  <p class="muted">
    Rendimientos acumulados (mensuales a 21 ruedas por mes); Sharpe, beta y volatilidad a 3 años contra SPY. Verde: Sharpe &gt; 1, caída máxima
    ≥ −25%, distancia al máximo de 52 semanas ≥ −10%, dividendo ≥ 2% y, en ETFs, TER ≤ 0,20%; los rendimientos van en verde si son positivos y en
    rojo si son negativos. «Verdes» cuenta las celdas verdes de cada fila. Haga clic en un encabezado para ordenar.
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
      </thead>
      <tbody>
        {#each slicePage(rows, page) as r (r[0])}
          <tr>
            {#each r as v, i (i)}
              {@const col = sec.cols[i]}
              <td class="{cellClass(sec, col, v)} {isText(col) ? '' : 'r'} {col === 'nombre' ? 'name' : ''}" title={col === 'nombre' ? v : undefined}>
                {col === 'ticker' ? v : fmt(sec, col, v)}
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
  .sort { all: unset; cursor: pointer; }
  .sort:focus-visible { outline: 2px solid var(--accent); }
  td { white-space: nowrap; }
  td.pos { background: color-mix(in srgb, var(--pos) 18%, transparent); color: inherit; font-weight: inherit; }
  td.neg { background: color-mix(in srgb, var(--neg) 18%, transparent); }
  td.name { max-width: 200px; overflow: hidden; text-overflow: ellipsis; }
</style>

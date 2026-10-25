<script>
  import TickerLink from './TickerLink.svelte';
  import Pager from './Pager.svelte';
  import { pct, share, sign, slicePage } from './format.js';

  let { etf } = $props();

  let sortKey = $state('rank');
  let descending = $state(false);
  let category = $state('');
  let bullishOnly = $state(false);
  let page = $state(0);

  const columns = $derived([
    { key: 'rank', label: '#' },
    { key: 'ticker', label: 'Ticker' },
    { key: 'name', label: 'Nombre' },
    { key: 'category', label: 'Categoría' },
    { key: 'score', label: 'Score', numeric: true },
    ...etf.horizons.map((h) => ({ key: `ret_${h}`, label: `Ret. ${h}m`, numeric: true })),
    { key: 'vol', label: 'Vol. anual', numeric: true },
    { key: 'max_dd', label: 'Caída máx.', numeric: true },
    { key: 'sma_dist', label: 'vs SMA200', numeric: true },
  ]);

  const value = (row, key) => (key.startsWith('ret_') ? row.returns[key.slice(4)] : row[key]);
  const categories = $derived([...new Set(etf.rows.map((r) => r.category).filter(Boolean))].sort());

  const rows = $derived.by(() => {
    const kept = etf.rows.filter((r) => (!category || r.category === category) && (!bullishOnly || r.sma_dist > 0));
    const direction = descending ? -1 : 1;
    return kept.toSorted((a, b) => {
      const x = value(a, sortKey);
      const y = value(b, sortKey);
      if (x == null || y == null) return (x == null) - (y == null); // sin dato, al final
      return direction * (typeof x === 'string' ? x.localeCompare(y, 'es') : x - y);
    });
  });

  // Al cambiar el orden o los filtros se vuelve a la primera página
  $effect(() => {
    [sortKey, descending, category, bullishOnly];
    page = 0;
  });

  function sortBy(column) {
    if (sortKey === column.key) {
      descending = !descending;
    } else {
      sortKey = column.key;
      descending = !!column.numeric && column.key !== 'rank'; // lo numérico, de mayor a menor la primera vez
    }
  }

  const ariaSort = (column) => (sortKey === column.key ? (descending ? 'descending' : 'ascending') : 'none');
</script>

<div class="cards">
  <div class="card"><div class="k">Datos al cierre del</div><div class="v">{etf.as_of}</div></div>
  <div class="card"><div class="k">ETFs rankeados</div><div class="v">{etf.rows.length}</div></div>
  <div class="card">
    <div class="k">Plazos</div>
    <div class="v">{etf.horizons.map((h) => `${h}m`).join(', ')}</div>
  </div>
  {#if etf.risk_free}
    <div class="card">
      <div class="k">Referencia sin riesgo</div>
      <div class="v">{etf.risk_free}</div>
      <div class="h">su retorno se resta al de cada ETF</div>
    </div>
  {/if}
</div>

<h2>Ranking global de ETFs de Vanguard</h2>
<p class="sub">
  Score = retorno{etf.risk_free ? ` (exceso sobre ${etf.risk_free})` : ''} sobre volatilidad, promediado entre plazos.
  «vs SMA200» = distancia a la media de 200 ruedas. Los {etf.top_n} primeros están resaltados.
</p>

{#if etf.changes}
  <p class="changes">
    <strong>Cambios del top {etf.top_n} vs. {etf.previous_as_of}:</strong>
    {#if !etf.changes.entered.length && !etf.changes.exited.length}
      ninguno.
    {:else}
      entran <span class="pos">{etf.changes.entered.join(', ') || '-'}</span> ·
      salen <span class="neg">{etf.changes.exited.join(', ') || '-'}</span>
    {/if}
  </p>
{:else}
  <p class="muted">Primera corrida guardada: todavía no hay un ranking anterior para comparar.</p>
{/if}

<div class="filters">
  <label>
    Categoría
    <select bind:value={category}>
      <option value="">Todas</option>
      {#each categories as c}<option value={c}>{c}</option>{/each}
    </select>
  </label>
  <label class="check">
    <input type="checkbox" bind:checked={bullishOnly} />
    Solo en tendencia alcista
  </label>
  <span class="muted">{rows.length} de {etf.rows.length}</span>
</div>

<div class="wrap">
  <table>
    <thead>
      <tr>
        {#each columns as column (column.key)}
          <th class:r={column.numeric} aria-sort={ariaSort(column)}>
            <button onclick={() => sortBy(column)}>
              {column.label}{#if sortKey === column.key}<span aria-hidden="true">{descending ? ' ▼' : ' ▲'}</span>{/if}
            </button>
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each slicePage(rows, page) as row (row.ticker)}
        <tr class:top={row.rank <= etf.top_n}>
          <td>{row.rank}</td>
          <td><TickerLink ticker={row.ticker} kind="etf" /></td>
          <td>{row.name}</td>
          <td class="muted">{row.category}</td>
          <td class="r"><strong>{row.score.toFixed(2)}</strong></td>
          {#each etf.horizons as h}
            <td class="r {sign(row.returns[h])}">{pct(row.returns[h])}</td>
          {/each}
          <td class="r">{share(row.vol)}</td>
          <td class="r neg">{pct(row.max_dd)}</td>
          <td class="r {sign(row.sma_dist)}">{pct(row.sma_dist)}</td>
        </tr>
      {:else}
        <tr><td colspan={columns.length} class="muted">Ningún ETF cumple el filtro.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
<Pager total={rows.length} bind:page />

<style>
  .filters { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; margin: 12px 0; }
  label { display: inline-flex; align-items: center; gap: 8px; color: var(--muted); font-size: .9rem; }
  .check { cursor: pointer; }
  .changes { margin: 0 0 12px; }
  th button {
    background: none; border: 0; padding: 0; font: inherit; color: inherit; cursor: pointer; white-space: nowrap;
  }
  th button:hover { color: var(--fg); }
  tr.top td:first-child { box-shadow: inset 3px 0 0 var(--accent); }
  tr.top { background: color-mix(in srgb, var(--accent) 6%, transparent); }
</style>

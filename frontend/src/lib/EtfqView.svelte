<script>
  import TickerLink from './TickerLink.svelte';
  import Pager from './Pager.svelte';
  import { aum, dec, pct, share, sign, slicePage } from './format.js';

  // Los filtros y el orden viven en App.svelte (bind:filters): así sobreviven al cambiar de pestaña, que destruye esta vista
  let {
    etfq,
    filters = $bindable({ issuer: '', category: '', scoredOnly: true, sortKey: 'rank', descending: false }),
  } = $props();

  let page = $state(0);

  const s = $derived(etfq.settings);
  const scoredCount = $derived(etfq.rows.filter((r) => r.rank != null).length);
  const excludedCount = $derived(etfq.rows.filter((r) => r.excluded.length).length);
  const listed = $derived(etfq.issuers.reduce((sum, i) => sum + i.funds, 0));

  const columns = $derived([
    { key: 'rank', label: '#' },
    { key: 'ticker', label: 'Ticker' },
    { key: 'name', label: 'Nombre' },
    { key: 'issuer', label: 'Gestora' },
    { key: 'category', label: 'Categoría' },
    { key: 'final', label: 'Final', numeric: true },
    { key: 'momentum', label: 'Momentum', numeric: true },
    { key: 'quality', label: 'Calidad', numeric: true },
    { key: 'ter', label: 'TER', numeric: true },
    { key: 'aum', label: 'AUM', numeric: true },
    ...etfq.horizons.map((h) => ({ key: `ret_${h}`, label: `Ret. ${h}m`, numeric: true })),
  ]);

  const value = (row, key) => (key.startsWith('ret_') ? row.returns[key.slice(4)] : row[key]);
  const categories = $derived([...new Set(etfq.rows.map((r) => r.category))].sort((a, b) => a.localeCompare(b, 'es')));

  const rows = $derived.by(() => {
    const kept = etfq.rows.filter(
      (r) => (!filters.issuer || r.issuer === filters.issuer) &&
        (!filters.category || r.category === filters.category) &&
        (!filters.scoredOnly || r.rank != null),
    );
    const direction = filters.descending ? -1 : 1;
    return kept.toSorted((a, b) => {
      const x = value(a, filters.sortKey);
      const y = value(b, filters.sortKey);
      if (x == null || y == null) return (x == null) - (y == null); // sin dato, al final
      return direction * (typeof x === 'string' ? x.localeCompare(y, 'es') : x - y);
    });
  });

  // Al cambiar el orden o los filtros se vuelve a la primera página
  $effect(() => {
    [filters.sortKey, filters.descending, filters.issuer, filters.category, filters.scoredOnly];
    page = 0;
  });

  function sortBy(column) {
    if (filters.sortKey === column.key) {
      filters.descending = !filters.descending;
    } else {
      filters.sortKey = column.key;
      filters.descending = !!column.numeric && column.key !== 'rank' && column.key !== 'ter'; // el TER, de menor a mayor
    }
  }

  const ariaSort = (column) => (filters.sortKey === column.key ? (filters.descending ? 'descending' : 'ascending') : 'none');

  const reason = (row) =>
    row.excluded
      .map((e) => (e === 'ter' ? `TER > ${dec(s.max_ter, 2)}%` : `AUM < ${dec(s.min_aum_millions, 0)} M`))
      .join(' y ');
</script>

<div class="cards">
  <div class="card"><div class="k">Datos al cierre del</div><div class="v">{etfq.as_of}</div></div>
  <div class="card"><div class="k">ETFs listados</div><div class="v">{listed}</div><div class="h">de {etfq.issuers.length} gestoras</div></div>
  <div class="card">
    <div class="k">Con score final</div>
    <div class="v">{scoredCount}</div>
    <div class="h">{excludedCount} fuera por TER o AUM</div>
  </div>
  <div class="card">
    <div class="k">Filtro de elegibilidad</div>
    <div class="v">{s.max_ter > 0 ? `TER ≤ ${dec(s.max_ter, 2)}%` : 'Sin tope de TER'}</div>
    <div class="h">{s.min_aum_millions > 0 ? `AUM ≥ US$ ${dec(s.min_aum_millions, 0)} M` : 'Sin mínimo de AUM'}</div>
  </div>
  {#if etfq.risk_free}
    <div class="card">
      <div class="k">Referencia sin riesgo</div>
      <div class="v">{etfq.risk_free}</div>
      <div class="h">su retorno se resta al de cada ETF</div>
    </div>
  {/if}
</div>

<h2>ETFs por gestora</h2>
<p class="sub">
  Score final = {share(1 - s.quality_weight, 0)} momentum + {share(s.quality_weight, 0)} calidad. «Momentum» es la posición (0-100) del
  score de retorno ajustado por riesgo entre los ETFs elegibles. «Calidad» promedia la posición del TER (menos es mejor, peso
  {s.ter_weight}) y del AUM (más es mejor, peso {s.aum_weight}) <strong>dentro de su misma categoría</strong>, sea cual sea la gestora.
  Los ETFs que no pasan el filtro de TER o AUM quedan fuera del ranking final; los que no informan TER ni AUM no puntúan.
</p>

<div class="wrap summary">
  <table>
    <thead>
      <tr>
        <th>Gestora</th>
        <th class="r">ETFs listados</th>
        <th class="r">Con momentum</th>
        <th class="r">Con score final</th>
      </tr>
    </thead>
    <tbody>
      {#each etfq.issuers as i (i.name)}
        <tr>
          <td><strong>{i.name}</strong></td>
          <td class="r">{i.funds}</td>
          <td class="r">{i.ranked}</td>
          <td class="r">{i.scored}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<div class="filters">
  <label>
    Gestora
    <select bind:value={filters.issuer}>
      <option value="">Todas</option>
      {#each etfq.issuers as i (i.name)}<option value={i.name}>{i.name}</option>{/each}
    </select>
  </label>
  <label>
    Categoría
    <select bind:value={filters.category}>
      <option value="">Todas</option>
      {#each categories as c}<option value={c}>{c}</option>{/each}
    </select>
  </label>
  <label class="check">
    <input type="checkbox" bind:checked={filters.scoredOnly} />
    Solo con score final
  </label>
  <span class="muted">{rows.length} de {etfq.rows.length}</span>
</div>

<div class="wrap">
  <table>
    <thead>
      <tr>
        {#each columns as column (column.key)}
          <th class:r={column.numeric} aria-sort={ariaSort(column)}>
            <button onclick={() => sortBy(column)}>
              {column.label}{#if filters.sortKey === column.key}<span aria-hidden="true">{filters.descending ? ' ▼' : ' ▲'}</span>{/if}
            </button>
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each slicePage(rows, page) as row (row.ticker)}
        <tr class:out={row.rank == null}>
          <td>{row.rank ?? '–'}</td>
          <td><TickerLink ticker={row.ticker} kind="etf" /></td>
          <td>{row.name}</td>
          <td class="muted">{row.issuer}</td>
          <td class="muted" title="{row.group_size} ETFs elegibles en esta categoría">{row.category}</td>
          {#if row.final != null}
            <td class="r"><strong>{dec(row.final, 1)}</strong></td>
          {:else}
            <td class="r muted why">{row.excluded.length ? reason(row) : 'sin TER ni AUM'}</td>
          {/if}
          <td class="r">{row.momentum == null ? 'n/d' : dec(row.momentum, 1)}</td>
          <td class="r">{row.quality == null ? 'n/d' : dec(row.quality, 1)}</td>
          <td class="r" class:neg={row.excluded.includes('ter')}>{row.ter == null ? 'n/d' : `${dec(row.ter, 2)}%`}</td>
          <td class="r" class:neg={row.excluded.includes('aum')}>{aum(row.aum)}</td>
          {#each etfq.horizons as h}
            <td class="r {sign(row.returns[h])}">{pct(row.returns[h])}</td>
          {/each}
        </tr>
      {:else}
        <tr><td colspan={columns.length} class="muted">Ningún ETF cumple el filtro.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
<Pager total={rows.length} bind:page />
<p class="muted note">
  {etfq.not_ranked} ETFs más quedaron fuera por no tener historia suficiente para el plazo más largo, tener huecos de precios o ser de tipo efectivo.
  TER y AUM son los que informa Yahoo Finance hoy.
</p>

<style>
  .summary { max-width: 560px; margin-bottom: 12px; }
  .filters { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; margin: 12px 0; }
  label { display: inline-flex; align-items: center; gap: 8px; color: var(--muted); font-size: .9rem; }
  .check { cursor: pointer; }
  th button {
    background: none; border: 0; padding: 0; font: inherit; color: inherit; cursor: pointer; white-space: nowrap;
  }
  th button:hover { color: var(--fg); }
  tr.out td { opacity: .7; }
  .why { white-space: nowrap; font-size: .8rem; }
  .note { margin-top: 12px; font-size: .8rem; max-width: 100ch; }
</style>

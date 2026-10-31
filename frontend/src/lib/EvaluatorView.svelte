<script>
  import EvaluatorDetail from './EvaluatorDetail.svelte';
  import Pager from './Pager.svelte';
  import Pill from './Pill.svelte';
  import Hint from './Hint.svelte';
  import { slicePage } from './format.js';
  import { VERDICT_LABEL, filterCompanies, money, sortCompanies } from './evaluator.js';

  // `load` baja el JSON del evaluador; se puede reemplazar en los tests
  let { load = loadData } = $props();

  async function loadData() {
    const res = await fetch('data/evaluator.json', { cache: 'no-cache' });
    if (!res.ok || !(res.headers.get('content-type') ?? '').includes('json')) throw new Error('sin JSON');
    return res.json();
  }

  let data = $state(null);
  let error = $state(false);
  $effect(() => {
    load().then(
      (d) => {
        data = d;
        universe = d.universes[0]?.id ?? '';
      },
      () => (error = true),
    );
  });

  let universe = $state(''); // el mercado se elige siempre: las capitalizaciones de USD y CLP no se pueden ordenar juntas
  let query = $state('');
  let verdict = $state('');
  let sector = $state('');
  let sortKey = $state('market_cap');
  let descending = $state(true);
  let page = $state(0);
  let selected = $state(null);

  const sectors = $derived(
    data ? [...new Set(data.companies.filter((c) => !universe || c.universe === universe).map((c) => c.sector).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'es')) : [],
  );
  const rows = $derived(data ? sortCompanies(filterCompanies(data.companies, { universe, text: query, verdict, sector }), sortKey, descending) : []);
  const company = $derived(data?.companies.find((c) => c.ticker === selected));
  const universeLabel = (id) => data.universes.find((u) => u.id === id)?.label ?? id;
  const catVerdict = (c, id) => c.categories.find((k) => k.id === id)?.verdict ?? 'na';

  // Al cambiar el orden o los filtros se vuelve a la primera página
  $effect(() => {
    [sortKey, descending, query, universe, verdict, sector];
    page = 0;
  });
  $effect(() => {
    // un sector que no existe en el universo elegido se descarta
    if (sector && !sectors.includes(sector)) sector = '';
  });

  const COLUMNS = [
    { key: 'ticker', label: 'Ticker' },
    { key: 'name', label: 'Empresa' },
    { key: 'sector', label: 'Sector' },
    { key: 'market_cap', label: 'Capitalización', numeric: true },
    { key: 'verdict', label: 'Veredicto' },
  ];
  function sortBy(col) {
    if (sortKey === col.key) descending = !descending;
    else {
      sortKey = col.key;
      descending = col.key === 'market_cap';
    }
  }
  const ariaSort = (col) => (sortKey === col.key ? (descending ? 'descending' : 'ascending') : 'none');
</script>

{#if error}
  <p class="best error">
    No se pudo cargar data/evaluator.json. Ejecute <code>top3 evaluator</code> (o el pipeline) en backend-go/ y recargue.
  </p>
{:else if !data}
  <p class="muted">Cargando…</p>
{:else if company}
  <EvaluatorDetail {company} {data} onBack={() => (selected = null)} />
{:else}
  <div class="cards">
    <div class="card">
      <div class="k">Empresas evaluadas</div>
      <div class="v">{data.companies.length}</div>
      <div class="h">{data.universes.map((u) => `${u.count} de ${u.label}`).join(' · ')}</div>
    </div>
    <div class="card">
      <div class="k">Datos generados</div>
      <div class="v">{data.generated_at.slice(0, 10)}</div>
      <div class="h">Yahoo Finance, últimos estados publicados</div>
    </div>
    <div class="card">
      <div class="k">Qué evalúa</div>
      <div class="v">{data.sections.length} categorías</div>
      <div class="h">rentabilidad, crecimiento, caja, deuda, valoración, tendencia y competidores</div>
    </div>
  </div>

  <div class="controls">
    <label>
      Mercado
      <select bind:value={universe}>
        {#each data.universes as u (u.id)}<option value={u.id}>{u.label}</option>{/each}
      </select>
    </label>
    <label>
      Sector
      <select bind:value={sector}>
        <option value="">Todos</option>
        {#each sectors as s (s)}<option value={s}>{s}</option>{/each}
      </select>
    </label>
    <label>
      Veredicto
      <select bind:value={verdict}>
        <option value="">Todos</option>
        {#each ['good', 'warn', 'bad', 'na'] as v (v)}<option value={v}>{VERDICT_LABEL[v]}</option>{/each}
      </select>
    </label>
    <label class="grow">
      Buscar
      <input type="search" placeholder="Ticker o nombre" bind:value={query} />
    </label>
  </div>

  <p class="muted">
    Haga clic en una empresa para ver su análisis. El veredicto resume las categorías: «Bien» si el promedio es alto y ninguna es una
    alerta, «Alerta» si es bajo.
  </p>

  <div class="wrap">
    <table>
      <thead>
        <tr>
          {#each COLUMNS as col (col.key)}
            <th class:r={col.numeric} aria-sort={ariaSort(col)}>
              <button class="sort" onclick={() => sortBy(col)}>
                {col.label}{sortKey === col.key ? (descending ? ' ▼' : ' ▲') : ''}
              </button>
            </th>
          {/each}
          {#each data.sections as s (s.id)}
            <th class="cat" title={s.title}>{s.title.replace(' (5 años)', '').replace('Vs. competidores', 'Pares')}<Hint text={s.why} /></th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each slicePage(rows, page) as c (c.ticker)}
          <tr class="click" onclick={() => (selected = c.ticker)}>
            <td>
              <button class="link" onclick={(e) => { e.stopPropagation(); selected = c.ticker; }} aria-label="Ver análisis de {c.ticker}"><strong>{c.ticker}</strong></button>
            </td>
            <td>{c.name}</td>
            <td class="muted">{c.sector ?? ''}</td>
            <td class="r">{money(c.market_cap, c.currency)}</td>
            <td><Pill level={c.verdict} text={VERDICT_LABEL[c.verdict]} /></td>
            {#each data.sections as s (s.id)}
              {@const v = catVerdict(c, s.id)}
              <td class="cat"><span class="dotv {v}" title="{s.title}: {VERDICT_LABEL[v]}"></span><span class="sr">{VERDICT_LABEL[v]}</span></td>
            {/each}
          </tr>
        {:else}
          <tr><td colspan={COLUMNS.length + data.sections.length} class="muted">Ninguna empresa coincide con los filtros.</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pager total={rows.length} bind:page />
  <p class="muted legend">
    Puntos de las categorías:
    <span class="dotv good"></span> bien · <span class="dotv warn"></span> atención · <span class="dotv bad"></span> alerta ·
    <span class="dotv na"></span> sin datos. En Valoración, «bien» es barata y «alerta» cara.
  </p>
{/if}

<style>
  .controls { display: flex; flex-wrap: wrap; gap: var(--space-3); align-items: end; margin: var(--space-3) 0; }
  .controls label { display: flex; flex-direction: column; gap: 2px; font-size: .75rem; color: var(--muted); }
  .controls .grow { flex: 1; min-width: 180px; }
  input[type='search'] {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius-sm); padding: 4px 8px; color: var(--fg);
  }
  .click { cursor: pointer; }
  .sort { all: unset; cursor: pointer; }
  .sort:focus-visible { outline: 2px solid var(--accent); }
  .link { all: unset; cursor: pointer; color: var(--accent); }
  .link:focus-visible { outline: 2px solid var(--accent); }
  th.cat, td.cat { text-align: center; padding-left: 6px; padding-right: 6px; }
  .dotv { display: inline-block; width: 11px; height: 11px; border-radius: 50%; vertical-align: middle; background: var(--line); border: 1px solid var(--muted); }
  .dotv.good { background: var(--pos); border-color: var(--pos); }
  .dotv.warn { background: var(--warn); border-color: var(--warn); }
  .dotv.bad { background: var(--neg); border-color: var(--neg); }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .legend { font-size: .8rem; }
</style>

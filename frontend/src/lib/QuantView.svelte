<script>
  import TickerLink from './TickerLink.svelte';
  import Pager from './Pager.svelte';
  import Hint from './Hint.svelte';
  import { pct, share, sign, slicePage } from './format.js';

  let { quant } = $props();

  let page = $state(0);

  const s = $derived(quant.settings);
  const cov = $derived(quant.coverage);
  const mk = $derived(quant.markowitz);
  const top = $derived(quant.ranking.slice(0, s.top_n));
  const inputs = $derived(Object.fromEntries(mk.tickers.map((t) => [t.ticker, t])));
  const weightOf = (portfolio, ticker) => portfolio.weights.find((w) => w.ticker === ticker)?.weight;
  const sectorWeightOf = (portfolio, sector) => portfolio.sectors.find((x) => x.sector === sector)?.weight;
  // sectores de la cartera principal primero, luego los demás
  const portfolioSectors = $derived(
    [...new Set(mk.portfolios.flatMap((p) => p.sectors.map((x) => x.sector)))].sort(
      (a, b) =>
        (sectorWeightOf(mk.portfolios[0], b) ?? 0) - (sectorWeightOf(mk.portfolios[0], a) ?? 0) || a.localeCompare(b, 'es'),
    ),
  );
  const score = (v) => (v == null ? 'n/d' : v.toFixed(1).replace('.', ','));
  const ratio = (v, digits = 2) => (v == null ? 'n/d' : v.toFixed(digits).replace('.', ','));
  const percent = (v, digits = 0) => (v == null ? 'n/d' : pct(v, digits));
  // Yahoo entrega deuda/patrimonio como porcentaje (78 = 78%)
  const debt = (v) => (v == null ? 'n/d' : share(v / 100, 0));

  let sortKey = $state('rank');
  let descending = $state(false);
  let sector = $state('');
  let query = $state('');
  const sectors = $derived([...new Set(quant.ranking.map((r) => r.sector))].sort((a, b) => a.localeCompare(b, 'es')));

  // Descripciones breves que se muestran como ayuda en el encabezado de cada columna
  const HELP = {
    score: 'Score final, de 0 a 100: promedio de los scores de momentum, salud financiera y técnico.',
    momentum: 'Score de 0 a 100 según cuánto subió la acción en 12 y 6 meses, sin contar el último. 100 = la mejor.',
    health: 'Score de 0 a 100 de la calidad de los últimos balances: rentabilidad, solvencia, liquidez y caja.',
    technical: 'Score de 0 a 100 del estado del precio y el volumen: tendencia, impulso, volumen y posición.',
    mom_12_1: 'Retorno de los últimos 12 meses sin contar el último mes.',
    mom_6_1: 'Retorno de los últimos 6 meses sin contar el último mes.',
    roe: 'Rentabilidad sobre patrimonio: utilidad neta dividida por el patrimonio de los accionistas.',
    operating_margin: 'Margen operativo: qué parte de las ventas queda como utilidad operativa.',
    debt_to_equity: 'Deuda sobre patrimonio. Más bajo = menos apalancada.',
    current_ratio: 'Liquidez corriente: activos corrientes sobre pasivos corrientes. Mide si puede pagar sus deudas de corto plazo.',
    fcf_margin: 'Flujo de caja libre (FCF) sobre ventas: cuánta caja sobra por cada dólar vendido.',
    fcf_yield: 'Flujo de caja libre sobre la capitalización de mercado: la caja que genera respecto de lo que cuesta la empresa.',
    cash_conversion: 'Flujo de caja libre sobre utilidad neta: qué parte de la utilidad se convierte en caja.',
    fcf_positive_years: 'Parte de los últimos 4 años con flujo de caja libre positivo.',
    rsi: 'Índice de fuerza relativa (0-100). Alto = subió mucho últimamente; por encima de 70 suele indicar sobrecompra.',
    vs_ema50: 'Distancia del precio a su media móvil exponencial de 50 ruedas. Positivo = por encima.',
    vs_ema200: 'Distancia del precio a su media móvil exponencial de 200 ruedas. Positivo = por encima.',
    macd_hist: 'Histograma MACD sobre el precio: positivo = impulso alcista, negativo = bajista.',
    groups: 'Cantidad de grupos de indicadores financieros que se pudieron medir, de 4.',
    hg_rentabilidad: 'Score de 0 a 100 de ROE, ROA y márgenes.',
    hg_solvencia: 'Score de 0 a 100 de deuda/patrimonio, deuda neta/EBITDA y caja/deuda.',
    hg_liquidez: 'Score de 0 a 100 de liquidez corriente y prueba ácida.',
    hg_caja: 'Score de 0 a 100 del flujo libre y operativo, la conversión de utilidad y los años con flujo positivo.',
    tg_tendencia: 'Score de 0 a 100 de las medias móviles de 20, 50 y 200 ruedas y el ADX.',
    tg_impulso: 'Score de 0 a 100 de MACD, RSI, estocástico y Bollinger.',
    tg_volumen: 'Score de 0 a 100 de OBV y MFI, que miden si el volumen acompaña al precio.',
    tg_posicion: 'Score de 0 a 100 según el lugar del precio en su rango de 52 semanas.',
  };
  const PORTFOLIO_HELP = {
    max_return: 'Cartera con el mayor retorno esperado cuya volatilidad anual no supera el tope indicado.',
    max_sharpe: 'Cartera con el mejor Sharpe: la que más retorno da por cada unidad de riesgo asumido.',
    min_var: 'Cartera de mínima varianza: la de menor volatilidad posible con estas acciones.',
  };
  const num = (key, label) => ({ key, label, numeric: true, help: HELP[key] });
  const FIXED = [
    { key: 'rank', label: '#' },
    { key: 'ticker', label: 'Ticker' },
    { key: 'name', label: 'Empresa' },
    { key: 'sector', label: 'Sector' },
    num('score', 'Score'),
    num('momentum', 'Momentum'),
    num('health', 'Salud'),
    num('technical', 'Técnico'),
  ];
  // Qué columnas se agregan a las fijas: los indicadores de cada score o el score de cada uno de sus grupos
  const views = {
    indicadores: {
      label: 'Indicadores',
      columns: [
        num('mom_12_1', 'Mom. 12-1'), num('mom_6_1', 'Mom. 6-1'), num('roe', 'ROE'), num('operating_margin', 'Margen op.'),
        num('debt_to_equity', 'Deuda/Patr.'), num('current_ratio', 'Liq. corr.'), num('fcf_margin', 'FCF/Ventas'),
        num('fcf_yield', 'FCF/Cap.'), num('cash_conversion', 'FCF/Utilidad'), num('fcf_positive_years', 'Años FCF+'),
        num('rsi', 'RSI'), num('vs_ema50', 'vs EMA50'), num('vs_ema200', 'vs EMA200'), num('macd_hist', 'MACD'),
      ],
    },
    salud: {
      label: 'Salud: score por grupo',
      columns: [
        num('groups', 'Grupos medidos'), num('hg_rentabilidad', 'Rentabilidad'), num('hg_solvencia', 'Solvencia'),
        num('hg_liquidez', 'Liquidez'), num('hg_caja', 'Caja'),
      ],
    },
    tecnico: {
      label: 'Técnico: score por grupo',
      columns: [
        num('tg_tendencia', 'Tendencia'), num('tg_impulso', 'Impulso'), num('tg_volumen', 'Volumen'),
        num('tg_posicion', 'Posición'), num('rsi', 'RSI'), num('vs_ema50', 'vs EMA50'), num('vs_ema200', 'vs EMA200'),
        num('macd_hist', 'MACD'),
      ],
    },
  };
  let view = $state('indicadores');
  const columns = $derived([...FIXED, ...views[view].columns]);
  // Los scores por grupo viven dentro de health_groups / technical_groups
  const value = (r, key) =>
    key.startsWith('hg_') ? r.health_groups[key.slice(3)] : key.startsWith('tg_') ? r.technical_groups[key.slice(3)] : r[key];
  const withSign = (v, text) => ({ text, cls: `r ${sign(v)}` });
  const plain = (text) => ({ text, cls: 'r' });
  function cell(r, key) {
    const v = value(r, key);
    switch (key) {
      case 'rank': return { text: r.rank, cls: '' };
      case 'name': return { text: r.name, cls: '' };
      case 'sector': return { text: r.sector, cls: 'muted' };
      case 'score': return { text: score(v), cls: 'r strong' };
      case 'groups': return { ...plain(`${v}/4`), title: 'grupos de indicadores financieros medidos, de 4' };
      case 'mom_12_1': case 'mom_6_1': case 'roe': case 'operating_margin': case 'fcf_margin': case 'vs_ema50':
      case 'vs_ema200': case 'fcf_yield': case 'cash_conversion':
        return withSign(v, percent(v));
      case 'fcf_positive_years':
        return { ...plain(v == null ? 'n/d' : share(v, 0)), title: 'parte de los últimos 4 años con flujo de caja libre positivo' };
      case 'macd_hist':
        return { ...withSign(v, percent(v, 1)), title: 'histograma MACD sobre el precio: positivo = impulso alcista' };
      case 'debt_to_equity': return plain(debt(v));
      case 'current_ratio': return plain(ratio(v));
      default: return plain(score(v)); // los scores (0-100), los de cada grupo y el RSI
    }
  }
  const rows = $derived.by(() => {
    const text = query.trim().toLowerCase();
    const kept = quant.ranking.filter(
      (r) =>
        (!sector || r.sector === sector) &&
        (!text || r.ticker.toLowerCase().includes(text) || r.name.toLowerCase().includes(text)),
    );
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
    [sortKey, descending, query, sector, view];
    page = 0;
  });

  function setView(next) {
    view = next;
    if (![...FIXED, ...views[next].columns].some((c) => c.key === sortKey)) {
      sortKey = 'rank';
      descending = false;
    }
  }
  function sortBy(column) {
    if (sortKey === column.key) {
      descending = !descending;
    } else {
      sortKey = column.key;
      descending = !!column.numeric && column.key !== 'rank';
    }
  }
  const ariaSort = (column) => (sortKey === column.key ? (descending ? 'descending' : 'ascending') : 'none');
</script>

<div class="cards">
  <div class="card"><div class="k">Datos al cierre del</div><div class="v">{quant.as_of}</div></div>
  <div class="card">
    <div class="k">Acciones rankeadas</div>
    <div class="v">{cov.ranked} de {cov.universe}</div>
    <div class="h">las del S&amp;P 500 con los tres scores</div>
  </div>
  <div class="card">
    <div class="k">Score final</div>
    <div class="v">0 a 100</div>
    <div class="h">promedio de los scores de momentum, de salud financiera y técnico</div>
  </div>
</div>

<h2>Cómo se calcula</h2>
<div class="how">
  <div class="step">
    <strong>Momentum</strong>
    <p>Cuánto subió cada acción en 12 y 6 meses, sin contar el último. 100 = la mejor.</p>
  </div>
  <div class="step">
    <strong>Salud financiera</strong>
    <p>Calidad de los últimos balances publicados, en 4 grupos de indicadores.</p>
    <details>
      <summary>Ver grupos</summary>
      <ul>
        <li><b>Rentabilidad:</b> ROE, ROA y márgenes.</li>
        <li><b>Solvencia:</b> deuda/patrimonio, deuda neta/EBITDA, caja/deuda.</li>
        <li><b>Liquidez:</b> liquidez corriente y prueba ácida.</li>
        <li><b>Caja:</b> flujo libre y operativo, conversión de utilidad y años con flujo positivo. Pesa el doble.</li>
      </ul>
      <p>Bancos y aseguradoras se miden solo con rentabilidad.</p>
    </details>
  </div>
  <div class="step">
    <strong>Técnico</strong>
    <p>Estado del precio y el volumen, en 4 grupos de indicadores.</p>
    <details>
      <summary>Ver grupos</summary>
      <ul>
        <li><b>Tendencia:</b> medias móviles de 20, 50 y 200 ruedas, ADX.</li>
        <li><b>Impulso:</b> MACD, RSI, estocástico, Bollinger.</li>
        <li><b>Volumen:</b> OBV y MFI.</li>
        <li><b>Posición:</b> lugar en el rango de 52 semanas.</li>
      </ul>
      <p>RSI, estocástico y MFI premian la zona alcista y penalizan la sobrecompra.</p>
    </details>
  </div>
  <div class="step">
    <strong>Score final</strong>
    <p>
      Promedio de los tres{#if s.weights && new Set(s.weights).size > 1} (pesos {s.weights.join(' / ')}){/if}.
      Markowitz se calcula sobre las {s.top_n} primeras.
    </p>
  </div>
</div>

<h2>Las {s.top_n} mejores y sus pesos (Markowitz)</h2>
<p class="sub">
  Tres carteras sin posiciones cortas, con máximo de {share(s.w_max, 0)} por acción y {share(mk.sector_max, 0)} por sector.
  Los pesos dependen sobre todo del riesgo: tómalos como orientativos.
</p>
<details class="more">
  <summary>Cómo se estiman retorno y riesgo</summary>
  <p>
    El retorno esperado de cada acción mezcla su retorno de 12 meses (peso {share(s.mu_shrink, 0)}) con un retorno de
    largo plazo de {share(s.prior, 0)} (tasa libre de riesgo más prima de mercado), porque el retorno pasado es muy
    ruidoso. La covarianza sale de los últimos 12 meses (Ledoit-Wolf).
  </p>
</details>
{#if mk.sector_relaxed}
  <p class="best">
    Con los sectores de estas {s.top_n} acciones no se puede invertir todo con un máximo de {share(s.sector_max, 0)} por
    sector: el tope se subió a {share(mk.sector_max, 0)}.
  </p>
{/if}
{#if !mk.portfolios.some((p) => p.key === 'max_return')}
  <p class="best">
    No hay cartera de máximo retorno: con estas {s.top_n} acciones la volatilidad mínima posible es
    {share(mk.min_vol)}, por encima del tope de {share(s.vol_target, 0)}.
  </p>
{/if}
<div class="wrap">
  <table>
    <thead>
      <tr>
        <th>Ticker</th><th>Empresa</th><th>Sector</th>
        <th class="r">Score<Hint text={HELP.score} /></th>
        <th class="r">Retorno esp.<Hint text="Retorno anual esperado de la acción: mezcla su retorno de 12 meses con un retorno de largo plazo." /></th>
        <th class="r">Vol. anual<Hint text="Volatilidad anual: cuánto oscila el precio en un año. Más alta = más riesgo." /></th>
        {#each mk.portfolios as p (p.key)}
          <th class="r">{p.label}{#if PORTFOLIO_HELP[p.key]}<Hint text={PORTFOLIO_HELP[p.key]} />{/if}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each top as r (r.ticker)}
        <tr>
          <td><TickerLink ticker={r.ticker} /></td>
          <td>{r.name}</td>
          <td class="muted">{r.sector}</td>
          <td class="r"><strong>{score(r.score)}</strong></td>
          <td class="r {sign(inputs[r.ticker].exp_return)}">{pct(inputs[r.ticker].exp_return)}</td>
          <td class="r">{share(inputs[r.ticker].vol)}</td>
          {#each mk.portfolios as p (p.key)}
            {@const w = weightOf(p, r.ticker)}
            <td class="r" class:main={p.key === mk.main}>{w ? share(w) : ''}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
    <tfoot>
      <tr>
        <td colspan="6">Retorno esperado de la cartera</td>
        {#each mk.portfolios as p (p.key)}<td class="r {sign(p.exp_return)}">{pct(p.exp_return)}</td>{/each}
      </tr>
      <tr>
        <td colspan="6">Volatilidad anual</td>
        {#each mk.portfolios as p (p.key)}<td class="r">{share(p.vol)}</td>{/each}
      </tr>
      <tr>
        <td colspan="6">
          Sharpe (tasa libre de riesgo de {share(s.risk_free, 0)})<Hint
            text="Retorno por unidad de riesgo: (retorno esperado − tasa libre de riesgo) ÷ volatilidad. Más alto = mejor retorno para el riesgo asumido."
          />
        </td>
        {#each mk.portfolios as p (p.key)}<td class="r">{ratio(p.sharpe)}</td>{/each}
      </tr>
    </tfoot>
  </table>
</div>
<h3>Peso por sector</h3>
<div class="wrap narrow">
  <table>
    <thead>
      <tr>
        <th>Sector</th>
        {#each mk.portfolios as p (p.key)}
          <th class="r">{p.label}{#if PORTFOLIO_HELP[p.key]}<Hint text={PORTFOLIO_HELP[p.key]} />{/if}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each portfolioSectors as name (name)}
        <tr>
          <td>{name}</td>
          {#each mk.portfolios as p (p.key)}
            {@const w = sectorWeightOf(p, name)}
            <td class="r" class:atcap={w >= mk.sector_max - 1e-4}>{w ? share(w) : ''}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
<p class="sub note">En negrita, los sectores que llegan al tope de {share(mk.sector_max, 0)}.</p>

<h2>Ranking del S&amp;P 500</h2>
<p class="sub">
  Las {cov.ranked} acciones con los tres scores, de mejor a peor score final.
  {#if cov.excluded.sin_momentum.length}Sin un año de precios (no tienen momentum): {cov.excluded.sin_momentum.join(', ')}.{/if}
  {#if cov.excluded.sin_datos_financieros.length}Sin datos financieros: {cov.excluded.sin_datos_financieros.join(', ')}.{/if}
  {#if cov.excluded.sin_datos_tecnicos.length}Sin datos técnicos: {cov.excluded.sin_datos_tecnicos.join(', ')}.{/if}
</p>
<details class="more">
  <summary>Qué significan las columnas</summary>
  <p>
    Porcentajes: ROE, márgenes, flujo de caja libre (FCF) sobre ventas, capitalización y utilidad, y distancia a las
    EMA. «Deuda/Patr.» = deuda sobre patrimonio; «Años FCF+» = parte de los últimos 4 años con flujo libre positivo.
  </p>
</details>
<div class="filters">
  <label>
    Buscar
    <input type="search" bind:value={query} placeholder="ticker o empresa" />
  </label>
  <label>
    Sector
    <select bind:value={sector}>
      <option value="">Todos</option>
      {#each sectors as name}<option value={name}>{name}</option>{/each}
    </select>
  </label>
  <label>
    Detalle
    <select value={view} onchange={(e) => setView(e.currentTarget.value)}>
      {#each Object.entries(views) as [key, v] (key)}<option value={key}>{v.label}</option>{/each}
    </select>
  </label>
  <span class="muted">{rows.length} de {quant.ranking.length}</span>
</div>
<div class="wrap">
  <table>
    <thead>
      <tr>
        {#each columns as column (column.key)}
          <th class:r={column.numeric} aria-sort={ariaSort(column)}>
            <button onclick={() => sortBy(column)}>
              {column.label}{#if sortKey === column.key}<span aria-hidden="true">{descending ? ' ▼' : ' ▲'}</span>{/if}
            </button>{#if column.help}<Hint text={column.help} />{/if}
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each slicePage(rows, page) as r (r.ticker)}
        <tr class:top={r.rank <= s.top_n}>
          {#each columns as column (column.key)}
            {#if column.key === 'ticker'}
              <td><TickerLink ticker={r.ticker} /></td>
            {:else}
              {@const c = cell(r, column.key)}
              <td class={c.cls} title={c.title}>{c.text}</td>
            {/if}
          {/each}
        </tr>
      {:else}
        <tr><td colspan={columns.length} class="muted">Ninguna acción cumple el filtro.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
<Pager total={rows.length} bind:page />

<style>
  .how { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: var(--space-2); margin: var(--space-3) 0; }
  .step {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px var(--space-3);
    box-shadow: var(--shadow); font-size: .85rem;
  }
  .step strong { font-size: .72rem; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  .step p { margin: 4px 0 0; }
  .step summary { cursor: pointer; color: var(--accent); margin-top: 6px; font-size: .8rem; }
  .step ul { margin: 6px 0 0; padding-left: 16px; }
  .step li { margin: 2px 0; }
  .filters { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; margin: 12px 0; }
  label { display: inline-flex; align-items: center; gap: 8px; color: var(--muted); font-size: .9rem; }
  input[type='search'] {
    background: var(--card); color: var(--fg); border: 1px solid var(--line); border-radius: var(--radius-sm);
    padding: 5px 10px; min-height: 32px;
  }
  th button {
    background: none; border: 0; padding: 0; font: inherit; color: inherit; cursor: pointer; white-space: nowrap;
    text-transform: inherit; letter-spacing: inherit;
  }
  th button:hover { color: var(--fg); }
  .narrow { max-width: 760px; }
  h3 { font-size: 1rem; margin: var(--space-4) 0 var(--space-2); }
  td.atcap { font-weight: 700; }
  .note { margin-top: var(--space-2); }
  td.strong { font-weight: 700; }
  td.main { font-weight: 650; background: color-mix(in srgb, var(--accent) 8%, transparent); }
  tfoot td { background: var(--th); font-variant-numeric: tabular-nums; border-top: 1px solid var(--line); }
  tr.top td:first-child { box-shadow: inset 3px 0 0 var(--accent); }
  tr.top { background: color-mix(in srgb, var(--accent) 6%, transparent); }
</style>

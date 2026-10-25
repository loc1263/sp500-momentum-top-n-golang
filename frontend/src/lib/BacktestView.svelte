<script>
  import Chart from './Chart.svelte';
  import Range from './Range.svelte';
  import TickerLink from './TickerLink.svelte';
  import { pct, usd, sign } from './format.js';

  let { universe, settings } = $props();

  let chosen = $state(null); // null = plazo del config
  const term = $derived(
    universe.terms.find((t) => t.months === (chosen ?? settings.months)) ?? universe.terms[0],
  );
  const base = $derived(term.calendars.find((c) => c.offset === 0) ?? term.calendars[0]);
  const s = $derived(term.summary);

  const series = $derived([
    ...term.calendars
      .filter((c) => c !== base)
      .map((c) => ({
        name: `Arranque +${c.offset}`, color: 'var(--muted)', width: 1, opacity: 0.45,
        points: c.equity.map((e) => ({ date: e.date, value: e.capital })),
      })),
    {
      name: 'Calendario base', color: 'var(--accent)', width: 2.5,
      points: base.equity.map((e) => ({ date: e.date, value: e.capital })),
    },
  ]);

  const last = $derived(base.equity.at(-1));
  const pickedDates = $derived(new Set(base.history.map((h) => h.date)));
  const columns = $derived(Math.max(...base.history.map((h) => h.picks.length), 0));
</script>

<div class="toolbar">
  <label>
    Plazo
    <select value={term.months} onchange={(e) => (chosen = Number(e.currentTarget.value))}>
      {#each universe.terms as t}
        <option value={t.months}>{t.months} meses</option>
      {/each}
    </select>
  </label>
  <span class="sub">
    {universe.start_date} a {universe.end_date} ({universe.years} años) · ranking {term.months} meses, rebalanceo cada {term.months} meses ·
    {universe.label}
  </span>
</div>

<div class="cards">
  <div class="card">
    <div class="k">Capital inicial</div>
    <div class="v">US$ {usd(settings.initial_capital)}</div>
    <div class="h">con lo que se empieza</div>
  </div>
  <div class="card">
    <div class="k">Capital final</div>
    <div class="v">US$ {usd(base.final_capital)}</div>
    <div class="h">en qué se convirtió, empezando en {base.first_rebalance}</div>
  </div>
  <div class="card">
    <div class="k">Ganancia total</div>
    <div class="v {sign(base.total_return)}">{pct(base.total_return)}</div>
    <div class="h">acumulada en todo el período</div>
  </div>
  <div class="card">
    <div class="k">Rendimiento anual</div>
    <div class="v {sign(base.cagr)}">{pct(base.cagr)}</div>
    <div class="h">promedio compuesto por año (CAGR), mismo arranque</div>
  </div>
  <div class="card">
    <div class="k">Resultado típico</div>
    <div class="v {sign(s.cagr_median)}">{pct(s.cagr_median)} anual</div>
    <div class="h">el del medio entre {s.calendars} fechas de arranque posibles</div>
  </div>
  <div class="card">
    <div class="k">Según el mes de arranque</div>
    <div class="v">{pct(s.cagr_min, 0)} a {pct(s.cagr_max, 0)}</div>
    <div class="h">anual, del peor al mejor arranque</div>
  </div>
</div>

<h2>Curva de capital</h2>
<Range period={universe} first={base.first_rebalance} />
<p class="sub">En gris, los demás calendarios de arranque posibles del mismo plazo.</p>
<Chart {series} />

<h2>Historial de selecciones (calendario base)</h2>
<Range period={{ ...universe, start_date: base.start_date, end_date: base.end_date }} first={base.first_rebalance} />
<p class="sub">
  Capital = valor al entrar a los tickers de la fila. Gan/Pérd = acumulado vs. capital inicial.
</p>
<div class="wrap">
  <table>
    <thead>
      <tr>
        <th>Fecha</th>
        {#each Array(columns) as _, i}<th>#{i + 1}</th>{/each}
        <th class="r">Capital (US$)</th><th class="r">Ganancia acumulada</th>
      </tr>
    </thead>
    <tbody>
      {#each base.history as h (h.date)}
        <tr>
          <td>{h.date}</td>
          {#each Array(columns) as _, i}
            {@const p = h.picks[i]}
            <td>{#if p}<TickerLink ticker={p.ticker} /> <span class={sign(p.return)}>{pct(p.return)}</span>{/if}</td>
          {/each}
          <td class="r">{usd(h.capital)}</td>
          <td class="r {sign(h.gain)}">{pct(h.gain)}</td>
        </tr>
      {/each}
      {#if last && !pickedDates.has(last.date)}
        <tr>
          <td>{last.date}</td>
          {#each Array(columns) as _}<td></td>{/each}
          <td class="r">{usd(last.capital)}</td>
          <td class="r {sign(last.capital - settings.initial_capital)}">
            {pct(last.capital / settings.initial_capital - 1)}
          </td>
        </tr>
      {/if}
    </tbody>
  </table>
</div>

<h2>Calendarios de arranque posibles ({term.calendars.length})</h2>
<Range period={universe} />
<p class="sub">
  El mismo plazo de {term.months} meses empezando en cada uno de los {term.months} meses posibles.
</p>
<div class="wrap">
  <table>
    <thead>
      <tr>
        <th>Desde</th><th>Hasta</th><th>Primer rebalanceo</th><th class="r">Rebalanceos</th><th class="r">Capital final (US$)</th>
        <th class="r">Ganancia total</th><th class="r">Rendimiento anual</th>
      </tr>
    </thead>
    <tbody>
      {#each term.calendars as c (c.offset)}
        <tr>
          <td>{c.start_date}</td>
          <td>{c.end_date}</td>
          <td>{c.first_rebalance}</td>
          <td class="r">{c.rebalances}</td>
          <td class="r">{usd(c.final_capital)}</td>
          <td class="r {sign(c.total_return)}">{pct(c.total_return)}</td>
          <td class="r {sign(c.cagr)}">{pct(c.cagr)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; margin-bottom: 4px; }
  .toolbar .sub { margin: 0; }
  label { display: inline-flex; align-items: center; gap: 8px; color: var(--muted); font-size: .9rem; }
</style>

<script>
  import Chart from './Chart.svelte';
  import Range from './Range.svelte';
  import Pager from './Pager.svelte';
  import { pct, int, sign, slicePage } from './format.js';

  let { universe, settings } = $props();

  let page = $state(0);

  const baseOf = (t) => t.calendars.find((c) => c.offset === 0) ?? t.calendars[0];
  const rows = $derived(
    universe.terms
      .map((t) => ({ months: t.months, base: baseOf(t), s: t.summary }))
      .sort((a, b) => b.s.cagr_median - a.s.cagr_median),
  );

  // Dos lecturas distintas: la más consistente (mediana) y el mayor resultado de un solo arranque (base)
  const steady = $derived(rows[0]);
  const topBase = $derived([...rows].sort((a, b) => b.base.final_capital - a.base.final_capital)[0]);
  const sameWinner = $derived(steady && topBase && steady.months === topBase.months);

  // Primer rebalanceo efectivo más tardío entre los plazos (el universo puede no tener empresas al inicio)
  const first = $derived(rows.map((r) => r.base.first_rebalance).sort().at(-1));

  // Eje común de las barras de rango (CAGR)
  const lo = $derived(Math.min(...rows.map((r) => r.s.cagr_min)));
  const hi = $derived(Math.max(...rows.map((r) => r.s.cagr_max)));
  const at = (v) => `${((v - lo) / (hi - lo || 1)) * 100}%`;

  const colors = ['var(--s1)', 'var(--s2)', 'var(--s3)', 'var(--s4)', 'var(--s5)', 'var(--s6)'];
  const series = $derived(
    universe.terms.map((t, i) => ({
      name: `${t.months} meses`, color: colors[i],
      points: baseOf(t).equity.map((e) => ({ date: e.date, value: e.capital })),
    })),
  );
</script>

{#if steady}
  <div class="best">
    <p class="line">
      <strong>Plazo más consistente: {steady.months} meses.</strong>
      Su resultado típico es <strong class={sign(steady.s.cagr_median)}>{pct(steady.s.cagr_median)} anual</strong>
      (mediana de {steady.s.calendars} arranques posibles). US$ {int(settings.initial_capital)} terminaron entre
      US$ {int(steady.s.capital_min)} y US$ {int(steady.s.capital_max)}.
    </p>
    {#if sameWinner}
      <p class="line muted">
        También es el que más dinero dejó con el arranque base (US$ {int(topBase.base.final_capital)}).
      </p>
    {:else}
      <p class="line">
        <strong>Mayor resultado con un solo arranque: {topBase.months} meses</strong>
        (US$ {int(topBase.base.final_capital)} con el arranque base), pero con otros arranques terminó entre
        US$ {int(topBase.s.capital_min)} y US$ {int(topBase.s.capital_max)}: es su mejor caso, no lo esperable.
      </p>
    {/if}
  </div>
{/if}

<h2>Comparación de plazos</h2>
<Range period={universe} {first} />
<p class="sub">
  De más a menos consistente. Típico, peor y mejor usan todos los arranques posibles; «un solo arranque» es el primer mes.
</p>
<div class="wrap">
  <table>
    <thead>
      <tr class="groups">
        <th></th>
        <th colspan="2" class="typical">Resultado típico</th>
        <th colspan="2">Peor arranque</th>
        <th colspan="2">Mejor arranque</th>
        <th colspan="2">Un solo arranque (base)</th>
      </tr>
      <tr>
        <th>Plazo</th>
        <th class="r typical">Anual</th><th class="r typical">Capital final</th>
        <th class="r">Anual</th><th class="r">Capital final</th>
        <th class="r">Anual</th><th class="r">Capital final</th>
        <th class="r">Anual</th><th class="r">Capital final</th>
      </tr>
    </thead>
    <tbody>
      {#each slicePage(rows, page) as r (r.months)}
        <tr>
          <td>
            <strong>{r.months} meses</strong>
            {#if r === steady}<span class="badge good">más consistente</span>{/if}
            {#if r === topBase && !sameWinner}<span class="badge">mayor retorno base</span>{/if}
          </td>
          <td class="r typical {sign(r.s.cagr_median)}">{pct(r.s.cagr_median)}</td>
          <td class="r typical">{int(r.s.capital_median)}</td>
          <td class="r {sign(r.s.cagr_min)}">{pct(r.s.cagr_min)}</td>
          <td class="r">{int(r.s.capital_min)}</td>
          <td class="r {sign(r.s.cagr_max)}">{pct(r.s.cagr_max)}</td>
          <td class="r">{int(r.s.capital_max)}</td>
          <td class="r {sign(r.base.cagr)}">{pct(r.base.cagr)}</td>
          <td class="r">{int(r.base.final_capital)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
<Pager total={rows.length} bind:page />
<p class="sub note">
  «Anual» = CAGR. «Capital final» = en qué se convirtieron US$ {int(settings.initial_capital)}.
  Arranques probados: {rows.map((r) => `${r.months}m: ${r.s.calendars}`).join(' · ')}.
</p>

<h2>¿Cuánto depende del mes de arranque?</h2>
<Range period={universe} {first} />
<p class="sub">
  Cada barra va del peor al mejor rendimiento anual; la marca es el resultado típico y el círculo, el arranque base.
  Barra corta = más estable.
</p>
<div class="ranges">
  {#each rows as r (r.months)}
    <div class="rrow">
      <span class="lbl">{r.months} meses</span>
      <div class="track">
        <div class="bar" style:left={at(r.s.cagr_min)} style:width="calc({at(r.s.cagr_max)} - {at(r.s.cagr_min)})"></div>
        <div class="median" style:left={at(r.s.cagr_median)} title="Típico {pct(r.s.cagr_median)}"></div>
        <div class="basedot" style:left={at(r.base.cagr)} title="Arranque base {pct(r.base.cagr)}"></div>
      </div>
      <span class="val">{pct(r.s.cagr_min, 0)} a {pct(r.s.cagr_max, 0)}</span>
    </div>
  {/each}
  <div class="axis"><span>{pct(lo, 0)}</span><span>rendimiento anual</span><span>{pct(hi, 0)}</span></div>
</div>

<h2>Evolución del capital por plazo</h2>
<Range period={universe} {first} />
<p class="sub">Cada línea es el arranque base de un plazo (un solo caso por plazo), no el promedio.</p>
<Chart {series} />

<style>
  .best .line { margin: 0 0 6px; }
  .best .line:last-child { margin-bottom: 0; }
  .typical { background: color-mix(in srgb, var(--accent) 7%, transparent); }
  .groups th { text-align: center; border-bottom: 0; padding-bottom: 0; }
  .badge {
    display: inline-block; margin-left: 6px; padding: 0 6px; border-radius: 10px; font-size: .7rem;
    border: 1px solid var(--line); color: var(--muted); vertical-align: 1px;
  }
  .badge.good { border-color: var(--pos); color: var(--pos); }
  .note { margin-top: 6px; font-size: .8rem; }

  .ranges { border: 1px solid var(--line); border-radius: 8px; padding: 10px 14px; }
  .rrow { display: grid; grid-template-columns: 70px 1fr 120px; align-items: center; gap: 12px; padding: 3px 0; }
  .lbl { font-weight: 600; }
  .val { color: var(--muted); font-size: .85rem; font-variant-numeric: tabular-nums; text-align: right; }
  .track { position: relative; height: 14px; background: var(--card); border-radius: 7px; }
  .bar { position: absolute; top: 3px; height: 8px; border-radius: 4px; background: var(--accent); opacity: .35; }
  .median {
    position: absolute; top: 0; width: 3px; height: 14px; margin-left: -1px; background: var(--accent);
    border-radius: 2px;
  }
  .basedot {
    position: absolute; top: 2px; width: 10px; height: 10px; margin-left: -5px; border-radius: 50%;
    background: var(--bg); border: 2px solid var(--fg);
  }
  .axis {
    display: grid; grid-template-columns: 1fr auto 1fr; margin-top: 4px; padding: 0 132px 0 82px;
    font-size: .75rem; color: var(--muted);
  }
  .axis span:last-child { text-align: right; }

  @media (max-width: 640px) {
    .ranges { padding: 8px 10px; }
    .rrow { grid-template-columns: 58px 1fr 88px; gap: 8px; }
    .val { font-size: .75rem; }
    .axis { padding: 0 96px 0 66px; }
    .axis span:nth-child(2) { display: none; }
    .axis { grid-template-columns: 1fr 1fr; }
  }
</style>

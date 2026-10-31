<script>
  import Hint from './Hint.svelte';
  import Pill from './Pill.svelte';
  import Spark from './Spark.svelte';
  import TickerLink from './TickerLink.svelte';
  import { dec } from './format.js';
  import { VERDICT_LABEL, formatMetric, money, pillText, price } from './evaluator.js';

  let { company, data, onBack } = $props();

  const sectionDefs = $derived(Object.fromEntries(data.sections.map((s) => [s.id, s])));
  const fin = $derived(company.fin_currency ?? company.currency);
  const sections = $derived(company.categories.filter((c) => c.metrics?.length || c.peers || sectionDefs[c.id]));
  const peerLabels = { good: 'Mejor', warn: 'Similar', bad: 'Peor' };
  const peerValue = (row, v) => {
    if (v == null) return '—';
    return row.id.endsWith('pe') ? `${dec(v, 1)}×` : `${dec(v, 1)} %`;
  };
</script>

<div class="head">
  <button class="back" onclick={onBack}>← Volver al listado</button>
  <h3>
    <TickerLink ticker={company.ticker} /> <span class="name">{company.name}</span>
    <Pill level={company.verdict} text={VERDICT_LABEL[company.verdict]} />
  </h3>
  <p class="muted facts">
    {company.sector ?? 'Sin sector'}{company.industry ? ` · ${company.industry}` : ''} ·
    Precio {price(company.price, company.currency)} · Capitalización {money(company.market_cap, company.currency)}
    {#if company.as_of}· Últimos resultados al {company.as_of}{/if}
    {#if company.fin_currency}· Estados financieros en {company.fin_currency}{/if}
  </p>
  {#if company.bank}
    <p class="muted">
      Banco o aseguradora: su deuda y su caja son materia prima del negocio, así que caja, deuda y liquidez no se evalúan.
    </p>
  {/if}
</div>

<div class="grid">
  {#each sections as cat (cat.id)}
    {@const def = sectionDefs[cat.id]}
    <section class="sec" class:wide={cat.id === 'peers' || cat.id === 'trend'} aria-labelledby="sec-{cat.id}">
      <div class="sec-h">
        <h4 id="sec-{cat.id}">{def.title}<Hint text={def.why} /></h4>
        <Pill level={cat.verdict} text={pillText(cat.verdict, '', def.labels)} />
      </div>

      {#if cat.id === 'peers'}
        {#if cat.peers}
          <p class="muted intro">{def.intro}</p>
          <div class="wrap">
            <table>
              <thead>
                <tr>
                  <th>Métrica</th>
                  <th class="r">{company.ticker}</th>
                  {#each cat.peers.tickers as t, i (t)}<th class="r" title={cat.peers.names[i]}>{t}</th>{/each}
                  <th class="r">Mediana</th>
                  <th>Vs.</th>
                </tr>
              </thead>
              <tbody>
                {#each cat.peers.rows as row (row.id)}
                  <tr>
                    <td>{data.metrics[row.id].name}<Hint text={data.metrics[row.id].why} /></td>
                    <td class="r me" class:best={row.best === -1}>{peerValue(row, row.mine)}</td>
                    {#each row.values as v, i (i)}<td class="r" class:best={row.best === i}>{peerValue(row, v)}</td>{/each}
                    <td class="r">{peerValue(row, row.median)}</td>
                    <td><Pill level={row.l} text={row.l === 'na' ? '—' : peerLabels[row.l]} /></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else}
          <p class="muted">No hay otras empresas de la misma industria o sector en este mercado para comparar.</p>
        {/if}
      {:else if !cat.metrics?.length}
        <p class="muted">Sin datos suficientes de Yahoo Finance.</p>
      {:else}
        <ul class="rows">
          {#each cat.metrics as m (m.id)}
            {@const md = data.metrics[m.id]}
            <li>
              <div class="row">
                <span class="mname">{md.name}<Hint text={md.why} /></span>
                <span class="mval">{formatMetric(m, md, fin)}</span>
                <Pill level={m.l} text={pillText(m.l, m.lb, def.labels)} />
              </div>
              {#if m.s}<Spark values={m.s} years={cat.years} label={md.name} />{/if}
              {#if m.lo != null}
                <div class="rng" role="img" aria-label="De {dec(m.lo, 2)} a {dec(m.hi, 2)}">
                  <div class="track"><span class="dot" style="left:{Math.min(100, Math.max(0, m.v))}%"></span></div>
                  <div class="lbls"><span>{dec(m.lo, 2)}</span><span>{dec(m.hi, 2)}</span></div>
                </div>
              {/if}
              {#if m.n}<p class="note muted">{m.n}</p>{/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/each}
</div>

<style>
  .head h3 { margin: var(--space-3) 0 var(--space-1); font-size: 1.15rem; display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2); }
  .name { font-weight: 500; }
  .facts { margin: 0 0 var(--space-2); font-size: .85rem; }
  .back { background: var(--card); border: 1px solid var(--line); border-radius: var(--radius-sm); padding: 3px 10px; cursor: pointer; }
  .back:hover { border-color: var(--muted); }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: var(--space-3); margin-top: var(--space-3); }
  .sec { background: var(--card); border: 1px solid var(--line); border-radius: var(--radius); padding: var(--space-3) var(--space-4); box-shadow: var(--shadow); min-width: 0; }
  .wide { grid-column: 1 / -1; }
  .sec-h { display: flex; justify-content: space-between; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
  h4 { margin: 0; font-size: .8rem; letter-spacing: .07em; text-transform: uppercase; }
  .intro { font-size: .82rem; margin: 0 0 var(--space-2); }
  .rows { list-style: none; margin: 0; padding: 0; }
  .rows li { padding: 6px 0; border-top: 1px solid var(--line); }
  .rows li:first-child { border-top: 0; }
  .row { display: grid; grid-template-columns: 1fr auto auto; gap: var(--space-2); align-items: center; }
  .mname { font-size: .85rem; }
  .mval { font-variant-numeric: tabular-nums; font-weight: 650; text-align: right; }
  .note { font-size: .78rem; margin: 2px 0 0; }
  .me { font-weight: 650; }
  td.best { color: var(--pos); font-weight: 650; }
  .rng { max-width: 300px; margin-top: 4px; }
  .track { position: relative; height: 6px; border-radius: 3px; background: linear-gradient(90deg, color-mix(in srgb, var(--neg) 45%, var(--card)), color-mix(in srgb, var(--s4) 45%, var(--card)), color-mix(in srgb, var(--pos) 45%, var(--card))); }
  .dot { position: absolute; top: -3px; width: 12px; height: 12px; margin-left: -6px; border-radius: 50%; background: var(--fg); border: 2px solid var(--card); }
  .lbls { display: flex; justify-content: space-between; font-size: .68rem; color: var(--muted); font-variant-numeric: tabular-nums; }
</style>

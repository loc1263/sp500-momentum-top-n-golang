<script>
  import TickerLink from './TickerLink.svelte';
  import Pager from './Pager.svelte';
  import { pct, share, usd, billions, sign, slicePage } from './format.js';

  let { actual } = $props();

  const weight = $derived(actual.picks.length ? 1 / actual.picks.length : 0);
  const from = $derived.by(() => {
    const d = new Date(`${actual.as_of}T00:00:00Z`);
    d.setUTCMonth(d.getUTCMonth() - actual.months);
    return d.toISOString().slice(0, 10);
  });
  let pickPage = $state(0);
  let altPage = $state(0);
  let trackPage = $state(0);
  const tracking = $derived(actual.tracking ?? []);
  const list = (tickers) => (tickers.length ? tickers.join(', ') : '-');
</script>

<div class="cards">
  <div class="card"><div class="k">Datos al cierre del</div><div class="v">{actual.as_of}</div></div>
  <div class="card"><div class="k">Plazo</div><div class="v">{actual.months} meses</div></div>
  <div class="card"><div class="k">Acciones</div><div class="v">{actual.picks.length}</div></div>
  <div class="card"><div class="k">Peso por acción</div><div class="v">{share(weight)}</div></div>
  {#if actual.next_rebalance}
    <div class="card">
      <div class="k">Próximo rebalanceo</div>
      <div class="v">{actual.next_rebalance}</div>
      <div class="h">cada {actual.months} meses desde la primera corrida guardada ({actual.anchor})</div>
    </div>
  {/if}
</div>

<h2>Top {actual.picks.length} de los últimos {actual.months} meses (modelo N-0)</h2>
<p class="range">
  <strong>Ventana de retorno:</strong> {from} a {actual.as_of}
  <span class="muted">({actual.months} meses)</span>
</p>
<p class="sub">
  Ponderación igual, rebalanceo cada {actual.months} meses. Cap. aprox. = precio × acciones en circulación hoy.
</p>
{#if actual.changes}
  <p class="changes">
    <strong>Cambios vs. {actual.previous_as_of}:</strong>
    {#if !actual.changes.entered.length && !actual.changes.exited.length}
      ninguno, la selección se mantiene.
    {:else}
      entran <span class="pos">{list(actual.changes.entered)}</span> ·
      salen <span class="neg">{list(actual.changes.exited)}</span> ·
      se mantienen {list(actual.changes.kept)}
    {/if}
  </p>
{:else if actual.previous_as_of === null}
  <p class="muted">Primera corrida guardada: todavía no hay una selección anterior para comparar.</p>
{/if}
<div class="wrap">
  <table>
    <thead>
      <tr>
        <th>#</th><th>Ticker</th><th class="r">Retorno {actual.months} meses</th>
        <th class="r">Cap. aprox. (US$ B)</th><th class="r">Precio (US$)</th><th class="r">Peso</th>
      </tr>
    </thead>
    <tbody>
      {#each slicePage(actual.picks, pickPage) as p (p.ticker)}
        <tr>
          <td>{p.rank}</td>
          <td><TickerLink ticker={p.ticker} /></td>
          <td class="r {sign(p.return)}">{pct(p.return)}</td>
          <td class="r">{billions(p.market_cap)}</td>
          <td class="r">{usd(p.price)}</td>
          <td class="r">{share(weight)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
<Pager total={actual.picks.length} bind:page={pickPage} />

{#if actual.alternates?.length}
  <h2>Suplentes</h2>
  <p class="sub">
    Los puestos siguientes al top {actual.picks.length}, como reemplazo posible.
  </p>
  <div class="wrap">
    <table>
      <thead>
        <tr>
          <th>#</th><th>Ticker</th><th class="r">Retorno {actual.months} meses</th>
          <th class="r">Cap. aprox. (US$ B)</th><th class="r">Precio (US$)</th>
        </tr>
      </thead>
      <tbody>
        {#each slicePage(actual.alternates, altPage) as p (p.ticker)}
          <tr>
            <td>{p.rank}</td>
            <td><TickerLink ticker={p.ticker} /></td>
            <td class="r {sign(p.return)}">{pct(p.return)}</td>
            <td class="r">{billions(p.market_cap)}</td>
            <td class="r">{usd(p.price)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pager total={actual.alternates.length} bind:page={altPage} />
{/if}

{#if tracking.length}
  <h2>Seguimiento de selecciones anteriores</h2>
  <p class="sub">
    Cómo le fue a cada selección guardada desde que se eligió. Dura {actual.months} meses; después figura como cerrada.
    Con pocas selecciones guardadas todavía no prueba nada.
  </p>
  <div class="wrap">
    <table>
      <thead>
        <tr>
          <th>Elegida el</th><th>Medida hasta</th><th class="r">Días</th><th>Estado</th>
          <th>Acciones</th><th class="r">Retorno (ponderación igual)</th>
        </tr>
      </thead>
      <tbody>
        {#each slicePage(tracking, trackPage) as t (t.as_of)}
          <tr>
            <td>{t.as_of}</td>
            <td>{t.until}</td>
            <td class="r">{t.days}</td>
            <td>{t.closed ? 'cerrada' : 'en curso'}</td>
            <td>
              {#each t.picks as p, i (p.ticker)}{i ? ', ' : ''}{p.ticker}
                <span class={sign(p.return)}>{pct(p.return)}</span>{/each}
            </td>
            <td class="r {sign(t.return)}">{pct(t.return)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pager total={tracking.length} bind:page={trackPage} />
{/if}

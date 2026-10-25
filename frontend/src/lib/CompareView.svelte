<script>
  import Chart from './Chart.svelte';
  import Range from './Range.svelte';
  import Pager from './Pager.svelte';
  import { pct, int, sign, slicePage } from './format.js';

  /** `universe` = modelo de siempre (N-0), `skip` = modelo skip-month (N-1); ambos con la forma { terms, ... }. */
  let { universe, skip, settings } = $props();

  let rowsPage = $state(0);
  let halfPage = $state(0);

  const baseOf = (t) => t.calendars.find((c) => c.offset === 0) ?? t.calendars[0];

  // Un renglón por plazo que tenga resultado en los dos modelos
  const rows = $derived(
    universe.terms
      .map((t) => ({ months: t.months, a: t, b: skip?.terms.find((x) => x.months === t.months) }))
      .filter((r) => r.b)
      .map((r) => {
        const diff = r.b.summary.cagr_median - r.a.summary.cagr_median;
        return { ...r, diff, winner: diff > 0 ? 'b' : diff < 0 ? 'a' : null };
      })
      .sort((x, y) => x.months - y.months),
  );

  const winsSkip = $derived(rows.filter((r) => r.winner === 'b').length);
  const winsBase = $derived(rows.filter((r) => r.winner === 'a').length);
  const avgDiff = $derived(rows.length ? rows.reduce((s, r) => s + r.diff, 0) / rows.length : 0);

  // Mitades del período: ¿el ganador se sostiene en las dos?
  const winnerOf = (a, b) => (a == null || b == null || a === b ? null : b > a ? 'N-1' : 'N-0');
  const halfRows = $derived(
    rows.map((r) => {
      const ha = r.a.summary.halves;
      const hb = r.b.summary.halves;
      const first = winnerOf(ha?.first, hb?.first);
      const second = winnerOf(ha?.second, hb?.second);
      return { ...r, ha, hb, first, second, steady: first && first === second ? first : null };
    }),
  );
  const halvesAvailable = $derived(halfRows.some((r) => r.first || r.second));
  const steadyBase = $derived(halfRows.filter((r) => r.steady === 'N-0').length);
  const steadySkip = $derived(halfRows.filter((r) => r.steady === 'N-1').length);
  const split = $derived(halfRows.find((r) => r.ha?.split)?.ha.split);

  let chosen = $state(null); // null = plazo del config
  const row = $derived(rows.find((r) => r.months === (chosen ?? settings.months)) ?? rows[0]);
  const series = $derived(
    row
      ? [
          { name: `${row.months}-0 (actual)`, color: 'var(--accent)', width: 2.5,
            points: baseOf(row.a).equity.map((e) => ({ date: e.date, value: e.capital })) },
          { name: `${row.months}-1 (skip-month)`, color: 'var(--s2)', width: 2.5,
            points: baseOf(row.b).equity.map((e) => ({ date: e.date, value: e.capital })) },
        ]
      : [],
  );
  const first = $derived(rows.map((r) => baseOf(r.a).first_rebalance).sort().at(-1));
</script>

{#if !rows.length}
  <p class="muted">
    Sin resultados del modelo skip-month para este universo y período. Ejecute top3 backtest --sweep (en backend-go/).
  </p>
{:else}
  <div class="best">
    <p class="line">
      <strong>
        {#if winsSkip === winsBase}Empate: cada modelo gana en {winsBase} de {rows.length} plazos.
        {:else if winsSkip > winsBase}El skip-month (N-1) gana en {winsSkip} de {rows.length} plazos.
        {:else}El modelo actual (N-0) gana en {winsBase} de {rows.length} plazos.{/if}
      </strong>
      Diferencia media del resultado típico (N-1 menos N-0):
      <strong class={sign(avgDiff)}>{pct(avgDiff, 1)} anual</strong>.
    </p>
    <p class="line muted">
      N-0 ordena por lo que subió cada acción en los últimos N meses; N-1 ignora el último mes para no premiar rebotes
      de corto plazo. Mismos datos, arranques y universo.
    </p>
  </div>

  <h2>Plazo por plazo</h2>
  <Range period={universe} {first} />
  <div class="wrap">
    <table>
      <thead>
        <tr class="groups">
          <th></th>
          <th colspan="2" class="typical">Resultado típico (anual)</th>
          <th></th>
          <th colspan="2">Peor / mejor arranque</th>
          <th colspan="2">Capital final típico</th>
          <th></th>
        </tr>
        <tr>
          <th>Plazo</th>
          <th class="r typical">N-0</th><th class="r typical">N-1</th>
          <th class="r">Diferencia</th>
          <th class="r">N-0</th><th class="r">N-1</th>
          <th class="r">N-0</th><th class="r">N-1</th>
          <th>Gana</th>
        </tr>
      </thead>
      <tbody>
        {#each slicePage(rows, rowsPage) as r (r.months)}
          <tr>
            <td><strong>{r.months} meses</strong></td>
            <td class="r typical {sign(r.a.summary.cagr_median)}">{pct(r.a.summary.cagr_median)}</td>
            <td class="r typical {sign(r.b.summary.cagr_median)}">{pct(r.b.summary.cagr_median)}</td>
            <td class="r {sign(r.diff)}">{pct(r.diff)}</td>
            <td class="r">{pct(r.a.summary.cagr_min, 0)} a {pct(r.a.summary.cagr_max, 0)}</td>
            <td class="r">{pct(r.b.summary.cagr_min, 0)} a {pct(r.b.summary.cagr_max, 0)}</td>
            <td class="r">{int(r.a.summary.capital_median)}</td>
            <td class="r">{int(r.b.summary.capital_median)}</td>
            <td>
              {#if r.winner}<span class="badge good">{r.winner === 'b' ? 'N-1' : 'N-0'}</span>{:else}-{/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pager total={rows.length} bind:page={rowsPage} />
  <p class="sub note">
    «Típico» = mediana de los arranques posibles, con US$ {int(settings.initial_capital)} iniciales. Una diferencia
    pequeña en «Gana» no prueba que un modelo sea mejor.
  </p>

  {#if halvesAvailable}
    <h2>¿Se sostiene en el tiempo?</h2>
    <Range period={universe} {first} />
    <p class="sub">
      El período se parte en dos mitades (corte cerca del {split}). Si un modelo gana en las dos, su ventaja no depende de
      un solo tramo del mercado.
      <strong>
        N-0 gana las dos mitades en {steadyBase} de {rows.length} plazos, N-1 en {steadySkip};
        el resto se reparte.
      </strong>
    </p>
    <div class="wrap">
      <table>
        <thead>
          <tr class="groups">
            <th></th>
            <th colspan="3">Primera mitad (anual)</th>
            <th colspan="3">Segunda mitad (anual)</th>
            <th></th>
          </tr>
          <tr>
            <th>Plazo</th>
            <th class="r">N-0</th><th class="r">N-1</th><th>Gana</th>
            <th class="r">N-0</th><th class="r">N-1</th><th>Gana</th>
            <th>Se sostiene</th>
          </tr>
        </thead>
        <tbody>
          {#each slicePage(halfRows, halfPage) as r (r.months)}
            <tr>
              <td><strong>{r.months} meses</strong></td>
              <td class="r {sign(r.ha?.first)}">{pct(r.ha?.first)}</td>
              <td class="r {sign(r.hb?.first)}">{pct(r.hb?.first)}</td>
              <td>{r.first ?? '-'}</td>
              <td class="r {sign(r.ha?.second)}">{pct(r.ha?.second)}</td>
              <td class="r {sign(r.hb?.second)}">{pct(r.hb?.second)}</td>
              <td>{r.second ?? '-'}</td>
              <td>
                {#if r.steady}<span class="badge good">{r.steady} en las dos</span>{:else}<span class="muted">no</span>{/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <Pager total={halfRows.length} bind:page={halfPage} />
    <p class="sub note">
      Con períodos cortos cada mitad tiene pocos rebalanceos: mejor mirar 6 años o más.
    </p>
  {/if}

  <h2>Evolución del capital</h2>
  <div class="toolbar">
    <label>
      Plazo
      <select value={row.months} onchange={(e) => (chosen = Number(e.currentTarget.value))}>
        {#each rows as r (r.months)}
          <option value={r.months}>{r.months} meses</option>
        {/each}
      </select>
    </label>
  </div>
  <p class="sub">Arranque base de cada modelo (un solo caso por modelo), no el promedio.</p>
  <Chart {series} />
{/if}

<style>
  .best .line { margin: 0 0 6px; }
  .best .line:last-child { margin-bottom: 0; }
  .typical { background: color-mix(in srgb, var(--accent) 7%, transparent); }
  .groups th { text-align: center; border-bottom: 0; padding-bottom: 0; }
  .badge {
    display: inline-block; padding: 0 6px; border-radius: 10px; font-size: .7rem;
    border: 1px solid var(--line); color: var(--muted);
  }
  .badge.good { border-color: var(--pos); color: var(--pos); }
  .note { margin-top: 6px; font-size: .8rem; }
  .toolbar { margin: 8px 0; }
</style>

<script>
  import ActualView from './lib/ActualView.svelte';
  import BacktestView from './lib/BacktestView.svelte';
  import SweepView from './lib/SweepView.svelte';
  import CompareView from './lib/CompareView.svelte';
  import EtfView from './lib/EtfView.svelte';
  import QuantView from './lib/QuantView.svelte';
  import AboutView from './lib/AboutView.svelte';
  import Glossary from './lib/Glossary.svelte';
  import Bunny from './lib/Bunny.svelte';
  import { share } from './lib/format.js';

  // Las cuatro primeras son la misma estrategia de momentum; ETFs y Modelo por score son análisis aparte
  const groups = [
    {
      label: 'Estrategia de momentum',
      tabs: [
        { id: 'actual', label: 'Top actual' },
        { id: 'backtest', label: 'Backtest' },
        { id: 'sweep', label: 'Plazos y sensibilidad' },
        { id: 'compare', label: 'N-0 vs N-1' },
      ],
    },
    { label: 'ETFs', tabs: [{ id: 'etf', label: 'ETFs Vanguard' }] },
    { label: 'Acciones', tabs: [{ id: 'quant', label: 'Modelo por score' }] },
  ];
  const tabs = groups.flatMap((g) => g.tabs);

  // About no está en el menú: es un botón aparte, en la esquina opuesta
  const initialTab = () =>
    [...tabs.map((t) => t.id), 'about'].includes(location.hash.slice(1)) ? location.hash.slice(1) : 'actual';

  let tab = $state(initialTab());
  let capKey = $state('0');
  let chosenYears = $state(null); // null = período por defecto del config
  let report = $state(null);
  let error = $state(null);
  let dark = $state(document.documentElement.dataset.theme === 'dark');

  function toggleTheme() {
    dark = !dark;
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
    try {
      localStorage.setItem('theme', dark ? 'dark' : 'light');
    } catch {
      /* el tema vale solo para esta sesión */
    }
  }

  // La página no pide nada al backend: lee el JSON estático que dejan el backend en Go al terminar
  async function load() {
    try {
      const res = await fetch('data/report.json', { cache: 'no-cache' });
      if (res.ok && (res.headers.get('content-type') ?? '').includes('json')) return await res.json();
    } catch {
      /* se informa abajo */
    }
    throw new Error('No se pudo cargar data/report.json. Ejecute go run ./cmd/top3 pipeline en backend-go/ y recargue.');
  }

  $effect(() => {
    load().then(
      (r) => {
        // Universo por defecto: el de mayor capitalización mínima (mega-cap)
        const all = [...r.actual, ...r.backtest];
        if (all.length) capKey = key(all.reduce((m, u) => ((u.min_cap_billions ?? 0) > (m.min_cap_billions ?? 0) ? u : m)));
        report = r;
      },
      (e) => (error = e.message),
    );
  });

  $effect(() => {
    history.replaceState(null, '', `#${tab}`);
  });

  const key = (u) => String(u.min_cap_billions ?? 0);
  const universes = $derived(
    report ? [...new Map([...report.actual, ...report.backtest].map((u) => [key(u), u])).values()] : [],
  );
  const actual = $derived(report?.actual.find((a) => key(a) === capKey));
  const universeData = $derived(report?.backtest.find((u) => key(u) === capKey));
  const periods = $derived(universeData?.periods ?? []);
  const period = $derived(
    periods.find((p) => p.years === (chosenYears ?? report?.settings.backtest_years)) ?? periods.at(-1),
  );
  // Lo que reciben las vistas: el universo con el período elegido
  const universe = $derived(
    universeData && period ? { label: universeData.label, ...period } : null,
  );
  const skipPeriod = $derived(
    report?.backtest_skip?.find((u) => key(u) === capKey)?.periods.find((p) => p.years === period?.years),
  );
  const label = $derived(universes.find((u) => key(u) === capKey)?.label ?? '');
</script>

<main>
  <header>
    <div class="corner">
      <button class="home" class:active={tab === 'actual'} aria-pressed={tab === 'actual'} onclick={() => (tab = 'actual')}>
        Home
      </button>
      <button class="about" class:active={tab === 'about'} aria-pressed={tab === 'about'} onclick={() => (tab = 'about')}>
        About
      </button>
      <button class="theme" onclick={toggleTheme} aria-label="Cambiar tema claro/oscuro" title="Cambiar tema">
        {dark ? '☀️ Claro' : '🌙 Oscuro'}
      </button>
    </div>
    {#if tab === 'etf'}
      <h1>ETFs de Vanguard</h1>
    {:else if tab === 'quant'}
      <h1>Modelo por score</h1>
    {:else if tab === 'about'}
      <h1>About</h1>
    {:else}
      <h1>Top {report?.settings.top_n ?? 'N'} del S&amp;P 500</h1>
    {/if}
    <p class="sub">
      {#if tab === 'etf'}
        Momentum ajustado por riesgo: ranking de los ETFs de Vanguard por cuánto subieron por unidad de riesgo.
      {:else if tab === 'quant'}
        Scores de momentum, salud financiera y técnico de las acciones del S&amp;P 500, su promedio y Markowitz sobre las mejores.
      {:else if tab === 'about'}
        Qué se muestra en esta aplicación y cómo leerlo.
      {:else}
        Momentum: cada N meses se compran, con ponderación igual, las acciones que más subieron en los últimos N meses.
      {/if}
      {#if report}
        Corrida del {report.run_date ?? 'sin datos'}.
      {/if}
    </p>
  </header>

  {#if error}
    <p class="best error">{error}</p>
  {:else if !report}
    <p class="muted">Cargando…</p>
  {:else}
    <nav>
      <div class="tabs" role="tablist">
        {#each groups as g (g.label)}
          <div class="group" role="presentation">
            <span class="cap" class:on={g.tabs.some((t) => t.id === tab)}>{g.label}</span>
            <div class="row" role="presentation">
              {#each g.tabs as t (t.id)}
                <button role="tab" aria-selected={tab === t.id} class:active={tab === t.id} onclick={() => (tab = t.id)}>
                  {t.label}
                </button>
              {/each}
            </div>
          </div>
        {/each}
      </div>
      <div class="filters">
        {#if tab !== 'actual' && tab !== 'etf' && tab !== 'quant' && tab !== 'about' && periods.length}
          <label>
            Período
            <select value={period?.years} onchange={(e) => (chosenYears = Number(e.currentTarget.value))}>
              {#each periods as p (p.years)}
                <option value={p.years}>
                  {p.years} años ({p.start_date.slice(0, 4)}–{p.end_date.slice(0, 4)})
                </option>
              {/each}
            </select>
          </label>
        {/if}
        {#if tab !== 'etf' && tab !== 'quant' && tab !== 'about'}
          <label>
            Universo
            <select bind:value={capKey}>
              {#each universes as u (key(u))}
                <option value={key(u)}>{u.label}</option>
              {/each}
            </select>
          </label>
        {/if}
      </div>
    </nav>

    {#if tab === 'backtest' || tab === 'sweep' || tab === 'compare'}<Glossary />{/if}

    {#if tab === 'about'}
      <AboutView settings={report.settings} />
    {:else if tab === 'etf'}
      {#if report.etf}
        <EtfView etf={report.etf} />
      {:else}
        <p class="muted">Sin ranking de ETFs. Ejecute top3 etf (en backend-go/).</p>
      {/if}
    {:else if tab === 'quant'}
      {#if report.quant}
        <QuantView quant={report.quant} />
      {:else}
        <p class="muted">Sin modelo por score. Ejecute top3 quant (en backend-go/).</p>
      {/if}
    {:else if tab === 'actual'}
      {#if actual}
        <ActualView {actual} />
      {:else}
        <p class="muted">Sin Top actual para «{label}». Ejecute top3 actual (en backend-go/).</p>
      {/if}
    {:else if !universe}
      <p class="muted">Sin backtest para «{label}». Ejecute top3 backtest --sweep (en backend-go/).</p>
    {:else if tab === 'backtest'}
      {#key `${capKey}-${period.years}`}<BacktestView {universe} settings={report.settings} />{/key}
    {:else if tab === 'compare'}
      <CompareView {universe} skip={skipPeriod} settings={report.settings} />
    {:else}
      <SweepView {universe} settings={report.settings} />
    {/if}

    <footer>
      Generado el {report.generated_at.replace('T', ' ')}. {#if tab === 'quant'}Modelo por score: foto de hoy, sin backtest; los datos financieros son los últimos publicados (Yahoo Finance). {/if}&nbsp;Backtest con comisión de {share(report.settings.commission ?? 0, 2)} por lado, sin impuestos, ponderación igual
      entre los tickers seleccionados. Excluye tickers sin datos en Yahoo (sesgo de supervivencia). Herramienta de
      análisis, no una recomendación de inversión.
    </footer>
  {/if}
</main>

<Bunny />

<style>
  main { max-width: 1280px; margin: 0 auto; padding: var(--space-4) var(--space-4) 48px; }
  header { position: relative; padding-right: 250px; min-height: 40px; }
  header .sub { margin-bottom: var(--space-3); }
  .corner { position: absolute; top: 0; right: 0; display: flex; gap: var(--space-2); }
  .corner button {
    background: var(--card); color: var(--fg); border: 1px solid var(--line); border-radius: var(--radius-sm);
    padding: 3px 10px; font-size: .8rem; cursor: pointer; box-shadow: var(--shadow);
    transition: border-color .12s ease, color .12s ease;
  }
  .corner button:hover { border-color: var(--muted); }
  .corner .home.active,
  .corner .about.active { border-color: var(--accent); color: var(--accent); font-weight: 600; }

  /* La navegación queda fija arriba al desplazarse por tablas largas */
  nav {
    position: sticky; top: 0; z-index: 10; display: flex; flex-wrap: wrap; justify-content: space-between;
    align-items: flex-end; gap: var(--space-2) var(--space-4); margin: 0 calc(var(--space-4) * -1) var(--space-2);
    padding: var(--space-2) var(--space-4) 0; background: color-mix(in srgb, var(--bg) 90%, transparent);
    backdrop-filter: blur(8px);
  }
  .tabs { display: flex; gap: var(--space-4); border-bottom: 1px solid var(--line); flex: 1 1 auto; overflow-x: auto; scrollbar-width: none; }
  .group { display: flex; flex-direction: column; flex: none; }
  .group + .group { border-left: 1px solid var(--line); padding-left: var(--space-4); }
  .cap {
    font-size: .64rem; font-weight: 600; text-transform: uppercase; letter-spacing: .08em; color: var(--muted);
    padding: 0 12px; opacity: .8;
  }
  .cap.on { color: var(--accent); opacity: 1; }
  .row { display: flex; gap: 2px; }
  .tabs::-webkit-scrollbar { display: none; }
  .tabs button {
    background: none; border: 0; border-bottom: 2px solid transparent; color: var(--muted); cursor: pointer;
    padding: 7px 12px; margin-bottom: -1px; font-size: .88rem; white-space: nowrap; border-radius: var(--radius-sm) var(--radius-sm) 0 0;
    transition: color .12s ease, background-color .12s ease, border-color .12s ease;
  }
  .tabs button:hover { color: var(--fg); background: var(--accent-soft); }
  .tabs button.active { color: var(--fg); border-bottom-color: var(--accent); font-weight: 600; }
  .filters { display: flex; flex-wrap: wrap; gap: var(--space-2) var(--space-4); padding-bottom: 6px; }
  nav label { display: inline-flex; align-items: center; gap: var(--space-2); color: var(--muted); font-size: .88rem; }
  .error { border-left-color: var(--neg); }
  footer {
    margin-top: 40px; padding-top: var(--space-4); border-top: 1px solid var(--line); color: var(--muted);
    font-size: .8rem; max-width: 100ch;
  }

  @media (max-width: 720px) {
    header { padding-right: 0; padding-top: 40px; }
    nav { position: static; margin: 0 0 var(--space-2); padding: 0; backdrop-filter: none; background: none; }
    .filters { width: 100%; }
    /* Sin scroll horizontal ni rótulos: todas las pestañas fluyen juntas y los grupos se separan con una línea */
    .tabs { flex-wrap: wrap; overflow: visible; gap: 0; border-bottom: 1px solid var(--line); }
    .group { display: contents; }
    .cap { display: none; }
    .row { display: contents; }
    .group:not(:first-child) > .row > button:first-child { border-left: 1px solid var(--line); border-radius: 0; }
    .tabs button { padding: 6px 9px; font-size: .82rem; }
  }
</style>

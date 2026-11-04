<script>
  import BacktestView from './lib/BacktestView.svelte';
  import SweepView from './lib/SweepView.svelte';
  import CompareView from './lib/CompareView.svelte';
  import Glossary from './lib/Glossary.svelte';
  import { share } from './lib/format.js';
  import { loadReport, capKeyOf as key } from './lib/report.js';

  // Página aparte (lab.html): el laboratorio de backtest no es para el usuario final, por eso no está en el menú principal
  const tabs = [
    { id: 'backtest', label: 'Backtest', title: 'Backtest' },
    { id: 'sweep', label: 'Plazos y sensibilidad', title: 'Plazos y sensibilidad' },
    { id: 'compare', label: 'N-0 vs N-1', title: 'N-0 vs N-1' },
  ];

  let tab = $state(tabs.some((t) => t.id === location.hash.slice(1)) ? location.hash.slice(1) : 'backtest');
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

  $effect(() => {
    loadReport().then(
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

  const SITE_NAME = 'Momentum y Scores';
  const sectionTitle = $derived(`Lab: ${tabs.find((t) => t.id === tab).title}`);
  $effect(() => {
    document.title = `${SITE_NAME} · ${sectionTitle}`;
  });

  const universes = $derived(
    report ? [...new Map([...report.actual, ...report.backtest].map((u) => [key(u), u])).values()] : [],
  );
  const universeData = $derived(report?.backtest.find((u) => key(u) === capKey));
  const periods = $derived(universeData?.periods ?? []);
  const period = $derived(
    periods.find((p) => p.years === (chosenYears ?? report?.settings.backtest_years)) ?? periods.at(-1),
  );
  // Lo que reciben las vistas: el universo con el período elegido
  const universe = $derived(universeData && period ? { label: universeData.label, ...period } : null);
  const skipPeriod = $derived(
    report?.backtest_skip?.find((u) => key(u) === capKey)?.periods.find((p) => p.years === period?.years),
  );
  const label = $derived(universes.find((u) => key(u) === capKey)?.label ?? '');
</script>

<main>
  <header>
    <div class="corner">
      <a class="home" href="./">Volver a la app</a>
      <button class="theme" onclick={toggleTheme} aria-label="Cambiar tema claro/oscuro" title="Cambiar tema">
        {dark ? '☀️ Claro' : '🌙 Oscuro'}
      </button>
    </div>
    <h1>{SITE_NAME}</h1>
    <h2 class="section">{sectionTitle}</h2>
    <p class="sub">
      Laboratorio de backtest. Momentum: cada N meses se compran, con ponderación igual, las acciones que más subieron en los últimos N meses.
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
        {#each tabs as t (t.id)}
          <button role="tab" aria-selected={tab === t.id} class:active={tab === t.id} onclick={() => (tab = t.id)}>
            {t.label}
          </button>
        {/each}
      </div>
      <div class="filters">
        {#if periods.length}
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
        <label>
          Universo
          <select bind:value={capKey}>
            {#each universes as u (key(u))}
              <option value={key(u)}>{u.label}</option>
            {/each}
          </select>
        </label>
      </div>
    </nav>

    <Glossary />

    {#if !universe}
      <p class="muted">Sin backtest para «{label}». Ejecute top3 backtest --sweep (en backend-go/).</p>
    {:else if tab === 'backtest'}
      {#key `${capKey}-${period.years}`}<BacktestView {universe} settings={report.settings} />{/key}
    {:else if tab === 'compare'}
      <CompareView {universe} skip={skipPeriod} settings={report.settings} />
    {:else}
      <SweepView {universe} settings={report.settings} />
    {/if}

    <footer>
      Generado el {report.generated_at.replace('T', ' ')}. Backtest con comisión de {share(report.settings.commission ?? 0, 2)} por lado,
      sin impuestos, ponderación igual entre los tickers seleccionados. Excluye tickers sin datos en Yahoo (sesgo de
      supervivencia). Herramienta de análisis, no una recomendación de inversión.
    </footer>
  {/if}
</main>

<style>
  main { max-width: 1280px; margin: 0 auto; padding: var(--space-4) var(--space-4) 48px; }
  header { position: relative; padding-right: 250px; min-height: 40px; }
  header .sub { margin-bottom: var(--space-3); }
  header .section {
    font-size: 1.05rem; font-weight: 600; letter-spacing: normal; text-transform: none; margin: 0 0 var(--space-1);
    padding: 0; border: 0; color: var(--accent);
  }
  .corner { position: absolute; top: 0; right: 0; display: flex; gap: var(--space-2); }
  .corner button,
  .corner a {
    background: var(--card); color: var(--fg); border: 1px solid var(--line); border-radius: var(--radius-sm);
    padding: 3px 10px; font-size: .8rem; cursor: pointer; box-shadow: var(--shadow); text-decoration: none;
    transition: border-color .12s ease, color .12s ease;
  }
  .corner button:hover,
  .corner a:hover { border-color: var(--muted); }

  nav {
    position: sticky; top: 0; z-index: 10; display: flex; flex-wrap: wrap; justify-content: space-between;
    align-items: flex-end; gap: var(--space-2) var(--space-4); margin: 0 calc(var(--space-4) * -1) var(--space-2);
    padding: var(--space-2) var(--space-4) 0; background: color-mix(in srgb, var(--bg) 90%, transparent);
    backdrop-filter: blur(8px);
  }
  .tabs { display: flex; gap: 2px; border-bottom: 1px solid var(--line); flex: 1 1 auto; overflow-x: auto; scrollbar-width: none; }
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
    .tabs { flex-wrap: wrap; overflow: visible; }
    .tabs button { padding: 6px 9px; font-size: .82rem; }
  }
</style>

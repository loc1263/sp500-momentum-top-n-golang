<script>
  import { int, share } from './format.js';

  let { settings } = $props();

  const sections = [
    ['Top actual', 'Las acciones que hoy lideran el ranking de momentum (N-0), con suplentes, qué entra y sale, próximo rebalanceo y seguimiento de selecciones pasadas.'],
    ['Backtest', 'Qué habría pasado en el pasado: curva de capital, selecciones en cada rebalanceo y resultado de cada fecha de arranque.'],
    ['Plazos y sensibilidad', 'Compara plazos de rebalanceo y mide cuánto depende el resultado del mes de arranque (típico, peor y mejor).'],
    ['N-0 vs N-1', 'El modelo de siempre frente a una variante que ignora el último mes («skip-month»), plazo por plazo y por mitades del período.'],
    ['ETFs Vanguard', 'Ranking sin backtest por retorno ajustado por riesgo a 3, 6 y 12 meses, con tendencia y caída máxima. Los ETFs de efectivo no entran.'],
    ['Modelo por score', 'Scores de momentum, salud financiera y técnico para todo el S&P 500, su promedio y Markowitz sobre las mejores. Es una foto de hoy, sin backtest.'],
  ];

  const sources = [
    ['Precios y datos financieros', 'Yahoo Finance (precios ajustados, volumen y últimos estados financieros).'],
    ['Composición del S&P 500', 'CSV de hanshof/sp500_constituents y altas y bajas de Wikipedia: en cada fecha solo compiten las empresas que estaban en el índice (Point-in-Time).'],
    ['ETFs de Vanguard', 'Página de productos de Vanguard; los nuevos se agregan solos.'],
  ];

  const limits = [
    'Sesgo de supervivencia: las empresas que ya no cotizan quedan fuera y los retornos probablemente están inflados.',
    'La capitalización large-cap y mega-cap usa las acciones en circulación de hoy, lo que adelanta la entrada de empresas hoy grandes.',
    'Pocas decisiones: en 5 años hay entre 5 y 20 rebalanceos por plazo.',
    'Modelo por score: datos financieros de los últimos publicados, sin histórico ni backtest.',
    'Sin impuestos ni deslizamiento (diferencia entre el precio de cierre y el obtenido).',
  ];
</script>

<div class="about">
  <h2>Qué es esta aplicación</h2>
  <p>
    Evalúa una estrategia de <strong>momentum</strong> sobre el S&amp;P 500: cada <em>N</em> meses se compran, con
    ponderación igual, las {settings.top_n} acciones que más subieron en los últimos <em>N</em> meses. Muestra qué habría
    pasado (backtest) y qué elegiría hoy. Plazo de referencia: {settings.months} meses; capital inicial: US$ {int(settings.initial_capital)}.
  </p>
  <p class="muted">Herramienta de análisis personal y educativa. <strong>No es una recomendación de inversión.</strong></p>

  <h2>Qué muestra cada sección</h2>
  <div class="grid">
    {#each sections as [tab, text] (tab)}
      <div class="item"><strong>{tab}</strong><p>{text}</p></div>
    {/each}
  </div>

  <h2>Filtros</h2>
  <p>
    <strong>Universo:</strong> índice completo, large-cap + mega-cap (desde US$ 10.000 M) o mega-cap (desde US$ 200.000 M).
    <strong>Período:</strong> años simulados hacia atrás, hasta la fecha de la corrida.
  </p>

  <h2>Datos</h2>
  <dl>
    {#each sources as [name, text] (name)}
      <dt>{name}</dt>
      <dd>{text}</dd>
    {/each}
  </dl>

  <h2>Qué incluyen los resultados</h2>
  <p>
    Comisión de {share(settings.commission ?? 0, 2)} por lado al armar la cartera y por las posiciones que cambian en cada
    rebalanceo. Cada plazo se mide con todas las fechas de arranque posibles y se informa la mediana.
  </p>
  <details open>
    <summary>Límites a tener presentes</summary>
    <ul>
      {#each limits as text}
        <li>{text}</li>
      {/each}
    </ul>
  </details>

  <h2>Cómo se actualiza</h2>
  <p>
    Ejecutar <code>go run ./cmd/top3 pipeline</code> en <code>backend-go</code> y recargar la página. La fecha de «Corrida» es la
    de la última ejecución.
  </p>
</div>

<style>
  .about { max-width: 960px; }
  .about h2:first-child { margin-top: 16px; }
  p { margin: 6px 0; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: var(--space-2); margin: var(--space-2) 0; }
  .item {
    background: var(--card); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px var(--space-3);
    box-shadow: var(--shadow); font-size: .85rem;
  }
  .item strong { font-size: .72rem; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  .item p { margin: 4px 0 0; }
  dl { margin: 8px 0; display: grid; grid-template-columns: max-content 1fr; gap: 6px 20px; }
  dt { font-weight: 600; }
  dd { margin: 0; color: var(--muted); }
  summary { cursor: pointer; font-weight: 600; }
  ul { margin: 4px 0 8px; padding-left: 20px; color: var(--muted); }
  li { margin: 3px 0; }
  code { background: var(--card); border: 1px solid var(--line); border-radius: 4px; padding: 0 5px; font-size: .85em; }
  @media (max-width: 640px) {
    dl { grid-template-columns: 1fr; gap: 2px; }
    dd { margin-bottom: 8px; }
  }
</style>

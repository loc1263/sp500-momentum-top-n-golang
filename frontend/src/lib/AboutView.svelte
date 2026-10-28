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
    ['Chile (IPSA)', 'El mismo modelo por score aplicado a las 30 acciones del IPSA de la Bolsa de Santiago, en pesos chilenos, con tasa libre de riesgo propia. La lista sale de Wikipedia y se renueva cada mes.'],
  ];

  // Conceptos que aparecen en las tablas, en tarjetas cortas
  const concepts = [
    ['Momentum', 'La idea de que las acciones que más subieron en los últimos meses tienden a seguir subiendo un tiempo. Se mide con el retorno del período.'],
    ['Backtest', 'Simula qué habría pasado en el pasado aplicando las mismas reglas. Sirve para evaluar la estrategia, pero no asegura lo que pasará.'],
    ['Rebalanceo', 'Cada N meses se revisa la cartera: salen las acciones que dejaron de liderar y entran las nuevas. Cada cambio paga comisión.'],
    ['Rendimiento anual (CAGR)', 'Cuánto creció el dinero por año, en promedio compuesto. Un 40% anual multiplica el capital por 1,4 cada año.'],
    ['Resultado típico (mediana)', 'El resultado del medio entre todas las fechas de arranque posibles: la mitad terminó mejor y la mitad peor.'],
    ['Volatilidad', 'Cuánto oscila el precio en un año (desviación estándar anual). Más alta = más riesgo, para bien y para mal.'],
    ['Caída máxima', 'La mayor baja desde un máximo hasta el mínimo que le siguió. Muestra lo que habría dolido en el peor momento.'],
    ['Tasa libre de riesgo', 'Lo que rinde una inversión sin riesgo (por ejemplo, bonos del Tesoro). Es el punto de comparación: 4% para EE. UU. y 5% para Chile.'],
    ['Sharpe', 'Retorno por unidad de riesgo: (retorno esperado − tasa libre de riesgo) ÷ volatilidad. Más alto = más retorno para el riesgo asumido.'],
    ['Markowitz', 'Reparte el dinero entre varias acciones según su retorno esperado, su riesgo y cómo se mueven juntas. Sin posiciones cortas y con tope por acción y por sector.'],
    ['Máximo retorno', 'Cartera de Markowitz con el mayor retorno esperado cuya volatilidad no supera el tope fijado (por ejemplo, 20% anual).'],
    ['Máximo Sharpe', 'Cartera de Markowitz con el mejor Sharpe: la que más retorno da por cada unidad de riesgo.'],
    ['Mínima varianza', 'Cartera de Markowitz de menor volatilidad posible con esas acciones, sin mirar el retorno. La más tranquila.'],
    ['Score (0-100)', 'Posición de la acción frente a las demás en un indicador: 100 = la mejor, 0 = la peor. El score final promedia momentum, salud y técnico.'],
    ['Salud financiera', 'Score de los últimos balances: rentabilidad, solvencia (deuda), liquidez y generación de caja. La caja pesa el doble.'],
    ['Score técnico', 'Score del precio y el volumen: tendencia (medias móviles, ADX), impulso (MACD, RSI), volumen y posición en el rango de 52 semanas.'],
    ['TER y AUM', 'En los ETFs: el TER es el costo anual de mantenerlos (menor es mejor) y el AUM es el patrimonio que administran (mayor da más liquidez).'],
  ];

  const sources = [
    ['Precios y datos financieros', 'Yahoo Finance (precios ajustados, volumen y últimos estados financieros).'],
    ['Composición del S&P 500', 'CSV de hanshof/sp500_constituents y altas y bajas de Wikipedia: en cada fecha solo compiten las empresas que estaban en el índice (Point-in-Time).'],
    ['ETFs de Vanguard', 'Página de productos de Vanguard; los nuevos se agregan solos.'],
    ['Acciones del IPSA', 'Tabla de Wikipedia con los nemotécnicos del índice (se guarda y renueva cada mes); precios y datos financieros de Yahoo Finance (sufijo .SN).'],
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

  <h2>Conceptos clave</h2>
  <div class="grid">
    {#each concepts as [name, text] (name)}
      <div class="item"><strong>{name}</strong><p>{text}</p></div>
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
  @media (max-width: 640px) {
    dl { grid-template-columns: 1fr; gap: 2px; }
    dd { margin-bottom: 8px; }
  }
</style>

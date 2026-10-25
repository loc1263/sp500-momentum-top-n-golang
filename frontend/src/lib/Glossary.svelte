<details class="glossary">
  <summary>¿Cómo leer esta página?</summary>
  <dl>
    <dt>Plazo</dt>
    <dd>
      Cada cuántos meses se revisa la cartera. En cada revisión se miran los últimos N meses y se compran las
      acciones que más subieron en ese lapso; N meses después se repite.
    </dd>
    <dt>Rendimiento anual (CAGR)</dt>
    <dd>
      Cuánto creció el dinero por año, en promedio compuesto. Un 40% anual significa que el capital se multiplica
      por 1,4 cada año.
    </dd>
    <dt>Fecha de arranque</dt>
    <dd>
      El mismo plazo, empezado en meses distintos, da resultados distintos. Por eso se prueban todas las fechas de
      arranque posibles de cada plazo (un plazo de 6 meses tiene 6).
    </dd>
    <dt>Resultado típico (mediana)</dt>
    <dd>
      El resultado del medio entre todos los arranques: la mitad terminó mejor y la mitad peor. Es la mejor idea de
      lo que se puede esperar, sin depender de una fecha con suerte.
    </dd>
    <dt>Peor y mejor arranque</dt>
    <dd>Los extremos entre todas las fechas de arranque. Cuanto más separados, más depende el resultado del azar.</dd>
    <dt>Calendario base</dt>
    <dd>
      Un único arranque: el primer mes del período. Es un caso concreto, no un promedio, y puede ser mucho mejor o
      peor que lo típico.
    </dd>
  </dl>
</details>

<style>
  .glossary {
    background: var(--card); border: 1px solid var(--line); border-radius: 8px; padding: 6px 14px; margin: 12px 0;
    font-size: .9rem;
  }
  summary { cursor: pointer; font-weight: 600; padding: 2px 0; }
  dl { margin: 8px 0 4px; display: grid; grid-template-columns: max-content 1fr; gap: 4px 16px; }
  dt { font-weight: 600; }
  dd { margin: 0; color: var(--muted); }
  @media (max-width: 640px) {
    dl { grid-template-columns: 1fr; }
    dd { margin-bottom: 6px; }
  }
</style>

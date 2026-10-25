<script>
  /** Leyenda "Período: desde a hasta (N años)" para encabezar cada tabla o gráfico.
   *  `period` = { start_date, end_date, years }. `first` = primer rebalanceo efectivo: si arranca bastante
   *  después del inicio, se avisa. */
  let { period, first = null } = $props();

  const late = $derived(first && (new Date(first) - new Date(period.start_date)) / 864e5 > 62);
</script>

<p class="range">
  <strong>Período:</strong>
  {period.start_date} a {period.end_date}
  {#if period.years}<span class="muted">({period.years} {period.years === 1 ? 'año' : 'años'})</span>{/if}
  {#if late}
    <span class="neg">
      · primer rebalanceo efectivo: {first} (antes no había suficientes empresas que cumplan el universo)
    </span>
  {/if}
</p>

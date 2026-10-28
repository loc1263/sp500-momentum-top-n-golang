<script>
  import QuantView from './QuantView.svelte';

  let { ipsa } = $props();
</script>

<p class="muted">
  Precios y capitalizaciones en pesos chilenos (CLP). Las {ipsa.coverage.universe} acciones del IPSA según
  <a href={ipsa.universe.url} target="_blank" rel="noopener">Wikipedia</a> (lista del {ipsa.universe.updated_at}, se renueva cada mes);
  los datos de Yahoo Finance de acciones chilenas pueden ser menos completos que los de EE. UU.
</p>
<QuantView quant={ipsa} title="Ranking del IPSA" />

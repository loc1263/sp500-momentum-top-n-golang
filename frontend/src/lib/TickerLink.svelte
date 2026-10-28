<script>
  let { ticker, kind = 'stocks' } = $props(); // kind: 'stocks' o 'etf' (stockanalysis.com los separa)

  // Yahoo usa BRK-B; stockanalysis.com usa brk.b
  // Bolsa de Santiago (Yahoo: COPEC.SN, SQM-B.SN): stockanalysis.com usa /quote/snse/COPEC/ y SQM.B, en mayúsculas
  const santiago = $derived(ticker.endsWith('.SN'));
  const href = $derived(
    santiago
      ? `https://stockanalysis.com/quote/snse/${encodeURIComponent(ticker.slice(0, -3).replace('-', '.'))}/`
      : `https://stockanalysis.com/${kind}/${encodeURIComponent(ticker.toLowerCase().replace('-', '.'))}/`,
  );
</script>

<a {href} target="_blank" rel="noopener noreferrer" title="Ver {ticker} en stockanalysis.com"><strong>{ticker}</strong></a>

<style>
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
</style>

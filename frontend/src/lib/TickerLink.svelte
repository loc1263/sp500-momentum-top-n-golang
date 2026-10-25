<script>
  let { ticker, kind = 'stocks' } = $props(); // kind: 'stocks' o 'etf' (stockanalysis.com los separa)

  // Yahoo usa BRK-B; stockanalysis.com usa brk.b
  const href = $derived(`https://stockanalysis.com/${kind}/${encodeURIComponent(ticker.toLowerCase().replace('-', '.'))}/`);
</script>

<a {href} target="_blank" rel="noopener noreferrer" title="Ver {ticker} en stockanalysis.com"><strong>{ticker}</strong></a>

<style>
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
</style>

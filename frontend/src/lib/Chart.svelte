<script>
  import { usd } from './format.js';

  /** series: [{ name, color, width?, opacity?, points: [{ date: 'YYYY-MM-DD', value }] }] */
  let { series, height = 280, label = 'Capital (US$)' } = $props();

  let width = $state(800);
  // En pantallas angostas el eje Y cede ancho al gráfico
  const margin = $derived({ top: 12, right: width < 480 ? 10 : 16, bottom: 28, left: width < 480 ? 50 : 64 });
  let hover = $state(null);

  const t = (d) => new Date(d).getTime();
  const all = $derived(series.flatMap((s) => s.points));
  const xMin = $derived(Math.min(...all.map((p) => t(p.date))));
  const xMax = $derived(Math.max(...all.map((p) => t(p.date))));
  const yMin = $derived(Math.min(...all.map((p) => p.value)));
  const yMax = $derived(Math.max(...all.map((p) => p.value)));
  const innerW = $derived(Math.max(width - margin.left - margin.right, 10));
  const innerH = $derived(height - margin.top - margin.bottom);

  const x = (d) => margin.left + ((t(d) - xMin) / (xMax - xMin || 1)) * innerW;
  const y = (v) => margin.top + innerH - ((v - yMin) / (yMax - yMin || 1)) * innerH;

  const yTicks = $derived(Array.from({ length: 5 }, (_, i) => yMin + ((yMax - yMin) * i) / 4));
  const xTicks = $derived.by(() => {
    const years = [];
    for (let yr = new Date(xMin).getUTCFullYear() + 1; yr <= new Date(xMax).getUTCFullYear(); yr++) {
      years.push(`${yr}-01-01`);
    }
    // Si los años no caben (unos 40 px cada uno), se muestra uno de cada k
    const step = Math.max(1, Math.ceil(years.length / Math.max(1, Math.floor(innerW / 40))));
    return years.filter((_, i) => i % step === 0);
  });

  const path = (pts) => pts.map((p, i) => `${i ? 'L' : 'M'}${x(p.date).toFixed(1)},${y(p.value).toFixed(1)}`).join(' ');

  function move(e) {
    const box = e.currentTarget.getBoundingClientRect();
    const px = e.clientX - box.left;
    const target = xMin + ((px - margin.left) / innerW) * (xMax - xMin);
    const dates = [...new Set(all.map((p) => p.date))].sort();
    hover = dates.reduce((a, b) => (Math.abs(t(b) - target) < Math.abs(t(a) - target) ? b : a), dates[0]);
  }

  const hoverRows = $derived(
    hover
      ? series
          .filter((s) => s.opacity == null || s.opacity >= 1)
          .map((s) => ({ s, p: s.points.find((p) => p.date === hover) }))
          .filter((r) => r.p)
      : [],
  );
</script>

<div class="chart" bind:clientWidth={width}>
  {#if all.length}
    <svg {width} {height} role="img" aria-label={label} onpointermove={move} onpointerleave={() => (hover = null)}>
      {#each yTicks as v (v)}
        <line x1={margin.left} x2={width - margin.right} y1={y(v)} y2={y(v)} class="grid" />
        <text x={margin.left - 8} y={y(v) + 4} text-anchor="end" class="tick">{usd(v).replace(/,\d+$/, '')}</text>
      {/each}
      {#each xTicks as d (d)}
        <line x1={x(d)} x2={x(d)} y1={margin.top} y2={margin.top + innerH} class="grid" />
        <text x={x(d)} y={height - 8} text-anchor="middle" class="tick">{d.slice(0, 4)}</text>
      {/each}
      {#each series as s (s.name)}
        <path d={path(s.points)} fill="none" stroke={s.color} stroke-width={s.width ?? 2}
          stroke-opacity={s.opacity ?? 1} stroke-linejoin="round" />
      {/each}
      {#if hover}
        <line x1={x(hover)} x2={x(hover)} y1={margin.top} y2={margin.top + innerH} class="cursor" />
        {#each hoverRows as { s, p } (s.name)}
          <circle cx={x(p.date)} cy={y(p.value)} r="4" fill={s.color} />
        {/each}
      {/if}
    </svg>
    {#if hover}
      <div class="tip" style:left="{Math.min(x(hover) + 12, width - 190)}px">
        <strong>{hover}</strong>
        {#each hoverRows as { s, p } (s.name)}
          <div><span class="dot" style:background={s.color}></span>{s.name}: {usd(p.value)}</div>
        {/each}
      </div>
    {/if}
    <div class="legend">
      {#each series.filter((s) => s.opacity == null || s.opacity >= 1) as s (s.name)}
        <span><span class="dot" style:background={s.color}></span>{s.name}</span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .chart { position: relative; width: 100%; border: 1px solid var(--line); border-radius: 8px; padding: 8px 0 4px; }
  svg { display: block; touch-action: pan-y; }
  .grid { stroke: var(--line); stroke-width: 1; }
  .cursor { stroke: var(--muted); stroke-dasharray: 3 3; }
  .tick { fill: var(--muted); font-size: 11px; }
  .tip {
    position: absolute; top: 12px; pointer-events: none; background: var(--card); border: 1px solid var(--line);
    border-radius: 6px; padding: 6px 10px; font-size: .8rem; font-variant-numeric: tabular-nums; width: 180px;
  }
  .legend { display: flex; flex-wrap: wrap; gap: 4px 16px; padding: 4px 16px 0; font-size: .8rem; color: var(--muted); }
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; }
</style>

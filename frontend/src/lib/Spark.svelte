<script>
  import { dec } from './format.js';
  import { scaleY } from './evaluator.js';

  // Evolución anual de una cifra: línea con un punto por año (el último, más grande). Los años sin dato quedan sin punto.
  let { values, years, label } = $props();

  const W = 300, H = 72, PX = 12, TOP = 10, BOTTOM = H - 10;
  const points = $derived(
    values.map((v, i) => (v == null ? null : { x: PX + (i * (W - 2 * PX)) / Math.max(values.length - 1, 1), v, i })).filter(Boolean),
  );
  const lo = $derived(Math.min(...points.map((p) => p.v)));
  const hi = $derived(Math.max(...points.map((p) => p.v)));
  const y = (v) => scaleY(v, hi === lo ? lo - 1 : lo, hi === lo ? hi + 1 : hi, TOP, BOTTOM);
  const line = $derived(points.map((p) => `${p.x.toFixed(1)},${y(p.v).toFixed(1)}`).join(' '));
  const zero = $derived(lo < 0 && hi > 0 ? y(0) : null);
</script>

{#if points.length >= 2}
  <svg viewBox="0 0 {W} {H}" role="img" aria-label="Evolución de {label}: {years.join(', ')}">
    {#each [TOP, (TOP + BOTTOM) / 2, BOTTOM] as g (g)}<line class="grid" x1="0" x2={W} y1={g} y2={g} />{/each}
    {#if zero != null}<line class="zero" x1="0" x2={W} y1={zero} y2={zero} />{/if}
    <polyline points={line} />
    {#each points as p, k (p.i)}
      <circle cx={p.x} cy={y(p.v)} r={k === points.length - 1 ? 5 : 3}><title>{years[p.i]}: {dec(p.v, 1)}</title></circle>
    {/each}
  </svg>
  <div class="years">{#each years as yr (yr)}<span>{yr}</span>{/each}</div>
{/if}

<style>
  svg { width: 100%; max-width: 300px; height: auto; display: block; }
  .grid { stroke: var(--line); stroke-width: 1; }
  .zero { stroke: var(--muted); stroke-width: 1; stroke-dasharray: 3 3; }
  polyline { fill: none; stroke: var(--accent); stroke-width: 2; stroke-linejoin: round; }
  circle { fill: var(--accent); }
  .years { display: flex; justify-content: space-between; max-width: 300px; font-size: .62rem; color: var(--muted); }
</style>

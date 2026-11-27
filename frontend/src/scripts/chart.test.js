// Pruebas de chart.js: el cuadro de valores sigue al cursor y nunca se sale del gráfico (también en pantallas angostas).
//
// Fecha: 2026-11-27

import { beforeEach, describe, expect, it, vi } from 'vitest';

const data = {
  ml: 64, iw: 720, mt: 12, ih: 240, x0: 19723, x1: 20454, y0: 100, y1: 200,
  s: [{ n: 'Base', c: '#fff', p: [['2024-01-01', 100], ['2025-01-01', 150], ['2026-01-01', 200]] }],
};

// jsdom no calcula medidas: se fijan las del gráfico y del cuadro.
function mount(width, tipWidth = 180) {
  document.body.innerHTML = `<div class="chart" data-chart='${JSON.stringify(data)}'><svg viewBox="0 0 800 280"></svg><div class="tip" hidden></div></div>`;
  const chart = document.querySelector('.chart');
  const tip = chart.querySelector('.tip');
  Object.defineProperty(chart, 'clientWidth', { value: width });
  Object.defineProperty(tip, 'offsetWidth', { value: tipWidth });
  chart.querySelector('svg').getBoundingClientRect = () => ({ left: 0, width, top: 0, height: 100 });
  return { chart, svg: chart.querySelector('svg'), tip };
}

const move = (svg, clientX) => svg.dispatchEvent(new MouseEvent('pointermove', { clientX, bubbles: true }));

describe('chart.js', () => {
  beforeEach(async () => {
    vi.resetModules();
    document.body.innerHTML = '';
    await import('./chart.js');
  });

  it('muestra la fecha más cercana y el valor de cada serie', async () => {
    const { svg, tip } = mount(800);
    window.initCharts(document);
    await Promise.resolve();
    move(svg, 400);
    expect(tip.hidden).toBe(false);
    expect(tip.textContent).toContain('Base');
    expect(svg.querySelector('line.cursor')).not.toBeNull();
  });

  it('oculta el cuadro y el cursor al salir del gráfico', async () => {
    const { svg, tip } = mount(800);
    window.initCharts(document);
    await Promise.resolve();
    move(svg, 400);
    svg.dispatchEvent(new Event('pointerleave'));
    expect(tip.hidden).toBe(true);
    expect(svg.querySelector('line.cursor')).toBeNull();
  });

  it.each([358, 800])('con el gráfico de %i px el cuadro queda dentro', async (width) => {
    const { svg, tip } = mount(width);
    window.initCharts(document);
    await Promise.resolve();
    for (const fx of [0, width * 0.5, width]) {
      move(svg, fx);
      expect(tip.style.left).toMatch(/px$/); // en %, el ancho fijo del cuadro se salía en pantallas angostas
      const left = parseFloat(tip.style.left);
      expect(left).toBeGreaterThanOrEqual(4);
      expect(left + 180).toBeLessThanOrEqual(width);
    }
  });

  it('no inicializa dos veces el mismo gráfico', async () => {
    const { svg, tip } = mount(800);
    window.initCharts(document);
    window.initCharts(document);
    await Promise.resolve();
    move(svg, 400);
    expect(tip.querySelectorAll('strong')).toHaveLength(1);
  });
});

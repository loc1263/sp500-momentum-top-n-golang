// Cursor y cuadro de valores de los gráficos de capital: al mover el mouse sobre el SVG marca la fecha más cercana y lista el valor
// de cada serie visible. Usa los datos incrustados en data-chart (escalas y puntos), sin dependencias. window.initCharts(raíz) activa
// los gráficos que haya dentro de raíz; las páginas pintadas con Alpine lo llaman cada vez que cambian el HTML.
//
// Fecha: 2026-11-20

(() => {
  const W = 800, H = 280;
  const NS = 'http://www.w3.org/2000/svg';
  const usd = new Intl.NumberFormat('es-AR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  const day = (d) => Date.parse(`${d}T00:00:00Z`) / 864e5;

  function initChart(chart) {
    if (chart.dataset.ready) return;
    chart.dataset.ready = '1';
    const c = JSON.parse(chart.dataset.chart);
    const svg = chart.querySelector('svg');
    const tip = chart.querySelector('.tip');
    const dates = [...new Set(c.s.flatMap((s) => s.p.map((p) => p[0])))].sort();
    if (!dates.length) return;
    const x = (d) => c.ml + ((day(d) - c.x0) / (c.x1 - c.x0 || 1)) * c.iw;
    const y = (v) => c.mt + c.ih - ((v - c.y0) / (c.y1 - c.y0 || 1)) * c.ih;

    const layer = document.createElementNS(NS, 'g');
    svg.appendChild(layer);

    function clear() {
      layer.replaceChildren();
      tip.hidden = true;
    }

    svg.addEventListener('pointerleave', clear);
    svg.addEventListener('pointermove', (e) => {
      const box = svg.getBoundingClientRect();
      const px = ((e.clientX - box.left) / box.width) * W;
      const target = c.x0 + ((px - c.ml) / c.iw) * (c.x1 - c.x0);
      const date = dates.reduce((a, b) => (Math.abs(day(b) - target) < Math.abs(day(a) - target) ? b : a), dates[0]);
      layer.replaceChildren();
      const line = document.createElementNS(NS, 'line');
      line.setAttribute('class', 'cursor');
      Object.entries({ x1: x(date), x2: x(date), y1: c.mt, y2: c.mt + c.ih }).forEach(([k, v]) => line.setAttribute(k, v));
      layer.appendChild(line);
      tip.replaceChildren();
      const head = document.createElement('strong');
      head.textContent = date;
      tip.appendChild(head);
      for (const s of c.s) {
        const p = s.p.find((q) => q[0] === date);
        if (!p) continue;
        const dot = document.createElementNS(NS, 'circle');
        Object.entries({ cx: x(date), cy: y(p[1]), r: 4 }).forEach(([k, v]) => dot.setAttribute(k, v));
        dot.style.fill = s.c;
        layer.appendChild(dot);
        const row = document.createElement('div');
        const swatch = document.createElement('span');
        swatch.className = 'dot';
        swatch.style.background = s.c;
        row.append(swatch, `${s.n}: ${usd.format(p[1])}`);
        tip.appendChild(row);
      }
      tip.style.left = `${(Math.min(x(date) + 12, W - 190) / W) * 100}%`;
      tip.hidden = false;
    });
  }

  window.initCharts = (root = document) => {
    // Alpine reemplaza el HTML en su propio ciclo: se espera a que termine antes de buscar los gráficos.
    queueMicrotask(() => (root.matches?.('.chart[data-chart]') ? [root] : root.querySelectorAll('.chart[data-chart]')).forEach(initChart));
  };
  window.initCharts();
})();

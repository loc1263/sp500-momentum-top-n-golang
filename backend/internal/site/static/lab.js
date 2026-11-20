// Páginas del Lab: elige del window.LAB (lab-datos.js) la página del universo y período del hash (lab-backtest.html#200-10) o la
// de por defecto, y Alpine la pinta. Los selectores de período y universo solo cambian el hash.
//
// Fecha: 2026-11-20

document.addEventListener('alpine:init', () => {
  const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

  Alpine.data('lab', (view) => ({
    p: null,
    term: null,
    cterm: null,
    loaded: false,
    init() {
      this.load();
      window.addEventListener('hashchange', () => this.load());
    },
    load() {
      const pages = (window.LAB && window.LAB.pages[view]) || {};
      const key = decodeURIComponent(location.hash.slice(1));
      this.p = pages[key] || pages[window.LAB?.default] || null;
      this.term = this.p?.Terms?.find((t) => t.Selected)?.Months ?? null;
      this.cterm = this.p?.Charts?.find((c) => c.Selected)?.Months ?? null;
      this.loaded = true;
      // Las pestañas de las otras vistas conservan el universo y período elegidos.
      document.querySelectorAll('nav.tabs a').forEach((a) => (a.href = a.getAttribute('href').split('#')[0] + location.hash));
    },
    go(key) {
      location.hash = key;
    },
    // Celda de tabla: el HTML que armó Go o, si no, el texto escapado.
    cell: (c) => c.HTML || esc(c.Text ?? ''),
  }));
});

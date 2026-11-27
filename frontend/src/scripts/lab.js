// Páginas del Lab: pide lab/<vista>/<universo>-<años>.json con la clave del hash (lab-backtest.html#200-10) o, sin hash o si no existe,
// la de por defecto (lab/default.json), y Alpine la pinta. Los selectores de período y universo solo cambian el hash.
//
// Fecha: 2026-11-27

import { fetchJSON } from './fetch-json.js';

const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export default (view) => ({
  p: null,
  term: null,
  cterm: null,
  loaded: false,
  init() {
    this.load();
    window.addEventListener('hashchange', () => this.load());
  },
  async load() {
    const hash = decodeURIComponent(location.hash.slice(1));
    let p = /^[A-Za-z0-9._-]+$/.test(hash) ? await fetchJSON(`lab/${view}/${hash}.json`) : null;
    if (!p) {
      const def = (await fetchJSON('lab/default.json'))?.default;
      p = def ? await fetchJSON(`lab/${view}/${def}.json`) : null;
    }
    if (hash !== decodeURIComponent(location.hash.slice(1))) return; // el hash cambió mientras bajaba
    this.p = p;
    this.term = p?.Terms?.find((t) => t.Selected)?.Months ?? null;
    this.cterm = p?.Charts?.find((c) => c.Selected)?.Months ?? null;
    this.loaded = true;
    // Las pestañas de las otras vistas conservan el universo y período elegidos.
    document.querySelectorAll('nav.tabs a').forEach((a) => (a.href = a.getAttribute('href').split('#')[0] + location.hash));
  },
  go(key) {
    location.hash = key;
  },
  // Celda de tabla: el HTML que armó Go o, si no, el texto escapado.
  cell: (c) => c.HTML || esc(c.Text ?? ''),
});

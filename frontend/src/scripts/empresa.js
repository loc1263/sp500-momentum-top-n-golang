// Página única de empresa del evaluador: toma el ticker del hash (empresa.html#AAPL) y pide su ficha, ya formateada por Go, a
// empresas/<TICKER>.json (un archivo por empresa: se baja solo la que se ve). Alpine pinta la ficha.
//
// Fecha: 2026-11-27

import { fetchJSON } from './fetch-json.js';

export default () => ({
  title: document.title.split(' · ')[0],
  c: null,
  loaded: false,
  fy: 0, // posición del ejercicio elegido en los diagramas de flujos
  get year() {
    return this.c?.Flows?.Years?.[this.fy] ?? {};
  },
  init() {
    this.load();
    window.addEventListener('hashchange', () => this.load());
  },
  async load() {
    // Misma regla que companyKey en Go: mayúsculas y lo raro reemplazado por «_».
    const key = decodeURIComponent(location.hash.slice(1)).toUpperCase().replace(/[^A-Za-z0-9._-]/g, '_');
    this.loaded = false;
    const c = key ? await fetchJSON(`empresas/${key}.json`) : null;
    if (key !== decodeURIComponent(location.hash.slice(1)).toUpperCase().replace(/[^A-Za-z0-9._-]/g, '_')) return; // el hash cambió mientras bajaba
    this.c = c;
    this.fy = 0;
    this.loaded = true;
    if (c) document.title = `${c.Ticker} · ${this.title}`;
  },
});

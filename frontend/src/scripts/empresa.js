// Página única de empresa del evaluador: toma el ticker del hash (empresa.html#AAPL) y busca sus datos, ya formateados por Go,
// en window.EMPRESAS (empresas.js). Alpine pinta la ficha; este componente solo resuelve cuál mostrar.
//
// Fecha: 2026-11-27

import { dataReady } from './data-ready.js';

export default () => ({
  c: null,
  loaded: false,
  async init() {
    await dataReady();
    this.load();
    window.addEventListener('hashchange', () => this.load());
  },
  load() {
    // Misma regla que companyKey en Go: mayúsculas y lo raro reemplazado por «_».
    const key = decodeURIComponent(location.hash.slice(1)).toUpperCase().replace(/[^A-Za-z0-9._-]/g, '_');
    this.c = (window.EMPRESAS || {})[key] || null;
    this.loaded = true;
    if (this.c) document.title = `${this.c.Ticker} · ${document.title.split(' · ')[0]}`;
  },
});

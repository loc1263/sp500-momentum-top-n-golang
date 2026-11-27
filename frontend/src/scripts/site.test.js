// Pruebas de site.js: botón de tema y globo de ayuda para pantallas táctiles.
//
// Fecha: 2026-11-27

import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

// El localStorage de jsdom no siempre existe según la versión de Node: se usa uno en memoria.
const store = new Map();
vi.stubGlobal('localStorage', {
  getItem: (k) => store.get(k) ?? null,
  setItem: (k, v) => store.set(k, String(v)),
  clear: () => store.clear(),
});

const touch = (on) => {
  window.matchMedia = vi.fn(() => ({ matches: on }));
};

// site.js se importa una sola vez: registra sus listeners en document y no se pueden quitar entre pruebas.
beforeAll(async () => {
  document.body.innerHTML = `
    <button id="theme"></button>
    <span class="hint" title="Explicación del margen">?</span>
    <span class="dotv good" title="Sharpe alto"></span>
    <p id="otro">texto</p>`;
  touch(false);
  await import('./site.js');
});
// Cierra, por la vía normal, el globo que haya dejado la prueba anterior.
const load = async () => document.getElementById('otro').dispatchEvent(new MouseEvent('click', { bubbles: true }));
const click = (sel) => document.querySelector(sel).dispatchEvent(new MouseEvent('click', { bubbles: true }));

describe('tema', () => {
  beforeEach(() => {
    store.clear();
    document.documentElement.dataset.theme = 'dark';
    touch(false);
  });

  it('alterna claro / oscuro y lo recuerda', async () => {
    await load();
    click('#theme');
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(localStorage.getItem('theme')).toBe('light');
    expect(document.getElementById('theme').textContent).toContain('Oscuro');
    click('#theme');
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('theme')).toBe('dark');
    expect(document.getElementById('theme').textContent).toContain('Claro');
  });
});

describe('globo de ayuda', () => {
  it('en pantalla táctil un toque abre el texto del title', async () => {
    touch(true);
    await load();
    click('.hint');
    expect(document.querySelector('.hint-pop')?.textContent).toBe('Explicación del margen');
  });

  it('otro toque sobre la misma ayuda lo cierra', async () => {
    touch(true);
    await load();
    click('.hint');
    click('.hint');
    expect(document.querySelector('.hint-pop')).toBeNull();
  });

  it('tocar fuera lo cierra y otra ayuda lo reemplaza', async () => {
    touch(true);
    await load();
    click('.hint');
    click('.dotv');
    expect(document.querySelectorAll('.hint-pop')).toHaveLength(1);
    expect(document.querySelector('.hint-pop').textContent).toBe('Sharpe alto');
    click('#otro');
    expect(document.querySelector('.hint-pop')).toBeNull();
  });

  it('con mouse no abre nada (el title nativo ya funciona)', async () => {
    touch(false);
    await load();
    click('.hint');
    expect(document.querySelector('.hint-pop')).toBeNull();
  });
});

// Pruebas de empresa.js: carga de la ficha por el hash y ejercicio elegido en los diagramas de flujos.
//
// Autor: Luis Oyanader C + Claude
// Fecha: 2026-11-27

import { beforeEach, describe, expect, it, vi } from 'vitest';

const fetchJSON = vi.hoisted(() => vi.fn());
vi.mock('./fetch-json.js', () => ({ fetchJSON }));

import empresa from './empresa.js';

const ficha = { Ticker: 'OKTA', Flows: { Currency: 'USD', Years: [{ Label: 'FY2026' }, { Label: 'FY2025' }] } };

describe('empresa.js', () => {
  beforeEach(() => {
    fetchJSON.mockReset();
    location.hash = '#okta';
  });

  it('pide empresas/<TICKER>.json con la clave en mayúsculas', async () => {
    fetchJSON.mockResolvedValue(ficha);
    const e = empresa();
    await e.load();
    expect(fetchJSON).toHaveBeenCalledWith('empresas/OKTA.json');
    expect(e.c).toBe(ficha);
    expect(e.loaded).toBe(true);
  });

  it('reemplaza lo raro del hash por «_», igual que Go', async () => {
    fetchJSON.mockResolvedValue(null);
    location.hash = '#brk/b';
    await empresa().load();
    expect(fetchJSON).toHaveBeenCalledWith('empresas/BRK_B.json');
  });

  it('year devuelve el ejercicio elegido y vuelve al más reciente al cambiar de empresa', async () => {
    fetchJSON.mockResolvedValue(ficha);
    const e = empresa();
    await e.load();
    e.fy = 1;
    expect(e.year.Label).toBe('FY2025');
    await e.load();
    expect(e.fy).toBe(0);
    expect(e.year.Label).toBe('FY2026');
  });

  it('sin diagramas year es un objeto vacío (la plantilla no falla)', async () => {
    fetchJSON.mockResolvedValue({ Ticker: 'JPM' });
    const e = empresa();
    await e.load();
    expect(e.year).toEqual({});
  });
});

import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import VisorView from './VisorView.svelte';
import { cellClass, filterRows, fmt, label, sortRows, toCsv } from './visor.js';

const section = (id, label, cols, rows, extra = {}) => ({
  id, label, cols, rows,
  fmt: { ret_1m: 'pct', sharpe: 'dec', market_cap_B: 'int', aum_B: 'int' },
  sign: ['ret_1m'],
  thr: { sharpe: ['>', 1] },
  search: [0, 1],
  ...extra,
});
const COLS = ['ticker', 'nombre', 'market_cap_B', 'ret_1m', 'sharpe', 'vs_spy'];
const data = {
  date: '2026-11-18', ref: 'SPY', ref_ytd: 7.5, rf: 3,
  sections: [
    section('acciones', 'Acciones', COLS, [
      ['AAA', 'Alfa Corp', 3000.4, 5.25, 1.5, 'SUPERA'],
      ['BBB', 'Beta, Inc', 200, -2, 0.5, 'NO SUPERA'],
      ['CCC', 'Gamma', 150, null, null, null],
    ]),
    section('etf', 'ETF', COLS, [['SPY', 'S&P 500 ETF', 800, 1, 1, 'SUPERA']]),
  ],
};
const load = () => Promise.resolve(data);
const tickers = () => screen.getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[0].textContent.trim());

describe('lib/visor.js', () => {
  const sec = data.sections[0];
  it('da formato a las celdas y deja vacías las que no tienen dato', () => {
    expect(fmt(sec, 'ret_1m', 5.25)).toBe('5.3%');
    expect(fmt(sec, 'sharpe', 1.5)).toBe('1.50');
    expect(fmt(sec, 'ret_1m', null)).toBe('');
    expect(fmt(sec, 'nombre', 'Alfa')).toBe('Alfa');
  });
  it('pinta por signo, por umbral estricto y por vs_spy', () => {
    expect(cellClass(sec, 'ret_1m', 1)).toBe('pos');
    expect(cellClass(sec, 'ret_1m', -1)).toBe('neg');
    expect(cellClass(sec, 'ret_1m', null)).toBe('');
    expect(cellClass(sec, 'sharpe', 1)).toBe('');
    expect(cellClass(sec, 'sharpe', 1.01)).toBe('pos');
    expect(cellClass(sec, 'vs_spy', 'NO SUPERA')).toBe('neg');
  });
  it('busca por texto y ordena dejando los vacíos al final', () => {
    expect(filterRows(sec, 'gam').map((r) => r[0])).toEqual(['CCC']);
    expect(filterRows(sec, '  ')).toHaveLength(3);
    expect(sortRows(sec.rows, 3, true).map((r) => r[0])).toEqual(['AAA', 'BBB', 'CCC']);
    expect(sortRows(sec.rows, 3, false).map((r) => r[0])).toEqual(['BBB', 'AAA', 'CCC']);
  });
  it('arma el CSV escapando comas', () => {
    expect(toCsv(sec, [sec.rows[1]]).split('\n')[1]).toBe('BBB,"Beta, Inc",200,-2,0.5,NO SUPERA');
  });
  it('abrevia los encabezados de plazos', () => {
    expect([label('ret_3m'), label('ret_5y'), label('sharpe'), label('otra')]).toEqual(['3m', '5a', 'Sharpe', 'otra']);
  });
});

describe('VisorView', () => {
  it('muestra la primera tabla y cambia a ETF', async () => {
    render(VisorView, { load });
    expect(await screen.findByText('Alfa Corp')).toBeInTheDocument();
    expect(tickers()).toEqual(['AAA', 'BBB', 'CCC']);
    await fireEvent.click(screen.getByRole('tab', { name: /ETF/ }));
    expect(tickers()).toEqual(['SPY']);
  });

  it('ordena al hacer clic en el encabezado y alterna el sentido', async () => {
    render(VisorView, { load });
    await screen.findByText('Alfa Corp');
    const th = screen.getByRole('button', { name: /^1m/ });
    await fireEvent.click(th);
    expect(tickers()).toEqual(['AAA', 'BBB', 'CCC']);
    await fireEvent.click(screen.getByRole('button', { name: /^1m/ }));
    expect(tickers()).toEqual(['BBB', 'AAA', 'CCC']);
  });

  it('filtra con el buscador y avisa si no hay coincidencias', async () => {
    render(VisorView, { load });
    await screen.findByText('Alfa Corp');
    await fireEvent.input(screen.getByRole('searchbox'), { target: { value: 'beta' } });
    expect(tickers()).toEqual(['BBB']);
    await fireEvent.input(screen.getByRole('searchbox'), { target: { value: 'zzz' } });
    expect(screen.getByText(/Ningún instrumento coincide/)).toBeInTheDocument();
  });

  it('avisa cómo generar el JSON si no se pudo cargar', async () => {
    render(VisorView, { load: () => Promise.reject(new Error('x')) });
    expect(await screen.findByText(/No se pudo cargar data\/visor.json/)).toBeInTheDocument();
  });
});

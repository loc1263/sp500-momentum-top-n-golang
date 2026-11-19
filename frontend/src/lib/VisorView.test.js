import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import VisorView from './VisorView.svelte';
import { activeFilters, cellClass, filterColumns, filterRows, fmt, label, sortRows, toCsv } from './visor.js';

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
const tickers = () => [...document.querySelectorAll('tbody tr')].map((r) => within(r).getAllByRole('cell')[0].textContent.trim());

describe('lib/visor.js', () => {
  const sec = data.sections[0];
  it('enlaza cada ticker a stockanalysis.com según el tipo de instrumento', async () => {
    render(VisorView, { load });
    await screen.findByText('AAA');
    expect(screen.getByRole('link', { name: 'AAA' })).toHaveAttribute('href', 'https://stockanalysis.com/stocks/aaa/');
    await fireEvent.click(screen.getByRole('tab', { name: /ETF/ }));
    expect(screen.getByRole('link', { name: 'SPY' })).toHaveAttribute('href', 'https://stockanalysis.com/etf/spy/');
  });
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
    expect(filterRows(sec, ' , ')).toHaveLength(3);
    expect(filterRows(sec, 'aaa, ccc').map((r) => r[0])).toEqual(['AAA', 'CCC']);
    expect(filterRows(sec, 'aaa,zzz').map((r) => r[0])).toEqual(['AAA']);
    expect(sortRows(sec.rows, 3, true).map((r) => r[0])).toEqual(['AAA', 'BBB', 'CCC']);
    expect(sortRows(sec.rows, 3, false).map((r) => r[0])).toEqual(['BBB', 'AAA', 'CCC']);
  });
  it('filtra por columna: texto, rango numérico y vs_spy, todos a la vez', () => {
    const ids = (f) => filterColumns(sec, sec.rows, f).map((r) => r[0]);
    expect(ids({})).toEqual(['AAA', 'BBB', 'CCC']);
    expect(ids({ nombre: { text: 'ALFA' } })).toEqual(['AAA']);
    expect(ids({ sharpe: { min: '1' } })).toEqual(['AAA']); // los vacíos no pasan
    expect(ids({ market_cap_B: { min: '160', max: '250' } })).toEqual(['BBB']);
    expect(ids({ ret_1m: { max: '0' } })).toEqual(['BBB']);
    expect(ids({ vs_spy: { eq: 'NO SUPERA' } })).toEqual(['BBB']);
    expect(ids({ sharpe: { min: '0' }, vs_spy: { eq: 'NO SUPERA' } })).toEqual(['BBB']);
    expect(ids({ sharpe: { min: '' }, nombre: { text: ' ' } })).toHaveLength(3);
    expect(activeFilters({ sharpe: { min: '1' }, nombre: { text: '' }, vs_spy: { eq: '' } })).toBe(1);
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
    await fireEvent.input(screen.getByRole('searchbox', { name: /Buscar/ }), { target: { value: 'beta' } });
    expect(tickers()).toEqual(['BBB']);
    await fireEvent.input(screen.getByRole('searchbox', { name: /Buscar/ }), { target: { value: 'zzz' } });
    expect(screen.getByText(/Ningún instrumento coincide/)).toBeInTheDocument();
  });

  it('filtra por columna, actualiza el contador y se limpia con el botón', async () => {
    render(VisorView, { load });
    await screen.findByText('Alfa Corp');
    await fireEvent.input(screen.getByLabelText('Sharpe mín'), { target: { value: '1' } });
    expect(tickers()).toEqual(['AAA']);
    expect(screen.getByRole('tab', { name: /Acciones \(1\)/ })).toBeInTheDocument();
    await fireEvent.change(screen.getByLabelText('Filtrar vs SPY'), { target: { value: 'NO SUPERA' } });
    expect(screen.getByText(/Ningún instrumento coincide/)).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: /Limpiar filtros \(2\)/ }));
    expect(tickers()).toEqual(['AAA', 'BBB', 'CCC']);
  });

  it('al cambiar de tabla se borran los filtros por columna', async () => {
    render(VisorView, { load });
    await screen.findByText('Alfa Corp');
    await fireEvent.input(screen.getByLabelText('Filtrar Nombre'), { target: { value: 'beta' } });
    await fireEvent.click(screen.getByRole('tab', { name: /ETF/ }));
    expect(tickers()).toEqual(['SPY']);
    expect(screen.getByLabelText('Filtrar Nombre')).toHaveValue('');
  });

  it('marca una fila con un clic y la desmarca con otro', async () => {
    render(VisorView, { load });
    await screen.findByText('Alfa Corp');
    const row = (t) => screen.getByText(t).closest('tr');
    await fireEvent.click(row('Alfa Corp'));
    expect(row('Alfa Corp')).toHaveClass('marked');
    await fireEvent.click(row('Gamma'));
    expect(row('Alfa Corp')).not.toHaveClass('marked');
    expect(row('Gamma')).toHaveClass('marked');
    await fireEvent.click(row('Gamma'));
    expect(row('Gamma')).not.toHaveClass('marked');
  });

  it('avisa cómo generar el JSON si no se pudo cargar', async () => {
    render(VisorView, { load: () => Promise.reject(new Error('x')) });
    expect(await screen.findByText(/No se pudo cargar data\/visor.json/)).toBeInTheDocument();
  });
});

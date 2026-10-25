import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import EtfView from './EtfView.svelte';
import { report } from '../test/fixtures.js';

const etf = report.etf;
const tickers = () => screen.getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[1].textContent.trim());

describe('EtfView', () => {
  it('lista el ranking con score, retornos por plazo y riesgo', () => {
    render(EtfView, { etf });
    expect(tickers()).toEqual(['VONV', 'VDE', 'BND']);
    const row = screen.getAllByRole('row')[1];
    expect(row).toHaveTextContent('Russell 1000 Value');
    expect(row).toHaveTextContent('1.67');
    expect(row).toHaveTextContent('+4,5%');
    expect(row).toHaveTextContent('+29,1%');
    expect(row).toHaveTextContent('11,1%'); // volatilidad sin signo
    expect(row).toHaveTextContent('-6,8%');
  });

  it('tiene una columna de retorno por plazo', () => {
    render(EtfView, { etf });
    expect(screen.getByRole('button', { name: /Ret\. 3m/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Ret\. 12m/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Ret\. 6m/ })).not.toBeInTheDocument();
  });

  it('muestra la referencia libre de riesgo solo si existe', () => {
    const { unmount } = render(EtfView, { etf });
    expect(screen.getByText('Referencia sin riesgo').nextElementSibling).toHaveTextContent('VUSB');
    unmount();
    render(EtfView, { etf: { ...etf, risk_free: null } });
    expect(screen.queryByText('Referencia sin riesgo')).not.toBeInTheDocument();
  });

  it('ordena al hacer clic: lo numérico de mayor a menor y luego al revés', async () => {
    render(EtfView, { etf });
    const header = screen.getByRole('button', { name: /Ret\. 12m/ });
    await fireEvent.click(header);
    expect(tickers()).toEqual(['VDE', 'VONV', 'BND']);
    expect(header.closest('th')).toHaveAttribute('aria-sort', 'descending');
    await fireEvent.click(header);
    expect(tickers()).toEqual(['BND', 'VONV', 'VDE']);
    expect(header.closest('th')).toHaveAttribute('aria-sort', 'ascending');
  });

  it('ordena el texto alfabéticamente', async () => {
    render(EtfView, { etf });
    await fireEvent.click(screen.getByRole('button', { name: /Ticker/ }));
    expect(tickers()).toEqual(['BND', 'VDE', 'VONV']);
  });

  it('filtra por categoría', async () => {
    render(EtfView, { etf });
    await fireEvent.change(screen.getByLabelText('Categoría'), { target: { value: 'Sectores' } });
    expect(tickers()).toEqual(['VDE']);
    expect(screen.getByText('1 de 3')).toBeInTheDocument();
  });

  it('puede mostrar solo los que están en tendencia alcista', async () => {
    render(EtfView, { etf });
    await fireEvent.click(screen.getByLabelText('Solo en tendencia alcista'));
    expect(tickers()).toEqual(['VONV', 'VDE']);
  });

  it('avisa si el filtro no deja ningún ETF', async () => {
    render(EtfView, { etf: { ...etf, rows: etf.rows.filter((r) => r.sma_dist < 0) } });
    await fireEvent.click(screen.getByLabelText('Solo en tendencia alcista'));
    expect(screen.getByText('Ningún ETF cumple el filtro.')).toBeInTheDocument();
  });

  it('resalta los del top', () => {
    render(EtfView, { etf });
    const rows = screen.getAllByRole('row').slice(1);
    expect(rows.map((r) => r.classList.contains('top'))).toEqual([true, true, false]);
  });

  it('muestra los cambios del top contra la corrida anterior', () => {
    render(EtfView, { etf });
    const line = screen.getByText(/Cambios del top 2 vs\. 2026-10-16/).closest('p');
    expect(line).toHaveTextContent('entran AMD');
    expect(line).toHaveTextContent('salen VTV');
  });

  it('avisa si el top no cambió y en la primera corrida', () => {
    const { unmount } = render(EtfView, { etf: { ...etf, changes: { entered: [], exited: [], kept: ['VONV'] } } });
    expect(screen.getByText(/ninguno\./)).toBeInTheDocument();
    unmount();
    render(EtfView, { etf: { ...etf, previous_as_of: null, changes: null } });
    expect(screen.getByText(/Primera corrida guardada/)).toBeInTheDocument();
  });

  it('enlaza los ETFs a la sección de ETFs de stockanalysis.com', () => {
    render(EtfView, { etf });
    expect(screen.getByRole('link', { name: 'VONV' })).toHaveAttribute('href', 'https://stockanalysis.com/etf/vonv/');
  });
});

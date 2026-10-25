import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import Range from './Range.svelte';
import TickerLink from './TickerLink.svelte';
import Glossary from './Glossary.svelte';
import Bunny from './Bunny.svelte';

describe('TickerLink', () => {
  it('enlaza a stockanalysis.com en minúsculas', () => {
    render(TickerLink, { ticker: 'NVDA' });
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://stockanalysis.com/stocks/nvda/');
  });
  it('convierte el guion de Yahoo en punto', () => {
    render(TickerLink, { ticker: 'BRK-B' });
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://stockanalysis.com/stocks/brk.b/');
  });
  it('los ETFs van a la sección /etf/', () => {
    render(TickerLink, { ticker: 'VTI', kind: 'etf' });
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://stockanalysis.com/etf/vti/');
  });
  it('abre en pestaña nueva de forma segura', () => {
    render(TickerLink, { ticker: 'AAPL' });
    const a = screen.getByRole('link');
    expect(a).toHaveAttribute('target', '_blank');
    expect(a).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });
});

describe('Range', () => {
  const period = { start_date: '2020-01-31', end_date: '2025-01-31', years: 5 };
  it('muestra las fechas y los años', () => {
    render(Range, { period });
    expect(screen.getByText(/2020-01-31 a 2025-01-31/)).toBeInTheDocument();
    expect(screen.getByText('(5 años)')).toBeInTheDocument();
  });
  it('usa singular para un año', () => {
    render(Range, { period: { ...period, years: 1 } });
    expect(screen.getByText('(1 año)')).toBeInTheDocument();
  });
  it('avisa si el primer rebalanceo llega más de dos meses tarde', () => {
    render(Range, { period, first: '2020-06-30' });
    expect(screen.getByText(/primer rebalanceo efectivo: 2020-06-30/)).toBeInTheDocument();
  });
  it('no avisa si el arranque es normal', () => {
    render(Range, { period, first: '2020-02-28' });
    expect(screen.queryByText(/primer rebalanceo efectivo/)).not.toBeInTheDocument();
  });
});

describe('Glossary', () => {
  it('define los términos clave', () => {
    render(Glossary);
    expect(screen.getByText('Rendimiento anual (CAGR)')).toBeInTheDocument();
    expect(screen.getByText('Calendario base')).toBeInTheDocument();
  });
});

describe('Bunny', () => {
  it('saluda con un letrero al hacerle clic', async () => {
    render(Bunny);
    expect(screen.queryByText(/Hola/)).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Conejito' }));
    expect(screen.getByRole('status')).toHaveTextContent('¡Hola!');
  });
});

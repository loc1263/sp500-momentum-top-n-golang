import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import IpsaView from './IpsaView.svelte';
import { report } from '../test/fixtures.js';

const ipsa = { ...report.quant, currency: 'CLP', universe: { updated_at: '2026-10-28', url: 'https://es.wikipedia.org/wiki/IPSA' } };

describe('IpsaView', () => {
  it('indica la moneda y el origen de la lista', () => {
    render(IpsaView, { ipsa });
    expect(screen.getByText(/pesos chilenos \(CLP\)/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Wikipedia' })).toHaveAttribute('href', ipsa.universe.url);
  });

  it('titula el ranking como el del IPSA', () => {
    render(IpsaView, { ipsa });
    expect(screen.getByRole('heading', { name: 'Ranking del IPSA' })).toBeInTheDocument();
  });
});

describe('IpsaView: sector en una palabra', () => {
  it('abrevia los sectores de varias palabras y deja el nombre completo de ayuda', () => {
    const data = { ...ipsa, ranking: ipsa.ranking.map((r, i) => (i === 2 ? { ...r, sector: 'Financial Services' } : r)) };
    render(IpsaView, { ipsa: data });
    const cell = [...document.querySelectorAll('td')].find((c) => c.textContent.trim() === 'Financial');
    expect(cell).toBeTruthy();
    expect(cell.getAttribute('title')).toBe('Financial Services');
  });
});

describe('IpsaView: sector en la tabla de Markowitz', () => {
  it('también lo muestra en una palabra', () => {
    const data = { ...ipsa, ranking: ipsa.ranking.map((r) => ({ ...r, sector: 'Financial Services' })) };
    render(IpsaView, { ipsa: data });
    const table = screen.getByRole('heading', { name: /mejores y sus pesos/ }).parentElement.querySelector('details.more + .wrap');
    expect(within(table).getAllByText('Financial').length).toBeGreaterThan(0);
    expect(within(table).queryByText('Financial Services')).toBeNull();
  });
});

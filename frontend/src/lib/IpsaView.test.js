import { render, screen } from '@testing-library/svelte';
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

import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Lab from './Lab.svelte';
import { report } from './test/fixtures.js';

const json = (body) => ({
  ok: true,
  headers: { get: () => 'application/json' },
  json: async () => body,
});

beforeEach(() => {
  location.hash = '';
  delete document.documentElement.dataset.theme;
});
afterEach(() => vi.unstubAllGlobals());

describe('Lab', () => {
  it('abre el Backtest por defecto, con filtros de período y universo', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(Lab);
    expect(await screen.findByText('Curva de capital')).toBeInTheDocument();
    expect(screen.getByLabelText('Universo')).toBeInTheDocument();
    expect(screen.getByLabelText('Período')).toBeInTheDocument();
    expect(document.querySelector('header h2.section')).toHaveTextContent('Lab: Backtest');
    expect(document.title).toBe('Momentum y Scores · Lab: Backtest');
  });

  it('cambia de pestaña y actualiza el hash', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(Lab);
    await screen.findByText('Curva de capital');
    await fireEvent.click(screen.getByRole('tab', { name: 'Plazos y sensibilidad' }));
    expect(await screen.findByText('Comparación de plazos')).toBeInTheDocument();
    await waitFor(() => expect(location.hash).toBe('#sweep'));
  });

  it('abre la pestaña indicada en el hash', async () => {
    location.hash = '#sweep';
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(Lab);
    expect(await screen.findByText('Comparación de plazos')).toBeInTheDocument();
  });

  it('enlaza de vuelta a la app', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(Lab);
    await screen.findByText('Curva de capital');
    expect(screen.getByRole('link', { name: 'Volver a la app' })).toHaveAttribute('href', './');
  });

  it('muestra un error si no se puede leer el archivo', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('red'); }));
    render(Lab);
    expect(await screen.findByText(/No se pudo cargar/)).toBeInTheDocument();
  });
});

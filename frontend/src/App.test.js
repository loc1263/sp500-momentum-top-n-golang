import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App.svelte';
import { report } from './test/fixtures.js';

const json = (body) => ({
  ok: true,
  headers: { get: () => 'application/json' },
  json: async () => body,
});
const fail = () => ({ ok: false, headers: { get: () => '' } });

beforeEach(() => {
  location.hash = '';
  delete document.documentElement.dataset.theme;
});
afterEach(() => vi.unstubAllGlobals());

describe('App', () => {
  it('carga el JSON estático y muestra el Top actual', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    expect(await screen.findByText('NVDA')).toBeInTheDocument();
    expect(screen.getByText(/Corrida del 2026-10-20/)).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('data/report.json', { cache: 'no-cache' });
  });

  it('no pide nada al backend (solo el JSON estático)', async () => {
    const f = vi.fn(async () => json(report));
    vi.stubGlobal('fetch', f);
    render(App);
    await screen.findByText('NVDA');
    expect(f.mock.calls.map((c) => c[0])).toEqual(['data/report.json']);
  });

  it('muestra un error si la respuesta no es JSON', async () => {
    const html = { ok: true, headers: { get: () => 'text/html' }, json: async () => ({}) };
    vi.stubGlobal('fetch', vi.fn(async () => html));
    render(App);
    expect(await screen.findByText(/No se pudo cargar data\/report.json/)).toBeInTheDocument();
  });

  it('muestra un error si no se puede leer el archivo', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('red'); }));
    render(App);
    expect(await screen.findByText(/No se pudo cargar/)).toBeInTheDocument();
  });

  it('cambia de pestaña y actualiza el hash', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    await screen.findByText('NVDA');
    await fireEvent.click(screen.getByRole('tab', { name: 'Plazos y sensibilidad' }));
    expect(await screen.findByText('Comparación de plazos')).toBeInTheDocument();
    await waitFor(() => expect(location.hash).toBe('#sweep'));
  });

  it('muestra el ranking de ETFs y oculta los filtros que no aplican', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    await screen.findByText('NVDA');
    await fireEvent.click(screen.getByRole('tab', { name: 'ETFs Vanguard' }));
    expect(await screen.findByText('Ranking global de ETFs de Vanguard')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('ETFs de Vanguard');
    expect(screen.getByRole('link', { name: 'VONV' })).toBeInTheDocument();
    expect(screen.queryByLabelText('Universo')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Período')).not.toBeInTheDocument();
    expect(screen.queryByText('¿Cómo leer esta página?')).not.toBeInTheDocument();
    await waitFor(() => expect(location.hash).toBe('#etf'));
  });

  it('muestra el modelo por score y oculta los filtros que no aplican', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    await screen.findByText('NVDA');
    await fireEvent.click(screen.getByRole('tab', { name: 'Modelo por score' }));
    expect(await screen.findByRole('heading', { name: 'Cómo se calcula' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Modelo por score');
    expect(screen.queryByLabelText('Universo')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Período')).not.toBeInTheDocument();
    await waitFor(() => expect(location.hash).toBe('#quant'));
  });

  it('avisa si todavía no hay modelo por score', async () => {
    location.hash = '#quant';
    vi.stubGlobal('fetch', vi.fn(async () => json({ ...report, quant: null })));
    render(App);
    expect(await screen.findByText(/Sin modelo por score/)).toBeInTheDocument();
  });

  it('muestra la sección About y oculta los filtros', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    await screen.findByText('NVDA');
    expect(screen.queryByRole('tab', { name: 'About' })).not.toBeInTheDocument(); // no es parte del menú
    await fireEvent.click(screen.getByRole('button', { name: 'About' }));
    expect(await screen.findByText('Qué es esta aplicación')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('About');
    expect(screen.queryByLabelText('Universo')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Período')).not.toBeInTheDocument();
    await waitFor(() => expect(location.hash).toBe('#about'));
  });

  it('avisa si todavía no hay ranking de ETFs', async () => {
    location.hash = '#etf';
    vi.stubGlobal('fetch', vi.fn(async () => json({ ...report, etf: null })));
    render(App);
    expect(await screen.findByText(/Sin ranking de ETFs/)).toBeInTheDocument();
  });

  it('abre la pestaña indicada en el hash', async () => {
    location.hash = '#backtest';
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    expect(await screen.findByText('Curva de capital')).toBeInTheDocument();
  });

  it('alterna el tema', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(report)));
    render(App);
    await fireEvent.click(screen.getByRole('button', { name: /tema/i }));
    expect(document.documentElement.dataset.theme).toBe('dark');
    await fireEvent.click(screen.getByRole('button', { name: /tema/i }));
    expect(document.documentElement.dataset.theme).toBe('light');
  });
});

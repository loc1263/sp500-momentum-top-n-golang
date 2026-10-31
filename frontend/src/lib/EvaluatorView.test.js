import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import EvaluatorView from './EvaluatorView.svelte';

const metric = (id, v, l, extra = {}) => ({ id, v, l, ...extra });
const cats = (verdict) => [
  { id: 'prof', verdict, metrics: [metric('net_margin', 20, 'good'), metric('roe', null, 'na')] },
  {
    id: 'val',
    verdict: 'warn',
    metrics: [
      metric('pe', 35, 'bad', { lb: 'Caro' }),
      metric('range_52w', 50, 'info', { lo: 80, hi: 120, n: 'En la zona media del rango del año.' }),
      metric('rating', null, 'info', { t: 'Buy' }),
    ],
  },
  { id: 'trend', verdict: 'good', years: ['FY2024', 'FY2025'], metrics: [metric('tr_revenue', 100e9, 'good', { lb: '+12,0 %/año', s: [90e9, 100e9] })] },
  {
    id: 'peers',
    verdict: 'warn',
    peers: { tickers: ['BBB'], names: ['Beta SA'], rows: [{ id: 'pe', mine: 12, values: [20], median: 20, l: 'good', best: -1 }] },
  },
];
const data = {
  generated_at: '2026-10-31T19:00:00',
  universes: [
    { id: 'sp500', label: 'S&P 500', currency: 'USD', count: 2 },
    { id: 'ipsa', label: 'IPSA (Chile)', currency: 'CLP', count: 1 },
  ],
  sections: [
    { id: 'prof', title: 'Rentabilidad', why: 'Mide la ganancia.', labels: ['Bien', 'Atención', 'Alerta'] },
    { id: 'val', title: 'Valoración', why: 'Precio vs. ganancias.', labels: ['Barata', 'Media', 'Cara'] },
    { id: 'trend', title: 'Tendencia (5 años)', why: 'Evolución.', labels: ['Bien', 'Atención', 'Alerta'] },
    { id: 'peers', title: 'Vs. competidores', intro: 'Verde = mejor.', why: 'Comparación.', labels: ['Mejor', 'Similar', 'Peor'] },
  ],
  metrics: {
    net_margin: { name: 'Margen neto', unit: 'pct', why: 'Ganancia por cada 100 vendidos.' },
    roe: { name: 'ROE', unit: 'pct', why: 'Retorno.' },
    pe: { name: 'P/E', unit: 'x', why: 'Precio / utilidad.' },
    range_52w: { name: 'Posición en el rango de 52 semanas', unit: 'pct', why: 'Rango.' },
    rating: { name: 'Consenso de analistas', unit: 'text', why: 'Analistas.' },
    tr_revenue: { name: 'Ingresos', unit: 'money', why: 'Ventas.' },
  },
  companies: [
    { ticker: 'AAA', name: 'Alfa Corp', universe: 'sp500', sector: 'Tech', industry: 'Software', currency: 'USD', price: 100, market_cap: 5e11, as_of: '2026-09-30', verdict: 'good', categories: cats('good') },
    { ticker: 'BBB', name: 'Beta SA', universe: 'ipsa', sector: 'Banks', currency: 'CLP', fin_currency: 'USD', bank: true, price: 5000, market_cap: 2e12, verdict: 'bad', categories: cats('bad') },
    { ticker: 'CCC', name: 'Gamma Inc', universe: 'sp500', sector: 'Tech', currency: 'USD', price: 10, market_cap: 1e9, verdict: 'warn', categories: cats('warn') },
  ],
};
const load = () => Promise.resolve(data);

describe('EvaluatorView: listado', () => {
  it('muestra las empresas ordenadas por capitalización y las cuenta', async () => {
    render(EvaluatorView, { load });
    await screen.findByText('Alfa Corp');
    const tickers = [...document.querySelectorAll('tbody tr strong')].map((e) => e.textContent);
    expect(tickers).toEqual(['AAA', 'CCC']); // arranca en el primer mercado (S&P 500), por capitalización
    expect(screen.getByText('2 de S&P 500 · 1 de IPSA (Chile)')).toBeInTheDocument();
  });

  it('filtra por mercado y por texto', async () => {
    render(EvaluatorView, { load });
    await screen.findByText('Alfa Corp');
    await fireEvent.change(screen.getByLabelText('Mercado'), { target: { value: 'ipsa' } });
    expect(screen.queryByText('Alfa Corp')).toBeNull();
    expect(screen.getByText('Beta SA')).toBeInTheDocument();
    await fireEvent.change(screen.getByLabelText('Mercado'), { target: { value: 'sp500' } });
    await fireEvent.input(screen.getByLabelText('Buscar'), { target: { value: 'gamma' } });
    expect(screen.getAllByRole('row')).toHaveLength(2); // encabezado + 1
  });

  it('avisa cuando nada coincide', async () => {
    render(EvaluatorView, { load });
    await screen.findByText('Alfa Corp');
    await fireEvent.input(screen.getByLabelText('Buscar'), { target: { value: 'zzz' } });
    expect(screen.getByText('Ninguna empresa coincide con los filtros.')).toBeInTheDocument();
  });

  it('informa si no se pudo cargar el JSON', async () => {
    render(EvaluatorView, { load: () => Promise.reject(new Error('x')) });
    expect(await screen.findByText(/No se pudo cargar data\/evaluator.json/)).toBeInTheDocument();
  });
});

describe('EvaluatorView: detalle', () => {
  async function open(name) {
    render(EvaluatorView, { load });
    await screen.findByText('Alfa Corp');
    if (name === 'Beta SA') await fireEvent.change(screen.getByLabelText('Mercado'), { target: { value: 'ipsa' } });
    await fireEvent.click(screen.getByText(name));
  }

  it('muestra las categorías con sus métricas y distintivos', async () => {
    await open('Alfa Corp');
    expect(screen.getByRole('heading', { name: /Rentabilidad/ })).toBeInTheDocument();
    expect(screen.getByText('20,0 %')).toBeInTheDocument(); // margen neto
    expect(screen.getByText('35,00×')).toBeInTheDocument(); // P/E
    expect(screen.getByText('Caro')).toBeInTheDocument(); // distintivo propio
    expect(screen.getByText('Buy')).toBeInTheDocument(); // consenso, un texto
    expect(screen.getByText('En la zona media del rango del año.')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /Evolución de Ingresos/ })).toBeInTheDocument();
    expect(screen.getByText('+12,0 %/año')).toBeInTheDocument();
  });

  it('la tabla de competidores marca el mejor valor', async () => {
    await open('Alfa Corp');
    const table = screen.getByRole('heading', { name: /Vs. competidores/ }).closest('section').querySelector('table');
    const row = within(table).getByText('P/E').closest('tr');
    expect(within(row).getByText('12,0×')).toHaveClass('best');
    expect(within(row).getByText('Mejor')).toBeInTheDocument();
    expect(within(table).getByText('BBB')).toBeInTheDocument();
  });

  it('un banco aclara que caja y deuda no se evalúan, y los montos van en la moneda de los estados financieros', async () => {
    await open('Beta SA');
    expect(screen.getByText(/Banco o aseguradora/)).toBeInTheDocument();
    expect(screen.getByText('100 B USD')).toBeInTheDocument(); // ingresos en USD aunque el precio sea CLP
    expect(screen.getByText(/Estados financieros en USD/)).toBeInTheDocument();
  });

  it('el botón vuelve al listado', async () => {
    await open('Alfa Corp');
    await fireEvent.click(screen.getByRole('button', { name: /Volver al listado/ }));
    await waitFor(() => expect(screen.getByLabelText('Mercado')).toBeInTheDocument());
  });
});

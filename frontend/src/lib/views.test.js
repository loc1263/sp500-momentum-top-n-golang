import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ActualView from './ActualView.svelte';
import BacktestView from './BacktestView.svelte';
import Chart from './Chart.svelte';
import SweepView from './SweepView.svelte';
import CompareView from './CompareView.svelte';
import { period, report, settings } from '../test/fixtures.js';

const universe = { label: 'Todo el índice', ...period(5) };

describe('ActualView', () => {
  const actual = report.actual[0];
  it('lista los picks con retorno y peso igual', () => {
    render(ActualView, { actual });
    expect(screen.getByText('NVDA')).toBeInTheDocument();
    expect(screen.getByText('+50,0%')).toBeInTheDocument();
    expect(screen.getByText('-2,0%')).toBeInTheDocument();
    expect(screen.getAllByText('50,0%')).toHaveLength(3); // tarjeta + una celda por pick
  });
  it('muestra los suplentes aparte, sin peso', () => {
    render(ActualView, { actual });
    expect(screen.getByText('Suplentes')).toBeInTheDocument();
    expect(screen.getByText('TSLA')).toBeInTheDocument();
    expect(screen.getByText('-5,0%')).toBeInTheDocument();
  });
  it('sin suplentes no muestra la sección', () => {
    render(ActualView, { actual: { ...actual, alternates: [] } });
    expect(screen.queryByText('Suplentes')).not.toBeInTheDocument();
  });
  it('calcula el inicio de la ventana restando el plazo', () => {
    render(ActualView, { actual });
    expect(screen.getByText(/2026-06-19 a 2026-10-19/)).toBeInTheDocument();
  });
  it('muestra los cambios contra la corrida anterior', () => {
    render(ActualView, { actual });
    const line = screen.getByText(/Cambios vs\. 2026-10-16/).closest('p');
    expect(line).toHaveTextContent('entran AMD');
    expect(line).toHaveTextContent('salen AAPL');
    expect(line).toHaveTextContent('se mantienen NVDA');
  });
  it('avisa cuando la selección no cambió', () => {
    const same = { ...actual, changes: { entered: [], exited: [], kept: ['NVDA'] } };
    render(ActualView, { actual: same });
    expect(screen.getByText(/ninguno, la selección se mantiene/)).toBeInTheDocument();
  });
  it('avisa en la primera corrida', () => {
    render(ActualView, { actual: { ...actual, previous_as_of: null, changes: null } });
    expect(screen.getByText(/Primera corrida guardada/)).toBeInTheDocument();
  });
  it('muestra el próximo rebalanceo', () => {
    render(ActualView, { actual });
    expect(screen.getByText('Próximo rebalanceo').nextElementSibling).toHaveTextContent('2026-10-01');
  });
  it('no falla sin picks', () => {
    render(ActualView, { actual: { ...actual, picks: [] } });
    expect(screen.getByText(/Top 0 de los últimos 4 meses/)).toBeInTheDocument();
  });
});

describe('Chart', () => {
  const series = [
    { name: 'A', color: 'red', points: [{ date: '2020-01-01', value: 1000 }, { date: '2022-01-01', value: 2000 }] },
  ];
  it('dibuja una línea por serie y la leyenda', () => {
    const { container } = render(Chart, { series });
    expect(container.querySelectorAll('path')).toHaveLength(1);
    expect(screen.getByText('A')).toBeInTheDocument();
  });
  it('no dibuja nada sin puntos', () => {
    const { container } = render(Chart, { series: [{ ...series[0], points: [] }] });
    expect(container.querySelector('svg')).toBeNull();
  });
  it('oculta de la leyenda las series atenuadas', () => {
    render(Chart, { series: [...series, { ...series[0], name: 'tenue', opacity: 0.4 }] });
    expect(screen.queryByText('tenue')).not.toBeInTheDocument();
  });
});

describe('BacktestView', () => {
  it('muestra el capital final del calendario base', () => {
    render(BacktestView, { universe, settings });
    expect(screen.getByText('Capital final').nextElementSibling).toHaveTextContent('US$');
    expect(screen.getByText('Historial de selecciones (calendario base)')).toBeInTheDocument();
  });
  it('arranca con el plazo del config y permite cambiarlo', async () => {
    render(BacktestView, { universe, settings });
    const select = screen.getByLabelText('Plazo');
    expect(select).toHaveValue('4');
    await fireEvent.change(select, { target: { value: '3' } });
    expect(screen.getByText(/ranking 3 meses/)).toBeInTheDocument();
  });
  it('lista un calendario por arranque', () => {
    render(BacktestView, { universe, settings });
    expect(screen.getByText('Calendarios de arranque posibles (3)')).toBeInTheDocument();
  });
});

describe('SweepView', () => {
  it('ordena los plazos por resultado típico y marca el más consistente', () => {
    render(SweepView, { universe, settings });
    const rows = screen.getAllByRole('row').slice(2);
    expect(rows[0]).toHaveTextContent('4 meses');
    expect(rows[0]).toHaveTextContent('más consistente');
    expect(rows[1]).toHaveTextContent('3 meses');
  });
  it('resume el plazo ganador', () => {
    render(SweepView, { universe, settings });
    expect(screen.getByText('Plazo más consistente: 4 meses.')).toBeInTheDocument();
  });
});

describe('CompareView', () => {
  // N-1 pierde en el plazo de 3 meses (CAGR típico 0,05 < 0,1) y gana en el de 4 (0,5 > 0,3)
  const skip = { label: 'Todo el índice', ...period(5) };
  skip.terms = [
    { ...skip.terms[0], summary: { ...skip.terms[0].summary, cagr_median: 0.05 } },
    { ...skip.terms[1], summary: { ...skip.terms[1].summary, cagr_median: 0.5 } },
  ];
  it('compara los dos modelos plazo por plazo y marca el ganador', () => {
    render(CompareView, { universe, skip, settings });
    const rows = screen.getAllByRole('row').slice(2);
    expect(rows[0]).toHaveTextContent('3 meses');
    expect(rows[0]).toHaveTextContent('N-0');
    expect(rows[1]).toHaveTextContent('4 meses');
    expect(rows[1]).toHaveTextContent('N-1');
  });
  it('resume cuántos plazos gana cada modelo', () => {
    render(CompareView, { universe, skip, settings });
    expect(screen.getByText(/Empate: cada modelo gana en 1 de 2 plazos/)).toBeInTheDocument();
  });
  it('avisa si falta el modelo skip-month', () => {
    render(CompareView, { universe, skip: undefined, settings });
    expect(screen.getByText(/Sin resultados del modelo skip-month/)).toBeInTheDocument();
  });
});

describe('CompareView: mitades', () => {
  const half = (first, second) => ({ split: '2023-01-01', first, second });
  const skip = { label: 'Todo el índice', ...period(5) };
  skip.terms = [
    { ...skip.terms[0], summary: { ...skip.terms[0].summary, halves: half(0.3, 0.0) } },
    { ...skip.terms[1], summary: { ...skip.terms[1].summary, halves: half(0.4, 0.6) } },
  ];
  // N-0 (fixtures): 3 meses = 0,1 / 0,05 ; 4 meses = 0,3 / 0,2
  it('marca si el mismo modelo gana las dos mitades', () => {
    render(CompareView, { universe, skip, settings });
    const rows = screen.getAllByRole('row').filter((r) => r.textContent.includes('en las dos'));
    expect(rows).toHaveLength(1);
    expect(rows[0]).toHaveTextContent('4 meses');
    expect(rows[0]).toHaveTextContent('N-1 en las dos');
    expect(screen.getByText(/N-0 gana las dos mitades en 0 de 2 plazos, N-1 en 1/)).toBeInTheDocument();
  });
  it('sin datos de mitades no muestra la sección', () => {
    const strip = (u) => ({ ...u, terms: u.terms.map((t) => ({ ...t, summary: { ...t.summary, halves: undefined } })) });
    render(CompareView, { universe: strip(universe), skip: strip(skip), settings });
    expect(screen.queryByText('¿Se sostiene en el tiempo?')).not.toBeInTheDocument();
  });
});

describe('ActualView: seguimiento', () => {
  it('lista las selecciones anteriores con su retorno y estado', () => {
    render(ActualView, { actual: report.actual[0] });
    expect(screen.getByText('Seguimiento de selecciones anteriores')).toBeInTheDocument();
    expect(screen.getByText('cerrada')).toBeInTheDocument();
    expect(screen.getByText('+10,0%')).toBeInTheDocument();
  });
  it('sin seguimiento no muestra la sección', () => {
    render(ActualView, { actual: { ...report.actual[0], tracking: [] } });
    expect(screen.queryByText('Seguimiento de selecciones anteriores')).not.toBeInTheDocument();
  });
});

import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import QuantView from './QuantView.svelte';
import { report } from '../test/fixtures.js';

const quant = report.quant;
// La tabla del ranking es la última de la página
const rankingTable = () => [...document.querySelectorAll('table')].at(-1);
const bodyRows = () => within(rankingTable()).getAllByRole('row').slice(1);
const rankingRows = () => bodyRows().map((r) => within(r).getAllByRole('cell')[1].textContent.trim());
const rowOf = (ticker) => bodyRows().find((r) => within(r).getAllByRole('cell')[1].textContent.trim() === ticker);

describe('QuantView', () => {
  it('resume la cobertura del ranking', () => {
    render(QuantView, { quant });
    expect(screen.getByText('3 de 4')).toBeInTheDocument();
    expect(screen.getByText(/Sin un año de precios .*: NEW/)).toBeInTheDocument();
    expect(screen.getByText(/Sin datos técnicos: OLD/)).toBeInTheDocument();
  });

  it('muestra el top con el peso de cada cartera de Markowitz', () => {
    render(QuantView, { quant });
    const table = screen.getByRole('heading', { name: /mejores y sus pesos/ }).parentElement.querySelector('details.more + .wrap');
    expect(within(table).getByRole('columnheader', { name: 'Mínima varianza' })).toBeInTheDocument();
    const mu = within(table).getAllByRole('row').find((r) => within(r).queryByText('MU'));
    expect(mu).toHaveTextContent('60,0%');
    expect(mu).toHaveTextContent('30,0%');
    expect(within(table).getByText('Sharpe (tasa libre de riesgo de 4%)')).toBeInTheDocument();
  });

  it('muestra el peso por sector y marca los que llegan al tope', () => {
    render(QuantView, { quant });
    const table = screen.getByRole('heading', { name: 'Peso por sector' }).nextElementSibling;
    const tech = within(table).getAllByRole('row').find((r) => within(r).queryByText('Technology'));
    expect(tech).toHaveTextContent('40,0%');
    expect(within(tech).getByText('40,0%')).toHaveClass('atcap');
    expect(within(tech).getByText('30,0%')).not.toHaveClass('atcap');
    expect(screen.queryByText(/el tope se subió/)).not.toBeInTheDocument();
  });

  it('avisa si el tope por sector tuvo que subirse', () => {
    render(QuantView, { quant: { ...quant, markowitz: { ...quant.markowitz, sector_relaxed: true, sector_max: 0.55 } } });
    expect(screen.getByText(/el tope se subió a 55%/)).toBeInTheDocument();
  });

  it('avisa si no se alcanza la volatilidad objetivo', () => {
    const only = { ...quant, markowitz: { ...quant.markowitz, portfolios: quant.markowitz.portfolios.slice(1), main: 'min_var' } };
    render(QuantView, { quant: only });
    expect(screen.getByText(/No hay cartera de máximo retorno/)).toBeInTheDocument();
  });

  it('muestra «n/d» donde falta el dato financiero y cuántos grupos se midieron', () => {
    render(QuantView, { quant });
    expect(rowOf('JPM')).toHaveTextContent('n/d');                 // sin deuda/patrimonio ni liquidez
  });

  it('muestra el score técnico y los indicadores', () => {
    render(QuantView, { quant });
    const mu = rowOf('MU');
    expect(mu).toHaveTextContent('90,5');       // score técnico
    expect(mu).toHaveTextContent('63,9');       // RSI
    expect(mu).toHaveTextContent('+40%');       // distancia a la EMA 200
    expect(screen.getByRole('button', { name: /Técnico/ })).toBeInTheDocument();
  });

  it('muestra los indicadores de flujo de caja y marca los que faltan', () => {
    render(QuantView, { quant });
    for (const name of ['FCF/Ventas', 'FCF/Cap.', 'FCF/Utilidad', 'Años FCF+']) {
      expect(screen.getByRole('button', { name })).toBeInTheDocument();
    }
    const mu = rowOf('MU');
    expect(mu).toHaveTextContent('+3%');       // FCF sobre capitalización
    expect(mu).toHaveTextContent('+34%');      // conversión de la utilidad en caja
    expect(mu).toHaveTextContent('75%');       // años con flujo libre positivo
    expect(rowOf('JPM')).toHaveTextContent('n/d');
  });

  it('cambia el detalle para ver el score de cada grupo de salud', async () => {
    render(QuantView, { quant });
    expect(screen.queryByRole('button', { name: 'Solvencia' })).not.toBeInTheDocument();
    await fireEvent.change(screen.getByLabelText('Detalle'), { target: { value: 'salud' } });
    for (const name of ['Grupos medidos', 'Rentabilidad', 'Solvencia', 'Liquidez', 'Caja']) {
      expect(screen.getByRole('button', { name })).toBeInTheDocument();
    }
    expect(rowOf('MU')).toHaveTextContent('4/4');
    expect(rowOf('MU')).toHaveTextContent('95,0');                 // rentabilidad
    const jpm = rowOf('JPM');
    expect(jpm).toHaveTextContent('1/4');
    expect(jpm).toHaveTextContent('n/d');                          // grupos sin medir
  });

  it('cambia el detalle para ver el score de cada grupo técnico y ordena por él', async () => {
    render(QuantView, { quant });
    await fireEvent.change(screen.getByLabelText('Detalle'), { target: { value: 'tecnico' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Tendencia' }));
    expect(rankingRows()).toEqual(['AMD', 'MU', 'JPM']);          // de mayor a menor tendencia
    await fireEvent.change(screen.getByLabelText('Detalle'), { target: { value: 'indicadores' } });
    expect(rankingRows()).toEqual(['MU', 'AMD', 'JPM']);              // la columna ya no existe: vuelve al ranking
  });

  it('filtra por sector y por texto', async () => {
    render(QuantView, { quant });
    expect(rankingRows()).toEqual(['MU', 'AMD', 'JPM']);
    await fireEvent.change(screen.getByLabelText('Sector'), { target: { value: 'Technology' } });
    expect(rankingRows()).toEqual(['MU', 'AMD']);
    await fireEvent.input(screen.getByLabelText('Buscar'), { target: { value: 'micron' } });
    expect(rankingRows()).toEqual(['MU']);
  });

  it('ordena el ranking al hacer clic en una columna', async () => {
    render(QuantView, { quant });
    await fireEvent.click(screen.getByRole('button', { name: /Salud/ }));
    expect(rankingRows()[0]).toBe('MU'); // numérico: de mayor a menor la primera vez
    await fireEvent.click(screen.getByRole('button', { name: /Salud/ }));
    expect(rankingRows()[0]).toBe('JPM');
  });
});

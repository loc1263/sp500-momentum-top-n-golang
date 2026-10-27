import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import EtfqView from './EtfqView.svelte';
import { report } from '../test/fixtures.js';

const etfq = report.etfq;
const mainRows = () => screen.getAllByRole('table')[1].querySelectorAll('tbody tr');

describe('EtfqView', () => {
  it('lista primero los ETFs con score final, de mayor a menor posición, con TER, AUM y retornos', () => {
    render(EtfqView, { etfq });
    const rows = mainRows();
    expect([...rows].map((r) => r.querySelectorAll('td')[1].textContent.trim())).toEqual(['VOO', 'IVV']);
    expect(rows[0]).toHaveTextContent('Vanguard S&P 500 ETF');
    expect(rows[0]).toHaveTextContent('37,5'); // score final
    expect(rows[0]).toHaveTextContent('0,03%');
    expect(rows[0]).toHaveTextContent('1.700,0 B');
    expect(rows[0]).toHaveTextContent('+20,0%');
    expect(rows[1]).toHaveTextContent('0,09%');
  });

  it('muestra el resumen por gestora y el filtro de elegibilidad', () => {
    render(EtfqView, { etfq });
    const summary = screen.getAllByRole('table')[0];
    expect(within(summary).getByText('iShares').closest('tr')).toHaveTextContent('40');
    expect(within(summary).getByText('Vanguard').closest('tr')).toHaveTextContent('30');
    expect(screen.getByText('TER ≤ 0,20%')).toBeInTheDocument();
    expect(screen.getByText('AUM ≥ US$ 100 M')).toBeInTheDocument();
    expect(screen.getByText('Referencia sin riesgo').nextElementSibling).toHaveTextContent('VUSB');
  });

  it('con «Solo con score final» desmarcado muestra los excluidos con su motivo', async () => {
    render(EtfqView, { etfq });
    await fireEvent.click(screen.getByRole('checkbox', { name: /Solo con score final/ }));
    const rows = mainRows();
    expect(rows).toHaveLength(4);
    expect(rows[2]).toHaveTextContent('CARO');
    expect(rows[2]).toHaveTextContent('TER > 0,20%');
    expect(rows[3]).toHaveTextContent('CHICO');
    expect(rows[3]).toHaveTextContent('AUM < 100 M');
    expect(rows[2]).toHaveTextContent('–'); // sin puesto
  });

  it('filtra por gestora y por categoría', async () => {
    render(EtfqView, { etfq });
    await fireEvent.change(screen.getByLabelText('Gestora'), { target: { value: 'iShares' } });
    expect([...mainRows()].map((r) => r.querySelectorAll('td')[1].textContent.trim())).toEqual(['IVV']);
    await fireEvent.change(screen.getByLabelText('Gestora'), { target: { value: '' } });
    await fireEvent.change(screen.getByLabelText('Categoría'), { target: { value: 'Sectores' } });
    expect(screen.getByText('Ningún ETF cumple el filtro.')).toBeInTheDocument(); // los de Sectores están excluidos
  });

  it('ordena al hacer clic: el TER de menor a mayor primero', async () => {
    render(EtfqView, { etfq });
    await fireEvent.click(screen.getByRole('button', { name: /^TER/ }));
    expect([...mainRows()].map((r) => r.querySelectorAll('td')[1].textContent.trim())).toEqual(['VOO', 'IVV']);
    await fireEvent.click(screen.getByRole('button', { name: /^TER/ }));
    expect([...mainRows()].map((r) => r.querySelectorAll('td')[1].textContent.trim())).toEqual(['IVV', 'VOO']);
  });

  it('tiene una columna de retorno por plazo', () => {
    render(EtfqView, { etfq });
    expect(screen.getByRole('button', { name: /Ret\. 3m/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Ret\. 12m/ })).toBeInTheDocument();
  });

  it('enlaza los ETFs a la sección de ETFs de stockanalysis.com', () => {
    render(EtfqView, { etfq });
    expect(screen.getByRole('link', { name: 'VOO' })).toHaveAttribute('href', 'https://stockanalysis.com/etf/voo/');
  });

  it('aclara cuando no hay tope de TER ni mínimo de AUM', () => {
    render(EtfqView, { etfq: { ...etfq, settings: { ...etfq.settings, max_ter: 0, min_aum_millions: 0 } } });
    expect(screen.getByText('Sin tope de TER')).toBeInTheDocument();
    expect(screen.getByText('Sin mínimo de AUM')).toBeInTheDocument();
  });
});

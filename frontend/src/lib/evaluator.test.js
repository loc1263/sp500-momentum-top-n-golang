import { describe, expect, it } from 'vitest';
import { filterCompanies, formatMetric, money, pillText, sortCompanies } from './evaluator.js';

describe('money', () => {
  it('abrevia en M y B (miles de millones) y agrega la moneda', () => {
    expect(money(2.5e10, 'USD')).toBe('25,0 B USD');
    expect(money(4.2e8)).toBe('420 M');
    expect(money(-1.5e9)).toBe('-1,5 B');
    expect(money(null)).toBe('n/d');
  });
});

describe('formatMetric', () => {
  it('formatea por unidad', () => {
    expect(formatMetric({ id: 'pe', v: 12.345 }, { unit: 'x' })).toBe('12,35×');
    expect(formatMetric({ id: 'roe', v: 18.04 }, { unit: 'pct' })).toBe('18,0 %');
    expect(formatMetric({ id: 'net_debt_years', v: 0.35 }, { unit: 'years' })).toBe('0,4 años');
    expect(formatMetric({ id: 'beta', v: 1.073 }, { unit: 'num' })).toBe('1,07');
  });
  it('el potencial al precio objetivo lleva signo', () => {
    expect(formatMetric({ id: 'target_upside', v: 10.7 }, { unit: 'pct' })).toBe('+10,7 %');
    expect(formatMetric({ id: 'target_upside', v: -3.55 }, { unit: 'pct' })).toBe('-3,6 %');
  });
  it('con caja neta muestra la caja en positivo y la moneda de los estados financieros', () => {
    expect(formatMetric({ id: 'net_debt', v: -3e10, lb: 'Caja neta' }, { unit: 'money' }, 'USD')).toBe('30,0 B USD');
    expect(formatMetric({ id: 'net_debt', v: 3e10 }, { unit: 'money' }, 'USD')).toBe('30,0 B USD');
  });
  it('el texto manda sobre el número y lo que falta es n/d', () => {
    expect(formatMetric({ id: 'rating', v: null, t: 'Buy' }, { unit: 'text' })).toBe('Buy');
    expect(formatMetric({ id: 'pe', v: null }, { unit: 'x' })).toBe('n/d');
  });
});

describe('pillText', () => {
  const labels = ['Barata', 'Media', 'Cara'];
  it('usa el texto propio, si no el de la sección según el nivel', () => {
    expect(pillText('bad', 'Caro', labels)).toBe('Caro');
    expect(pillText('bad', '', labels)).toBe('Cara');
    expect(pillText('good', '', undefined)).toBe('Bien');
    expect(pillText('info', '', labels)).toBe('Informativo');
    expect(pillText('na', '', labels)).toBe('Sin datos');
  });
});

const companies = [
  { ticker: 'AAA', name: 'Alfa Corp', universe: 'sp500', sector: 'Tech', verdict: 'warn', market_cap: 100 },
  { ticker: 'BBB', name: 'Beta SA', universe: 'ipsa', sector: 'Banks', verdict: 'good', market_cap: 500 },
  { ticker: 'CCC', name: 'Gamma', universe: 'sp500', sector: 'Tech', verdict: 'bad', market_cap: null },
];

describe('filterCompanies / sortCompanies', () => {
  it('filtra por mercado, sector, veredicto y texto (ticker o nombre)', () => {
    expect(filterCompanies(companies, { universe: 'sp500' }).map((c) => c.ticker)).toEqual(['AAA', 'CCC']);
    expect(filterCompanies(companies, { sector: 'Banks' }).map((c) => c.ticker)).toEqual(['BBB']);
    expect(filterCompanies(companies, { verdict: 'bad' }).map((c) => c.ticker)).toEqual(['CCC']);
    expect(filterCompanies(companies, { text: ' ALF ' }).map((c) => c.ticker)).toEqual(['AAA']);
    expect(filterCompanies(companies, { text: 'bbb' }).map((c) => c.ticker)).toEqual(['BBB']);
  });
  it('ordena por capitalización con los sin dato al final y por veredicto de mejor a peor', () => {
    expect(sortCompanies(companies, 'market_cap', true).map((c) => c.ticker)).toEqual(['BBB', 'AAA', 'CCC']);
    expect(sortCompanies(companies, 'market_cap', false).map((c) => c.ticker)).toEqual(['AAA', 'BBB', 'CCC']);
    expect(sortCompanies(companies, 'verdict', false).map((c) => c.ticker)).toEqual(['BBB', 'AAA', 'CCC']);
    expect(sortCompanies(companies, 'name', false).map((c) => c.ticker)).toEqual(['AAA', 'BBB', 'CCC']);
  });
});

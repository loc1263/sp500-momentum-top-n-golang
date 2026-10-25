import { describe, expect, it } from 'vitest';
import { billions, int, pct, share, sign, usd } from './format.js';

describe('pct', () => {
  it('lleva signo y coma decimal', () => {
    expect(pct(0.123)).toBe('+12,3%');
    expect(pct(-0.05)).toBe('-5,0%');
  });
  it('trata el cero como positivo', () => expect(pct(0)).toBe('+0,0%'));
  it('respeta los decimales pedidos', () => expect(pct(0.1234, 0)).toBe('+12%'));
  it('devuelve n/d sin dato', () => {
    expect(pct(null)).toBe('n/d');
    expect(pct(undefined)).toBe('n/d');
  });
});

describe('share', () => {
  it('no lleva signo', () => expect(share(0.25)).toBe('25,0%'));
});

describe('usd', () => {
  it('usa dos decimales y separador de miles', () => expect(usd(1234.5)).toBe('1.234,50'));
  it('devuelve n/d sin dato', () => expect(usd(null)).toBe('n/d'));
});

describe('billions', () => {
  it('convierte a miles de millones', () => expect(billions(2.5e12)).toBe('2.500'));
  it('devuelve n/d sin dato', () => expect(billions(null)).toBe('n/d'));
});

describe('int', () => {
  it('redondea sin decimales', () => expect(int(1234.6)).toBe('1.235'));
});

describe('sign', () => {
  it('elige la clase por signo', () => {
    expect(sign(1)).toBe('pos');
    expect(sign(-1)).toBe('neg');
    expect(sign(0)).toBe('');
  });
});

import { dec, int } from './format.js';

/** Veredictos de mejor a peor: sirven para ordenar y filtrar. */
export const LEVEL_ORDER = { good: 0, warn: 1, bad: 2, na: 3 };
export const VERDICT_LABEL = { good: 'Bien', warn: 'Atención', bad: 'Alerta', na: 'Sin datos' };

/** Monto abreviado en la moneda dada: 2.5e10 -> "25,0 B USD" (B = miles de millones, como en el resto del sitio). */
export function money(v, currency = '') {
  if (v == null) return 'n/d';
  const a = Math.abs(v);
  const body = a >= 1e9 ? `${dec(a / 1e9, a >= 1e11 ? 0 : 1)} B` : a >= 1e6 ? `${dec(a / 1e6, a >= 1e8 ? 0 : 1)} M` : int(a);
  return `${v < 0 ? '-' : ''}${body}${currency ? ` ${currency}` : ''}`;
}

/** Precio con dos decimales y moneda. */
export const price = (v, currency = '') => (v == null ? 'n/d' : `${dec(v, 2)}${currency ? ` ${currency}` : ''}`);

/**
 * Valor de una métrica como texto. `def` es su definición del diccionario (unidad) y `finCurrency` la moneda de los
 * estados financieros (distinta de la del precio en algunas acciones chilenas).
 */
export function formatMetric(m, def, finCurrency = '') {
  if (m.t) return m.t;
  if (m.v == null) return 'n/d';
  switch (def?.unit) {
    case 'pct':
      // los que pueden ser negativos o son una variación van con signo
      return m.id === 'target_upside' ? `${m.v > 0 ? '+' : m.v < 0 ? '-' : ''}${dec(Math.abs(m.v), 1)} %` : `${dec(m.v, 1)} %`;
    case 'x':
      return `${dec(m.v, 2)}×`;
    case 'years':
      return `${dec(m.v, 1)} años`;
    case 'money':
      // con más caja que deuda se muestra la caja neta en positivo
      return money(m.lb === 'Caja neta' ? Math.abs(m.v) : m.v, finCurrency);
    default:
      return dec(m.v, 2);
  }
}

/** Texto del distintivo de una métrica o categoría: el propio si lo trae, si no el de su sección según el nivel. */
export function pillText(level, own, labels) {
  if (own) return own;
  if (level === 'info') return 'Informativo';
  if (level === 'na') return 'Sin datos';
  return labels?.[{ good: 0, warn: 1, bad: 2 }[level]] ?? VERDICT_LABEL[level];
}

/** Posición (0-100) de un valor entre dos extremos, para dibujar las series. */
export function scaleY(v, min, max, top, bottom) {
  const span = max === min ? 1 : max - min;
  return bottom - ((v - min) / span) * (bottom - top);
}

/** Empresas filtradas y ordenadas para el listado. */
export function filterCompanies(companies, { universe = '', text = '', verdict = '', sector = '' }) {
  const q = text.trim().toLowerCase();
  return companies.filter(
    (c) =>
      (!universe || c.universe === universe) &&
      (!sector || c.sector === sector) &&
      (!verdict || c.verdict === verdict) &&
      (!q || c.ticker.toLowerCase().includes(q) || c.name.toLowerCase().includes(q)),
  );
}

export function sortCompanies(companies, key, descending) {
  const direction = descending ? -1 : 1;
  const value = (c) => (key === 'verdict' ? LEVEL_ORDER[c.verdict] : key === 'market_cap' ? c.market_cap : c[key]);
  return companies.toSorted((a, b) => {
    const x = value(a);
    const y = value(b);
    if (x == null || y == null) return (x == null) - (y == null); // sin dato, al final
    return direction * (typeof x === 'string' ? x.localeCompare(y, 'es') : x - y);
  });
}

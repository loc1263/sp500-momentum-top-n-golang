const formatter = (digits) =>
  new Intl.NumberFormat('es-AR', { minimumFractionDigits: digits, maximumFractionDigits: digits });
const n0 = formatter(0);
const n2 = formatter(2);

/** Fracción como porcentaje con signo: 0.123 -> "+12,3%". */
export const pct = (v, digits = 1) =>
  v == null ? 'n/d' : `${v >= 0 ? '+' : '-'}${formatter(digits).format(Math.abs(v * 100))}%`;
/** Fracción como porcentaje sin signo (pesos, participaciones): 0.25 -> "25,0%". */
export const share = (v, digits = 1) => formatter(digits).format(v * 100) + '%';
/** Número con decimales fijos y coma: 0.0945 -> "0,09" (dec(v, 2)). */
export const dec = (v, digits = 1) => formatter(digits).format(v);
/** Patrimonio (AUM) abreviado: 1.7e12 -> "1.729,3 B", 5e8 -> "500 M" (B = miles de millones). */
export const aum = (v) => (v == null ? 'n/d' : v >= 1e9 ? `${formatter(1).format(v / 1e9)} B` : `${n0.format(v / 1e6)} M`);
export const usd = (v) => (v == null ? 'n/d' : n2.format(v));
export const billions = (v) => (v == null ? 'n/d' : n0.format(v / 1e9));
export const int = (v) => n0.format(v);
/** Clase CSS según el signo (verde / rojo). */
export const sign = (v) => (v > 0 ? 'pos' : v < 0 ? 'neg' : '');
export const PAGE_SIZE = 12;
export const slicePage = (rows, page) => rows.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

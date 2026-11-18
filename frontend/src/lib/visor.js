// Lógica de la pestaña Visor (sin DOM, para probarla aparte): formato de celdas, color, búsqueda, orden y CSV.

const TEXT = new Set(['ticker', 'nombre', 'sector', 'categoria', 'vs_spy']);
const OPS = { '>': (a, b) => a > b, '>=': (a, b) => a >= b, '<=': (a, b) => a <= b };

/** Encabezados de las columnas (las que no están acá se muestran con su nombre). */
export const LABELS = {
  ticker: 'Ticker', nombre: 'Nombre', sector: 'Sector', categoria: 'Categoría', market_cap_B: 'Cap. (B)', aum_B: 'AUM (B)',
  expense_ratio: 'TER %', sharpe: 'Sharpe', beta: 'Beta', max_drawdown: 'Máx. caída 5a', dist_max52: 'Dist. máx 52s',
  vs_ma200: 'vs MA200', rel_spy_1y: 'vs SPY 1a', rel_spy_3y: 'vs SPY 3a', dividend_yield: 'Dividendo', ytd: 'YTD', ytd_spy: 'YTD SPY',
  ytd_vs_spy: 'YTD vs SPY', vs_spy: 'vs SPY', verdes: 'Verdes',
};

/** Encabezado de una columna: «ret_3m» → «3m», «ret_1y» → «1a». */
export function label(col) {
  const m = /^ret_(\d+)([my])$/.exec(col);
  if (m) return m[1] + (m[2] === 'y' ? 'a' : 'm');
  return LABELS[col] ?? col;
}

export const isText = (col) => TEXT.has(col);

/** Texto de una celda según el formato de su columna ('pct', 'dec', 'int'); vacío si no hay dato. */
export function fmt(sec, col, v) {
  if (v === null || v === undefined) return '';
  switch (sec.fmt[col]) {
    case 'pct': return `${v.toFixed(1)}%`;
    case 'dec': return v.toFixed(2);
    case 'int': return v.toLocaleString('es', { maximumFractionDigits: 0 });
    default: return String(v);
  }
}

/** Color de una celda: 'pos' (verde), 'neg' (rojo) o '' según el criterio de su columna. */
export function cellClass(sec, col, v) {
  if (v === null || v === undefined) return '';
  if (sec.sign.includes(col)) return v > 0 ? 'pos' : v < 0 ? 'neg' : '';
  if (sec.thr[col]) return OPS[sec.thr[col][0]](v, sec.thr[col][1]) ? 'pos' : '';
  if (col === 'vs_spy') return v === 'SUPERA' ? 'pos' : v === 'NO SUPERA' ? 'neg' : '';
  return '';
}

/** Filas cuyo ticker, nombre, sector, categoría o vs_spy contienen el texto buscado. */
export function filterRows(sec, query) {
  const q = query.trim().toLowerCase();
  if (!q) return sec.rows;
  return sec.rows.filter((r) => sec.search.some((i) => String(r[i] ?? '').toLowerCase().includes(q)));
}

/** Filas ordenadas por la columna `index`; los vacíos van siempre al final. Sin columna, el orden original. */
export function sortRows(rows, index, descending) {
  if (index === null) return rows;
  const d = descending ? -1 : 1;
  return [...rows].sort((a, b) => {
    const x = a[index], y = b[index];
    if (x === null && y === null) return 0;
    if (x === null) return 1;
    if (y === null) return -1;
    return (typeof x === 'number' ? x - y : String(x).localeCompare(String(y), 'es')) * d;
  });
}

/** CSV (con comas y comillas escapadas) de las filas, con los nombres de columna originales. */
export function toCsv(sec, rows) {
  const cell = (v) => {
    if (v === null || v === undefined) return '';
    const s = String(v);
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  return [sec.cols, ...rows].map((r) => r.map(cell).join(',')).join('\n');
}

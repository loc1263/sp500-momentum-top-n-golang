// Datos mínimos con la forma del JSON de data/report.json

export const settings = { top_n: 2, months: 4, backtest_years: 5, initial_capital: 1000 };

const equity = (values) => values.map((capital, i) => ({ date: `202${i}-06-30`, capital }));

const calendar = (offset, final, cagr) => ({
  offset,
  start_date: '2020-01-31',
  end_date: '2022-12-31',
  first_rebalance: '2020-02-28',
  rebalances: 6,
  final_capital: final,
  total_return: final / 1000 - 1,
  cagr,
  equity: equity([1000, (1000 + final) / 2, final]),
  history: [
    {
      date: '2020-06-30', capital: 1000, gain: 0,
      picks: [{ ticker: 'NVDA', return: 0.5 }, { ticker: 'BRK-B', return: -0.1 }],
    },
  ],
});

const term = (months, cagrs) => ({
  months,
  summary: {
    calendars: cagrs.length,
    cagr_median: cagrs[0],
    cagr_min: Math.min(...cagrs),
    cagr_max: Math.max(...cagrs),
    capital_median: 1500,
    capital_min: 1200,
    capital_max: 1900,
    halves: { split: '2023-01-01', first: cagrs[0], second: cagrs[1] },
  },
  calendars: cagrs.map((c, i) => calendar(i, 1000 * (1 + c) ** 3, c)),
});

export const period = (years) => ({
  years,
  start_date: `${2025 - years}-01-31`,
  end_date: '2025-01-31',
  terms: [term(3, [0.1, 0.05, 0.2]), term(4, [0.3, 0.2, 0.4])],
});

export const report = {
  run_date: '2026-10-20',
  generated_at: '2026-10-20T10:00:00',
  settings,
  actual: [
    {
      min_cap_billions: 0, label: 'Todo el índice', as_of: '2026-10-19', months: 4,
      previous_as_of: '2026-10-16', anchor: '2026-06-18', next_rebalance: '2026-10-01',
      changes: { entered: ['AMD'], exited: ['AAPL'], kept: ['NVDA'] },
      picks: [
        { rank: 1, ticker: 'NVDA', return: 0.5, market_cap: 4e12, price: 180.5 },
        { rank: 2, ticker: 'BRK-B', return: -0.02, market_cap: 1e12, price: 480 },
      ],
      alternates: [{ rank: 3, ticker: 'TSLA', return: -0.05, market_cap: 8e11, price: 250 }],
      tracking: [
        {
          as_of: '2026-06-18', until: '2026-10-18', days: 122, closed: true, return: 0.1,
          picks: [{ ticker: 'NVDA', return: 0.3 }, { ticker: 'AMD', return: -0.1 }],
        },
      ],
    },
  ],
  etf: {
    as_of: '2026-10-19', run_at: '2026-10-19T10:00:00', top_n: 2, horizons: [3, 12], risk_free: 'VUSB',
    previous_as_of: '2026-10-16', changes: { entered: ['AMD'], exited: ['VTV'], kept: ['VONV'] },
    rows: [
      { rank: 1, ticker: 'VONV', name: 'Russell 1000 Value', category: 'Acciones EE. UU. estilo', score: 1.67,
        vol: 0.111, max_dd: -0.068, sma_dist: 0.096, returns: { 3: 0.045, 12: 0.291 } },
      { rank: 2, ticker: 'VDE', name: 'Energy', category: 'Sectores', score: 1.66, vol: 0.219, max_dd: -0.15,
        sma_dist: 0.171, returns: { 3: 0.136, 12: 0.601 } },
      { rank: 3, ticker: 'BND', name: 'Total Bond Market', category: 'Bonos EE. UU.', score: -0.4, vol: 0.05,
        max_dd: -0.04, sma_dist: -0.01, returns: { 3: -0.01, 12: 0.02 } },
    ],
  },
  quant: {
    as_of: '2026-10-19', generated_at: '2026-10-19T10:00:00',
    settings: { top_n: 2, vol_target: 0.3, w_max: 0.25, sector_max: 0.4, mu_shrink: 0.25, risk_free: 0.04, prior: 0.09, weights: [1, 1, 1] },
    coverage: { universe: 4, momentum: 3, health: 4, technical: 3, ranked: 3,
      excluded: { sin_momentum: ['NEW'], sin_datos_financieros: [], sin_datos_tecnicos: ['OLD'] } },
    ranking: [
      { rank: 1, ticker: 'MU', name: 'Micron Technology', sector: 'Technology', market_cap: 9.9e11, score: 99.4, momentum: 99.6,
        health: 99.2, health_groups: { rentabilidad: 95, solvencia: 99, liquidez: 98, caja: 99 },
        technical_groups: { tendencia: 80, impulso: 90, volumen: 85, posicion: 99 }, technical: 90.5, rsi: 63.9, vs_ema50: 0.12, vs_ema200: 0.4, macd_hist: 0.011, groups: 4, mom_12_1: 4.45, mom_6_1: 1.17, roe: 0.88, operating_margin: 0.8, debt_to_equity: 3.7,
        net_debt_ebitda: 0, current_ratio: 3.3, fcf_margin: 0.22, fcf_yield: 0.03, cash_conversion: 0.34, fcf_positive_years: 0.75, weight: 0.6 },
      { rank: 2, ticker: 'AMD', name: 'Advanced Micro Devices', sector: 'Technology', market_cap: 8.4e11, score: 93.8,
        momentum: 98.6, health: 89, health_groups: { rentabilidad: 60, solvencia: 90, liquidez: 80, caja: 85 },
        technical_groups: { tendencia: 95, impulso: 70, volumen: 60, posicion: 90 }, technical: 80, rsi: 55, vs_ema50: -0.02, vs_ema200: 0.1, macd_hist: -0.004, groups: 4, mom_12_1: 1.7, mom_6_1: 0.8, roe: 0.1, operating_margin: 0.12,
        debt_to_equity: 5, net_debt_ebitda: 0, current_ratio: 2.5, fcf_margin: 0.15, fcf_yield: 0.02, cash_conversion: 1.1, fcf_positive_years: 1, weight: 0.4 },
      { rank: 3, ticker: 'JPM', name: 'JPMorgan Chase', sector: 'Financial Services', market_cap: 8.8e11, score: 73.2,
        momentum: 73.3, health: 73.1, health_groups: { rentabilidad: 73, solvencia: null, liquidez: null, caja: null },
        technical_groups: { tendencia: 50, impulso: 60, volumen: 55, posicion: 70 }, technical: 60.2, rsi: 48, vs_ema50: 0.01, vs_ema200: 0.05, macd_hist: 0.0, groups: 1, mom_12_1: 0.2, mom_6_1: 0.1, roe: 0.18, operating_margin: 0.43,
        debt_to_equity: null, net_debt_ebitda: null, current_ratio: null, fcf_margin: null, fcf_yield: null, cash_conversion: null, fcf_positive_years: null, weight: 0 },
    ],
    markowitz: {
      min_vol: 0.25, vol_target: 0.3, w_max: 0.25, sector_max: 0.4, sector_relaxed: false, main: 'max_return',
      tickers: [{ ticker: 'MU', exp_return: 0.32, vol: 0.81 }, { ticker: 'AMD', exp_return: 0.32, vol: 0.68 }],
      portfolios: [
        { key: 'max_return', label: 'Máximo retorno (volatilidad hasta 30%)', exp_return: 0.25, vol: 0.3, sharpe: 0.71,
          weights: [{ ticker: 'MU', weight: 0.6 }, { ticker: 'AMD', weight: 0.4 }],
          sectors: [{ sector: 'Technology', weight: 0.4 }, { sector: 'Industrials', weight: 0.6 }] },
        { key: 'min_var', label: 'Mínima varianza', exp_return: 0.2, vol: 0.25, sharpe: 0.64,
          weights: [{ ticker: 'AMD', weight: 0.7 }, { ticker: 'MU', weight: 0.3 }],
          sectors: [{ sector: 'Industrials', weight: 0.7 }, { sector: 'Technology', weight: 0.3 }] },
      ],
    },
  },
  backtest: [
    { min_cap_billions: 0, label: 'Todo el índice', periods: [period(3), period(5)] },
  ],
};

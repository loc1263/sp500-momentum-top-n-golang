// Package config es la configuración tipada: una struct por sección de config.toml, con los valores por defecto en
// Default(). Los tags json coinciden con los nombres de las secciones y claves porque la configuración de cada corrida
// del backtest se guarda en SQLite como JSON (tabla runs.params) y el informe la lee de ahí.
package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"top3/internal/errs"
)

// Data es [data]: histórico, base SQLite y parámetros de las descargas (Wikipedia, CSV histórico y Yahoo).
type Data struct {
	HistoryYears        float64 `toml:"history_years" json:"history_years"`
	DBPath              string  `toml:"db_path" json:"db_path"`
	HistoricalURL       string  `toml:"historical_url" json:"historical_url"`
	WikipediaURL        string  `toml:"wikipedia_url" json:"wikipedia_url"`
	WikipediaChangesURL string  `toml:"wikipedia_changes_url" json:"wikipedia_changes_url"`
	UserAgent           string  `toml:"user_agent" json:"user_agent"`
	HTTPTimeout         int     `toml:"http_timeout" json:"http_timeout"`
	HistoricalTimeout   int     `toml:"historical_timeout" json:"historical_timeout"`
	BatchSize           int     `toml:"batch_size" json:"batch_size"`
	SharesWorkers       int     `toml:"shares_workers" json:"shares_workers"`
	PriceOverlapDays    int     `toml:"price_overlap_days" json:"price_overlap_days"`
	StaleDays           int     `toml:"stale_days" json:"stale_days"`
	PriceDriftTolerance float64 `toml:"price_drift_tolerance" json:"price_drift_tolerance"`
	RunsKeepDays        int     `toml:"runs_keep_days" json:"runs_keep_days"`
	SharesTTLDays       int     `toml:"shares_ttl_days" json:"shares_ttl_days"`
	NoDataTTLDays       int     `toml:"no_data_ttl_days" json:"no_data_ttl_days"`
}

// HistoryStart es el primer día del histórico a descargar (YYYY-MM-DD): `history_years` años antes de `today`.
func (d Data) HistoryStart(today time.Time) string {
	days := int(math.RoundToEven(d.HistoryYears * 365.25))
	return today.AddDate(0, 0, -days).Format("2006-01-02")
}

// Backtest es [backtest]: período, plazo, cartera y costos.
type Backtest struct {
	StartDate           string    `toml:"start_date" json:"start_date"`
	EndDate             string    `toml:"end_date" json:"end_date"`
	BacktestYears       float64   `toml:"backtest_years" json:"backtest_years"`
	PeriodsYears        []float64 `toml:"periods_years" json:"periods_years"`
	Months              int       `toml:"months" json:"months"`
	InitialCapital      float64   `toml:"initial_capital" json:"initial_capital"`
	TopN                int       `toml:"top_n" json:"top_n"`
	Commission          float64   `toml:"commission" json:"commission"`
	Alternates          int       `toml:"alternates" json:"alternates"`
	TradingDaysPerMonth int       `toml:"trading_days_per_month" json:"trading_days_per_month"`
}

// Tickers es [tickers]: símbolo viejo -> símbolo actual en Yahoo.
type Tickers struct {
	Aliases map[string]string `toml:"aliases" json:"aliases"`
}

// Simulation es [simulation]: plazos que compara --sweep.
type Simulation struct {
	Months []int `toml:"months" json:"months"`
}

// Output es [output]: reportes, logs y JSON del frontend.
type Output struct {
	Dir            string `toml:"dir" json:"dir"`
	KeepDays       int    `toml:"keep_days" json:"keep_days"`
	BacktestSubdir string `toml:"backtest_subdir" json:"backtest_subdir"`
	ActualSubdir   string `toml:"actual_subdir" json:"actual_subdir"`
	BacktestPrefix string `toml:"backtest_prefix" json:"backtest_prefix"`
	SweepPrefix    string `toml:"sweep_prefix" json:"sweep_prefix"`
	OffsetsPrefix  string `toml:"offsets_prefix" json:"offsets_prefix"`
	ActualPrefix   string `toml:"actual_prefix" json:"actual_prefix"`
	EtfSubdir      string `toml:"etf_subdir" json:"etf_subdir"`
	EtfPrefix      string `toml:"etf_prefix" json:"etf_prefix"`
	LogDir         string `toml:"log_dir" json:"log_dir"`
	BacktestLog    string `toml:"backtest_log" json:"backtest_log"`
	ActualLog      string `toml:"actual_log" json:"actual_log"`
	EtfLog         string `toml:"etf_log" json:"etf_log"`
	EtfqLog        string `toml:"etfq_log" json:"etfq_log"`
	QuantLog       string `toml:"quant_log" json:"quant_log"`
	ExportLog      string `toml:"export_log" json:"export_log"`
	PruneLog       string `toml:"prune_log" json:"prune_log"`
	FrontendData   string `toml:"frontend_data" json:"frontend_data"`
	HistoryFile    string `toml:"history_file" json:"history_file"`
}

// Etf es [etf]: ranking de los ETFs de Vanguard.
type Etf struct {
	UpdateList     bool    `toml:"update_list" json:"update_list"`
	ListURL        string  `toml:"list_url" json:"list_url"`
	CSV            string  `toml:"csv" json:"csv"`
	HorizonsMonths []int   `toml:"horizons_months" json:"horizons_months"`
	TopN           int     `toml:"top_n" json:"top_n"`
	MinVol         float64 `toml:"min_vol" json:"min_vol"`
	RiskFreeTicker string  `toml:"risk_free_ticker" json:"risk_free_ticker"`
}

// Family es una gestora de [etfq]: Name es como la identifica Yahoo Finance (fundFamily) y Label el nombre que se muestra.
type Family struct {
	Name  string `toml:"name" json:"name"`
	Label string `toml:"label" json:"label"`
}

// Etfq es [etfq]: ETFs de las grandes gestoras, con su calidad (costos y liquidez). La sección de ETFs de Vanguard ([etf]) es
// independiente; el score de momentum sí usa sus plazos, referencia libre de riesgo y volatilidad mínima.
type Etfq struct {
	Families []Family `toml:"families" json:"families"`
	TopN     int      `toml:"top_n" json:"top_n"`
	// TER máximo en % anual y AUM mínimo en millones de US$ para ser elegible (0 = sin tope / sin mínimo)
	MaxTER         float64 `toml:"max_ter" json:"max_ter"`
	MinAumMillions float64 `toml:"min_aum_millions" json:"min_aum_millions"`
	// Peso de la calidad en el score final (0-1) y pesos de TER y AUM dentro de la calidad
	QualityWeight  float64   `toml:"quality_weight" json:"quality_weight"`
	QualityWeights []float64 `toml:"quality_weights" json:"quality_weights"`
	// Días que se reutiliza la categoría guardada de cada ETF (casi no cambia)
	CategoryTTLDays int `toml:"category_ttl_days" json:"category_ttl_days"`
}

// Quant es [quant]: modelo por score y Markowitz.
type Quant struct {
	Weights             []float64 `toml:"weights" json:"weights"`
	HealthWeights       []float64 `toml:"health_weights" json:"health_weights"`
	TopN                int       `toml:"top_n" json:"top_n"`
	VolTarget           float64   `toml:"vol_target" json:"vol_target"`
	WMax                float64   `toml:"w_max" json:"w_max"`
	SectorMax           float64   `toml:"sector_max" json:"sector_max"`
	MuShrink            float64   `toml:"mu_shrink" json:"mu_shrink"`
	EquityPremium       float64   `toml:"equity_premium" json:"equity_premium"`
	RiskFree            float64   `toml:"risk_free" json:"risk_free"`
	FundamentalsTTLDays int       `toml:"fundamentals_ttl_days" json:"fundamentals_ttl_days"`
}

// Universe es [universe]: filtro por capitalización.
type Universe struct {
	MinCapBillions      float64   `toml:"min_cap_billions" json:"min_cap_billions"`
	SweepMinCapBillions []float64 `toml:"sweep_min_cap_billions" json:"sweep_min_cap_billions"`
}

// Config es toda la configuración: valores por defecto con lo que defina el TOML encima (sección por sección).
type Config struct {
	Data       Data       `toml:"data" json:"data"`
	Backtest   Backtest   `toml:"backtest" json:"backtest"`
	Tickers    Tickers    `toml:"tickers" json:"tickers"`
	Simulation Simulation `toml:"simulation" json:"simulation"`
	Output     Output     `toml:"output" json:"output"`
	Etf        Etf        `toml:"etf" json:"etf"`
	Etfq       Etfq       `toml:"etfq" json:"etfq"`
	Quant      Quant      `toml:"quant" json:"quant"`
	Universe   Universe   `toml:"universe" json:"universe"`
}

// Default son los valores que se usan cuando el TOML no dice otra cosa.
func Default() Config {
	return Config{
		Data: Data{
			HistoryYears:        12,
			DBPath:              "data/top3.db",
			HistoricalURL:       "https://raw.githubusercontent.com/hanshof/sp500_constituents/main/sp_500_historical_components.csv",
			WikipediaURL:        "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies",
			WikipediaChangesURL: "https://en.wikipedia.org/wiki/Historical_components_of_the_S%26P_500",
			UserAgent:           "Mozilla/5.0",
			HTTPTimeout:         30,
			HistoricalTimeout:   60,
			BatchSize:           100,
			SharesWorkers:       8,
			PriceOverlapDays:    7,
			StaleDays:           30,
			PriceDriftTolerance: 0.0001,
			RunsKeepDays:        7,
			SharesTTLDays:       30,
			NoDataTTLDays:       30,
		},
		Backtest: Backtest{
			BacktestYears:       5,
			PeriodsYears:        []float64{3, 5, 10},
			Months:              4,
			InitialCapital:      1000,
			TopN:                4,
			Commission:          0.0005,
			Alternates:          4,
			TradingDaysPerMonth: 21,
		},
		Tickers:    Tickers{Aliases: defaultAliases()},
		Simulation: Simulation{Months: []int{3, 4, 6, 8, 12}},
		Output: Output{
			Dir: "output", KeepDays: 7, BacktestSubdir: "backtest", ActualSubdir: "actual",
			BacktestPrefix: "top3_backtest", SweepPrefix: "top3_sweep", OffsetsPrefix: "top3_offsets",
			ActualPrefix: "top3_actual", EtfSubdir: "etf", EtfPrefix: "top3_etf", LogDir: "log",
			BacktestLog: "top3_backtest.log", ActualLog: "top3_actual.log", EtfLog: "top3_etf.log", EtfqLog: "top3_etfq.log",
			QuantLog: "top3_quant.log", ExportLog: "export_frontend.log", PruneLog: "top3_prune.log",
			FrontendData: "../frontend/public/data/report.json",
			HistoryFile:  "history/history.json",
		},
		Etf: Etf{
			UpdateList:     true,
			ListURL:        "https://investor.vanguard.com/investment-products/list/funddetail/all",
			CSV:            "vanguard_etfs.csv",
			HorizonsMonths: []int{3, 6, 12},
			TopN:           15,
			MinVol:         0.01,
			RiskFreeTicker: "VUSB",
		},
		Etfq: Etfq{
			Families: []Family{
				{"iShares", "iShares"}, {"Vanguard", "Vanguard"}, {"State Street Investment Management", "SPDR"},
				{"Invesco", "Invesco"}, {"Schwab ETFs", "Schwab"},
			},
			TopN: 20, MaxTER: 0.50, MinAumMillions: 100, QualityWeight: 0.25, QualityWeights: []float64{4, 1}, CategoryTTLDays: 30,
		},
		Quant: Quant{
			Weights: []float64{1, 1, 1}, HealthWeights: []float64{1, 1, 1, 2}, TopN: 10, VolTarget: 0.20,
			WMax: 0.25, SectorMax: 0.40, MuShrink: 0.25, EquityPremium: 0.05, RiskFree: 0.04,
			FundamentalsTTLDays: 7,
		},
		Universe: Universe{MinCapBillions: 10, SweepMinCapBillions: []float64{0, 10, 200}},
	}
}

func defaultAliases() map[string]string {
	return map[string]string{"BK": "BNY", "FI": "FISV", "MMC": "MRSH", "EQR": "VMRK", "PSKY": "SKYD", "SATS": "ECHO"}
}

// Load es Default() con lo que defina el TOML de `path` encima. Un archivo que no existe da los valores por defecto
// (existed = false). Una sección o clave desconocida es un error: casi siempre es un typo que de otro modo se ignoraría.
func Load(path string) (cfg Config, existed bool, err error) {
	cfg = Default()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	cfg.Tickers.Aliases = nil // si el TOML define alias, reemplazan a los de fábrica (no se mezclan)
	md, err := toml.Decode(string(raw), &cfg)
	if err != nil {
		return cfg, true, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Tickers.Aliases == nil {
		cfg.Tickers.Aliases = defaultAliases()
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return cfg, true, unknownKeys(undecoded)
	}
	return cfg, true, nil
}

func unknownKeys(keys []toml.Key) error {
	sections := map[string][]string{}
	var order []string
	for _, k := range keys {
		if len(k) == 1 {
			return errs.CfgUnknownKey.Errorf("Sección desconocida en la configuración: [%s]", k[0])
		}
		sec := strings.Join(k[:len(k)-1], ".")
		if _, seen := sections[sec]; !seen {
			order = append(order, sec)
		}
		sections[sec] = append(sections[sec], k[len(k)-1])
	}
	sort.Strings(order)
	names := sections[order[0]]
	sort.Strings(names)
	return errs.CfgUnknownKey.Errorf("Clave desconocida en [%s]: %s", order[0], strings.Join(names, ", "))
}

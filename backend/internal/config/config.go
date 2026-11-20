// Package config es la configuración tipada: una struct por sección de config.toml, con los valores por defecto en
// Default(). Los tags json coinciden con los nombres de las secciones y claves porque la configuración de cada corrida
// del backtest se guarda en SQLite como JSON (tabla runs.params) y el informe la lee de ahí.
//
// Fecha: 2026-11-19
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

	"topwx/internal/errs"
)

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

func (d Data) HistoryStart(today time.Time) string {
	days := int(math.RoundToEven(d.HistoryYears * 365.25))
	return today.AddDate(0, 0, -days).Format("2006-01-02")
}

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

type Tickers struct {
	Aliases map[string]string `toml:"aliases" json:"aliases"`
}

type Simulation struct {
	Months []int `toml:"months" json:"months"`
}

type Output struct {
	LogDir       string `toml:"log_dir" json:"log_dir"`
	BacktestLog  string `toml:"backtest_log" json:"backtest_log"`
	ActualLog    string `toml:"actual_log" json:"actual_log"`
	EtfqLog      string `toml:"etfq_log" json:"etfq_log"`
	QuantLog     string `toml:"quant_log" json:"quant_log"`
	EvaluatorLog string `toml:"evaluator_log" json:"evaluator_log"`
	VisorLog     string `toml:"visor_log" json:"visor_log"`
	ExportLog    string `toml:"export_log" json:"export_log"`
	PruneLog     string `toml:"prune_log" json:"prune_log"`
	ReportJSON   string `toml:"report_json" json:"report_json"`
	HistoryFile  string `toml:"history_file" json:"history_file"`
}

type Family struct {
	Name  string `toml:"name" json:"name"`
	Label string `toml:"label" json:"label"`
}

type Etfq struct {
	HorizonsMonths  []int     `toml:"horizons_months" json:"horizons_months"`
	MinVol          float64   `toml:"min_vol" json:"min_vol"`
	RiskFreeTicker  string    `toml:"risk_free_ticker" json:"risk_free_ticker"`
	Families        []Family  `toml:"families" json:"families"`
	TopN            int       `toml:"top_n" json:"top_n"`
	MaxTER          float64   `toml:"max_ter" json:"max_ter"`
	MinAumMillions  float64   `toml:"min_aum_millions" json:"min_aum_millions"`
	QualityWeight   float64   `toml:"quality_weight" json:"quality_weight"`
	QualityWeights  []float64 `toml:"quality_weights" json:"quality_weights"`
	CategoryTTLDays int       `toml:"category_ttl_days" json:"category_ttl_days"`
}

type Quant struct {
	Weights             []float64 `toml:"weights" json:"weights"`
	HealthWeights       []float64 `toml:"health_weights" json:"health_weights"`
	TopN                int       `toml:"top_n" json:"top_n"`
	VolTarget           float64   `toml:"vol_target" json:"vol_target"`
	WMax                float64   `toml:"w_max" json:"w_max"`
	WMin                float64   `toml:"w_min" json:"w_min"`
	WStep               float64   `toml:"w_step" json:"w_step"`
	SectorMax           float64   `toml:"sector_max" json:"sector_max"`
	MuShrink            float64   `toml:"mu_shrink" json:"mu_shrink"`
	EquityPremium       float64   `toml:"equity_premium" json:"equity_premium"`
	RiskFree            float64   `toml:"risk_free" json:"risk_free"`
	FundamentalsTTLDays int       `toml:"fundamentals_ttl_days" json:"fundamentals_ttl_days"`
}

type Evaluator struct {
	Output      string  `toml:"output" json:"output"`
	Peers       int     `toml:"peers" json:"peers"`
	Sp500       bool    `toml:"sp500" json:"sp500"`
	MinCoverage float64 `toml:"min_coverage" json:"min_coverage"`
}

type Visor struct {
	Output            string   `toml:"output" json:"output"`
	Tickers           []string `toml:"tickers" json:"tickers"`
	CapMinBillions    float64  `toml:"cap_min_billions" json:"cap_min_billions"`
	CapMegaBillions   float64  `toml:"cap_mega_billions" json:"cap_mega_billions"`
	EtfTop            int      `toml:"etf_top" json:"etf_top"`
	EtfAumMinMillions float64  `toml:"etf_aum_min_millions" json:"etf_aum_min_millions"`
	IncludeLeveraged  bool     `toml:"include_leveraged" json:"include_leveraged"`
	Rf                float64  `toml:"rf" json:"rf"`
	MinCoverage       float64  `toml:"min_coverage" json:"min_coverage"`
}

type Universe struct {
	MinCapBillions      float64   `toml:"min_cap_billions" json:"min_cap_billions"`
	SweepMinCapBillions []float64 `toml:"sweep_min_cap_billions" json:"sweep_min_cap_billions"`
}

type Config struct {
	Data       Data       `toml:"data" json:"data"`
	Backtest   Backtest   `toml:"backtest" json:"backtest"`
	Tickers    Tickers    `toml:"tickers" json:"tickers"`
	Simulation Simulation `toml:"simulation" json:"simulation"`
	Output     Output     `toml:"output" json:"output"`
	Etfq       Etfq       `toml:"etfq" json:"etfq"`
	Quant      Quant      `toml:"quant" json:"quant"`
	Evaluator  Evaluator  `toml:"evaluator" json:"evaluator"`
	Visor      Visor      `toml:"visor" json:"visor"`
	Universe   Universe   `toml:"universe" json:"universe"`
}

func Default() Config {
	return Config{
		Data: Data{
			HistoryYears:        12,
			DBPath:              "data/topwx.db",
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
			LogDir:      "log",
			BacktestLog: "topwx_backtest.log", ActualLog: "topwx_actual.log", EtfqLog: "topwx_etfq.log",
			QuantLog: "topwx_quant.log", EvaluatorLog: "topwx_evaluator.log", VisorLog: "topwx_visor.log", ExportLog: "topwx_export.log", PruneLog: "topwx_prune.log",
			ReportJSON:  "output/report.json",
			HistoryFile: "history/history.json",
		},
		Etfq: Etfq{
			HorizonsMonths: []int{3, 6, 12}, MinVol: 0.01, RiskFreeTicker: "VUSB",
			Families: []Family{
				{"iShares", "iShares"}, {"Vanguard", "Vanguard"}, {"State Street Investment Management", "SPDR"},
				{"Invesco", "Invesco"}, {"Schwab ETFs", "Schwab"},
			},
			TopN: 20, MaxTER: 0.50, MinAumMillions: 100, QualityWeight: 0.25, QualityWeights: []float64{4, 1}, CategoryTTLDays: 30,
		},
		Quant: Quant{
			Weights: []float64{1, 1, 1}, HealthWeights: []float64{1, 1, 1, 2}, TopN: 10, VolTarget: 0.20,
			WMax: 0.25, WMin: 0.10, WStep: 0.01, SectorMax: 0.40, MuShrink: 0.25, EquityPremium: 0.05, RiskFree: 0.04,
			FundamentalsTTLDays: 7,
		},
		Evaluator: Evaluator{Output: "output/evaluator.json", Peers: 3, Sp500: true, MinCoverage: 0.5},
		Visor: Visor{
			Output: "output/visor.json", Tickers: []string{"SPY", "QQQ"}, CapMinBillions: 100, CapMegaBillions: 200,
			EtfTop: 300, EtfAumMinMillions: 500, Rf: 3, MinCoverage: 0.5,
		},
		Universe: Universe{MinCapBillions: 10, SweepMinCapBillions: []float64{0, 10, 200}},
	}
}

func defaultAliases() map[string]string {
	return map[string]string{"BK": "BNY", "FI": "FISV", "MMC": "MRSH", "EQR": "VMRK", "PSKY": "SKYD", "SATS": "ECHO"}
}

func Load(path string) (cfg Config, existed bool, err error) {
	cfg = Default()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	cfg.Tickers.Aliases = nil
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

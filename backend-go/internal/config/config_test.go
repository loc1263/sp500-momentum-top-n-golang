package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	cfg, existed, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil || existed {
		t.Fatalf("existed=%v err=%v", existed, err)
	}
	if cfg.Backtest.TopN != 4 || cfg.Tickers.Aliases["BK"] != "BNY" {
		t.Fatalf("defaults mal: %+v", cfg.Backtest)
	}
}

func TestLoadOverridesSectionByKey(t *testing.T) {
	cfg, _, err := Load(write(t, "[backtest]\ntop_n = 7\nperiods_years = [2, 4]\n[tickers.aliases]\nAAA = \"BBB\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backtest.TopN != 7 || cfg.Backtest.Months != 4 {
		t.Fatalf("top_n/months: %+v", cfg.Backtest)
	}
	if len(cfg.Backtest.PeriodsYears) != 2 || cfg.Backtest.PeriodsYears[1] != 4 {
		t.Fatalf("periods: %v", cfg.Backtest.PeriodsYears)
	}
	if len(cfg.Tickers.Aliases) != 1 || cfg.Tickers.Aliases["AAA"] != "BBB" {
		t.Fatalf("los alias del TOML deben reemplazar a los de fábrica: %v", cfg.Tickers.Aliases)
	}
}

func TestLoadRejectsUnknownKeysAndSections(t *testing.T) {
	_, _, err := Load(write(t, "[backtest]\ntopn = 3\n"))
	if err == nil || !strings.Contains(err.Error(), "TOP-CFG-004") || !strings.Contains(err.Error(), "[backtest]") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = Load(write(t, "[nueva]\nx = 1\n"))
	if err == nil || !strings.Contains(err.Error(), "[nueva]") {
		t.Fatalf("err = %v", err)
	}
}

func TestShippedConfigLoads(t *testing.T) {
	if _, _, err := Load("../../config.toml"); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryStart(t *testing.T) {
	d := Data{HistoryYears: 12}
	got := d.HistoryStart(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC))
	if got != "2014-10-24" && got != "2014-10-25" {
		t.Fatalf("got %s", got)
	}
}

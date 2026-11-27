// Pruebas del caché de datos fundamentales crudos.
//
// Fecha: 2026-11-27
package company

import (
	"path/filepath"

	"encoding/json"
	"testing"
	"topwx/internal/db"
	"topwx/internal/repo"

	"github.com/loc1263/gohoo"
)

func TestSnapshotSurvivesTheJSONCache(t *testing.T) {
	ts := map[string]any{"timeseries": map[string]any{"result": []any{map[string]any{
		"annualFreeCashFlow": []any{map[string]any{"asOfDate": "2025-12-31", "reportedValue": map[string]any{"raw": 5.0}}},
	}}}}
	js, err := json.Marshal(stored{V: version, Info: map[string]any{"sector": "Tech"}, TS: ts})
	if err != nil {
		t.Fatal(err)
	}
	s, ok := parse(string(js))
	if !ok || s.Info["sector"] != "Tech" {
		t.Fatalf("parse: %v %v", s, ok)
	}
	if pts := gohoo.TimeseriesPoints(s.TS, "annualFreeCashFlow"); len(pts) != 1 || pts[0].Value != 5 {
		t.Errorf("series tras el caché: %v", pts)
	}
}

func TestParseRejectsOldCacheFormat(t *testing.T) {
	// El formato anterior de la tabla fundamentals guardaba solo los campos del score de salud, con v=2.
	if _, ok := parse(`{"v":2,"sector":"Tech","fcf_history":[]}`); ok {
		t.Error("el formato viejo no debe leerse como snapshot")
	}
}

func TestSectorsReadsOnlyTheCurrentFormat(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	cur, _ := json.Marshal(stored{V: version, Info: map[string]any{"sector": "Tech"}})
	if err := repo.SaveFundamentals(d, map[string]string{"NEW": string(cur), "OLD": `{"v":2,"sector":"Energy"}`}, db.Today()); err != nil {
		t.Fatal(err)
	}
	got := Sectors(d, 7)
	if len(got) != 1 || got["NEW"] != "Tech" {
		t.Errorf("Sectors: %v", got)
	}
}

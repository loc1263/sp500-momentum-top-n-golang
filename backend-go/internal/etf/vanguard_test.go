package etf

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"top3/internal/actual"
)

func changes(entered, exited []string) actual.Changes {
	return actual.Changes{Entered: entered, Exited: exited, Kept: []string{}}
}

func profile(ticker, longName string, isETF bool, style, kind string, closed bool) map[string]any {
	return map[string]any{"profile": map[string]any{"ticker": ticker, "longName": longName, "isETF": isETF, "style": style, "type": kind,
		"fundFact": map[string]any{"isClosed": closed}}}
}

func response(extra ...map[string]any) map[string]any {
	var entities []any
	for i := 0; i < 25; i++ {
		entities = append(entities, profile(fmt.Sprintf("E%02d", i), fmt.Sprintf("Vanguard Fund %d ETF", i), true, "Stock Funds", "Domestic Stock - General", false))
	}
	for _, e := range extra {
		entities = append(entities, e)
	}
	return map[string]any{"fund": map[string]any{"entity": entities}}
}

func TestParseKeepsOnlyOpenEtfsAndCleansNames(t *testing.T) {
	data := response(
		profile("VTI", "Vanguard Total Stock Market ETF Shares", true, "Stock Funds", "Domestic Stock - General", false),
		profile("VTSAX", "Vanguard Total Stock Market Index Fund Admiral Shares", false, "Stock Funds", "x", false),
		profile("OLD", "Vanguard Old ETF", true, "Stock Funds", "x", true))
	etfs, err := ParseEtfs(data)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Fund{}
	for _, e := range etfs {
		by[e.Ticker] = e
	}
	if _, ok := by["VTSAX"]; ok {
		t.Fatal("un fondo que no es ETF no entra")
	}
	if _, ok := by["OLD"]; ok {
		t.Fatal("un ETF cerrado no entra")
	}
	if by["VTI"] != (Fund{"VTI", "Total Stock Market", "Acciones EE. UU. amplio"}) {
		t.Fatalf("VTI: %+v", by["VTI"])
	}
}

func TestCategoryFromTheStyleVanguardReports(t *testing.T) {
	for _, c := range []struct{ name, style, kind, want string }{
		{"Vanguard Core Bond ETF", "Bond Funds", "Intermediate-Term Bond", "Bonos EE. UU."},
		{"Vanguard Total International Bond ETF", "Bond Funds", "Taxable Bond", "Bonos internacionales"},
		{"Vanguard Emerging Markets ex-China ETF", "Stock Funds", "International/Global Stock", "Acciones internacionales"},
		{"Vanguard Energy ETF", "Stock Funds", "Domestic Stock - Sector-Specific", "Sectores"},
		{"Vanguard Mid-Cap ETF", "Stock Funds", "Domestic Stock - More Aggressive", "Acciones EE. UU. amplio"},
	} {
		got := Category(map[string]any{"longName": c.name, "style": c.style, "type": c.kind})
		if got != c.want {
			t.Errorf("%s: %q, quiero %q", c.name, got, c.want)
		}
	}
}

func TestAResponseWithTheWrongShapeOrTooFewEtfsIsRejected(t *testing.T) {
	if _, err := ParseEtfs(map[string]any{"algo": []any{}}); err == nil || !strings.Contains(err.Error(), "forma esperada") {
		t.Fatalf("err = %v", err)
	}
	few := map[string]any{"fund": map[string]any{"entity": []any{
		profile("A", "Vanguard A ETF", true, "", "", false), profile("B", "Vanguard B ETF", true, "", "", false)}}}
	if _, err := ParseEtfs(few); err == nil || !strings.Contains(err.Error(), "solo 2 ETFs") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateListAppendsOnlyNewTickersAndNeverEditsExistingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "e.csv")
	os.WriteFile(path, []byte("ticker,name,category\r\nVTI,Mi nombre,Mi categoría\r\nGONE,Cerrado,Cat\r\n"), 0o644)
	etfs := []Fund{{"VTI", "Total Stock Market", "Otra"}, {"ZZZ", "Zeta, con coma", "Sectores"}, {"AAA", "Alfa", "Sectores"}}
	added, gone, err := UpdateList(path, etfs)
	if err != nil || !reflect.DeepEqual(added, []string{"AAA", "ZZZ"}) || !reflect.DeepEqual(gone, []string{"GONE"}) {
		t.Fatalf("added=%v gone=%v err=%v", added, gone, err)
	}
	raw, _ := os.ReadFile(path)
	text := string(raw)
	if !strings.HasPrefix(text, "ticker,name,category\r\nVTI,Mi nombre,Mi categoría\r\nGONE,Cerrado,Cat\r\n") ||
		!strings.HasSuffix(text, "AAA,Alfa,Sectores\r\nZZZ,\"Zeta, con coma\",Sectores\r\n") {
		t.Fatalf("archivo: %q", text)
	}
	added, gone, _ = UpdateList(path, etfs) // idempotente
	again, _ := os.ReadFile(path)
	if len(added) != 0 || !reflect.DeepEqual(gone, []string{"GONE"}) || string(again) != text {
		t.Fatalf("segunda pasada: added=%v gone=%v", added, gone)
	}
}

func TestUpdateListAddsTheMissingFinalNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "e.csv")
	os.WriteFile(path, []byte("ticker,name,category\nVTI,Total,Cat"), 0o644)
	if _, _, err := UpdateList(path, []Fund{{"VTI", "x", "y"}, {"NEW", "Nuevo", "Sectores"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "ticker,name,category\nVTI,Total,Cat\nNEW,Nuevo,Sectores\n" {
		t.Fatalf("archivo: %q", raw)
	}
}

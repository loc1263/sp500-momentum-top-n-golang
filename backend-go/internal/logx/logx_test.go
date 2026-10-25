package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"top3/internal/errs"
)

func setup(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	if err := Setup("t.log", dir); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	SetConsole(&console, Info)
	t.Cleanup(func() { Close(); SetConsole(os.Stderr, Info) })
	return &console, filepath.Join(dir, "t.log")
}

func TestDebugGoesOnlyToFile(t *testing.T) {
	console, path := setup(t)
	Debugf("solo archivo %d", 1)
	Infof("a todos lados")
	Close()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "[DEBUG] solo archivo 1") || !strings.Contains(string(data), "[INFO] a todos lados") {
		t.Fatalf("archivo: %q", data)
	}
	if strings.Contains(console.String(), "solo archivo") || !strings.Contains(console.String(), "a todos lados") {
		t.Fatalf("consola: %q", console.String())
	}
}

func TestSummaryGroupsByCodeAndKeepsHighestLevel(t *testing.T) {
	setup(t)
	Warnf(errs.BktFewTickers.Msg("pocos en %s"), "2020-01-01")
	Warnf(errs.BktFewTickers.Msg("pocos en %s"), "2020-05-01")
	Errorf("%s", errs.NetRateLimit.Msg("limitado"))
	Warnf("aviso suelto")
	items := summaryItems()
	if len(items) != 3 {
		t.Fatalf("items: %+v", items)
	}
	byCode := map[string]SummaryItem{}
	for _, i := range items {
		byCode[i.Code] = i
	}
	if byCode["TOP-BKT-003"].Count != 2 || byCode["TOP-NET-005"].Level != Error || byCode["sin código"].Count != 1 {
		t.Fatalf("items: %+v", byCode)
	}
	text := strings.Join(FormatSummary(items, "log/x.log"), "\n")
	if !strings.Contains(text, "1 errores, 3 avisos") || strings.Index(text, "TOP-NET-005") > strings.Index(text, "TOP-BKT-003") {
		t.Fatalf("los errores van primero:\n%s", text)
	}
}

func TestMergeSummariesJoinsStepsPerCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	data := `{"code":"TOP-A-1","count":2,"level":30,"sample":"x","step":"uno"}` + "\n" +
		`{"code":"TOP-A-1","count":1,"level":40,"sample":"x","step":"dos"}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got := MergeSummaries(path)
	if len(got) != 1 || got[0].Count != 3 || got[0].Level != Error || strings.Join(got[0].Steps, ",") != "uno,dos" {
		t.Fatalf("got %+v", got)
	}
}

func TestRunCLIReturnsExitCode(t *testing.T) {
	setup(t)
	if code := RunCLI(func() error { return nil }); code != 0 {
		t.Fatalf("code %d", code)
	}
	if code := RunCLI(func() error { return errs.BktEmpty.Errorf("vacío") }); code != 1 {
		t.Fatalf("code %d", code)
	}
}

// El archivo de log va con búfer: los avisos se ven en disco al instante y el resto al cerrar.
func TestLogFileIsBufferedButWarningsFlushAtOnce(t *testing.T) {
	dir := t.TempDir()
	if err := Setup("t.log", dir); err != nil {
		t.Fatal(err)
	}
	defer Close()
	read := func() string {
		b, _ := os.ReadFile(filepath.Join(dir, "t.log"))
		return string(b)
	}
	Debugf("detalle %d", 1)
	if got := read(); got != "" {
		t.Fatalf("el DEBUG no debería haber llegado al disco todavía: %q", got)
	}
	Warnf("TOP-BKT-001: algo")
	if got := read(); !strings.Contains(got, "detalle 1") || !strings.Contains(got, "TOP-BKT-001") {
		t.Fatalf("un aviso vuelca todo lo anterior: %q", got)
	}
	Debugf("final")
	Close()
	if got := read(); !strings.HasSuffix(strings.TrimSpace(got), "final") {
		t.Fatalf("al cerrar se vuelca el resto: %q", got)
	}
}

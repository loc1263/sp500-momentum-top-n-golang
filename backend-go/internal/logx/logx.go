// Package logx es el logging del proyecto: consola a nivel INFO (avisos en amarillo, errores en rojo), archivo a nivel
// DEBUG que se sobrescribe en cada corrida, y un resumen de avisos y errores agrupados por código.
//
// Fecha: 2026-11-19
package logx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"top3/internal/errs"
	"top3/internal/termfmt"
)

const (
	Debug   = 10
	Info    = 20
	Warning = 30
	Error   = 40
)

var levelNames = map[int]string{Debug: "DEBUG", Info: "INFO", Warning: "WARNING", Error: "ERROR"}

const (
	SummaryFileEnv = "TOP3_SUMMARY_FILE"
	StepEnv        = "TOP3_STEP"
)

var (
	mu          sync.Mutex
	colorMu     sync.Mutex
	file        io.WriteCloser
	logfile     string
	consoleOut  io.Writer = os.Stderr
	consoleMin            = Info
	summary               = map[string]*SummaryItem{}
	colorCache            = map[*os.File]bool{}
	codePattern           = regexp.MustCompile(`^(TOP-[A-Z]{3}-\d{3}): `)
	codeToken             = regexp.MustCompile(`TOP-[A-Z]{3}-\d{3}`)
	prefix                = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3}) \[(\w+)\]`)
)

type SummaryItem struct {
	Code   string   `json:"code"`
	Count  int      `json:"count"`
	Level  int      `json:"level"`
	Sample string   `json:"sample"`
	Step   string   `json:"step,omitempty"`
	Steps  []string `json:"steps,omitempty"`
}

func ColorEnabled(stream *os.File) bool {
	colorMu.Lock()
	defer colorMu.Unlock()
	if v, ok := colorCache[stream]; ok {
		return v
	}
	on := isTerminal(stream) && os.Getenv("NO_COLOR") == ""
	if on {
		enableVT(stream)
	}
	colorCache[stream] = on
	return on
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func Heading(text string, stream *os.File) string {
	if stream == nil {
		stream = os.Stderr
	}
	if ColorEnabled(stream) {
		return termfmt.Green + text + termfmt.Reset
	}
	return text
}

func Paint(text string, level int) string {
	if !ColorEnabled(os.Stderr) || level < Warning {
		return text
	}
	if level >= Error {
		return termfmt.Red + text + termfmt.Reset
	}
	return termfmt.Yellow + text + termfmt.Reset
}

func Setup(name, dir string) error {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
	summary = map[string]*SummaryItem{}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	logfile = filepath.Join(dir, name)
	f, err := os.Create(logfile)
	if err != nil {
		return err
	}
	file = &bufferedFile{bw: bufio.NewWriterSize(f, 64<<10), f: f}
	return nil
}

type bufferedFile struct {
	bw *bufio.Writer
	f  *os.File
}

func (b *bufferedFile) Write(p []byte) (int, error) { return b.bw.Write(p) }
func (b *bufferedFile) Flush() error                { return b.bw.Flush() }
func (b *bufferedFile) Close() error {
	err := b.bw.Flush()
	if cerr := b.f.Close(); err == nil {
		err = cerr
	}
	return err
}

type flusher interface{ Flush() error }

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
}

func SetConsole(w io.Writer, min int) {
	mu.Lock()
	defer mu.Unlock()
	if w == nil {
		w = os.Stderr
	}
	consoleOut, consoleMin = w, min
}

func stamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05") + fmt.Sprintf(",%03d", t.Nanosecond()/1e6)
}

func logf(level int, format string, args ...any) {
	if level < Warning {
		mu.Lock()
		quiet := file == nil && level < consoleMin
		mu.Unlock()
		if quiet {
			return
		}
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	line := fmt.Sprintf("%s [%s] %s", stamp(time.Now()), levelNames[level], msg)

	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		fmt.Fprintln(file, line)
		if f, ok := file.(flusher); ok && level >= Warning {
			f.Flush()
		}
	}
	if level >= consoleMin {
		fmt.Fprintln(consoleOut, formatConsole(line, level))
	}
	if level >= Warning {
		code := "sin código"
		if m := codePattern.FindStringSubmatch(msg); m != nil {
			code = m[1]
		}
		item, ok := summary[code]
		if !ok {
			item = &SummaryItem{Code: code, Level: Warning, Sample: msg}
			summary[code] = item
		}
		item.Count++
		item.Level = max(item.Level, level)
	}
}

func formatConsole(line string, level int) string {
	lines := termfmt.WrapDefault(line)
	if consoleOut != io.Writer(os.Stderr) || !ColorEnabled(os.Stderr) {
		return strings.Join(lines, "\n")
	}
	if level >= Warning {
		for i := range lines {
			lines[i] = Paint(lines[i], level)
		}
		return strings.Join(lines, "\n")
	}
	lines[0] = prefix.ReplaceAllString(lines[0],
		termfmt.Color("$1", termfmt.Dim)+" ["+termfmt.Color("$2", termfmt.Cyan)+"]")
	for i := range lines {
		lines[i] = codeToken.ReplaceAllStringFunc(lines[i], func(s string) string { return termfmt.Color(s, termfmt.Bold) })
	}
	return strings.Join(lines, "\n")
}

func Debugf(format string, args ...any) { logf(Debug, format, args...) }
func Infof(format string, args ...any)  { logf(Info, format, args...) }
func Warnf(format string, args ...any)  { logf(Warning, format, args...) }
func Errorf(format string, args ...any) { logf(Error, format, args...) }

func FormatSummary(items []SummaryItem, logPath string) []string {
	var nErr, nWarn int
	for _, i := range items {
		if i.Level >= Error {
			nErr += i.Count
		} else {
			nWarn += i.Count
		}
	}
	lines := []string{Heading(strings.Repeat("=", 78), nil),
		Heading(fmt.Sprintf("RESUMEN DE AVISOS Y ERRORES: %d errores, %d avisos", nErr, nWarn), nil),
		Heading(strings.Repeat("=", 78), nil)}
	ordered := append([]SummaryItem(nil), items...)
	sort.SliceStable(ordered, func(a, b int) bool {
		if ordered[a].Level != ordered[b].Level {
			return ordered[a].Level > ordered[b].Level
		}
		return ordered[a].Code < ordered[b].Code
	})
	for _, item := range ordered {
		entry, known := errs.Catalog[item.Code]
		kind := "AVISO"
		if item.Level >= Error {
			kind = "ERROR"
		}
		title := item.Sample
		if known {
			title = entry.Title
		}
		steps := ""
		if len(item.Steps) > 0 {
			steps = "  [" + strings.Join(item.Steps, ", ") + "]"
		}
		eight := strings.Repeat(" ", 8)
		for _, l := range termfmt.Wrap(fmt.Sprintf("  %-5s %s  x%d  %s%s", kind, item.Code, item.Count, title, steps), termfmt.MaxWidth, &eight) {
			lines = append(lines, Paint(l, item.Level))
		}
		if known {
			sixteen := strings.Repeat(" ", 16)
			sample := item.Sample
			if len(sample) > len(item.Code)+2 {
				sample = sample[len(item.Code)+2:]
			}
			lines = append(lines, termfmt.Wrap("        ej.: "+sample, termfmt.MaxWidth, &sixteen)...)
			lines = append(lines, termfmt.Wrap("        qué hacer: "+entry.Action, termfmt.MaxWidth, &eight)...)
		}
	}
	if logPath == "" {
		logPath = "log/"
	}
	four := strings.Repeat(" ", 4)
	lines = append(lines, termfmt.Wrap(fmt.Sprintf("  Detalle: %s  (buscar por código, p. ej. grep %s)", logPath, ordered[0].Code), termfmt.MaxWidth, &four)...)
	return lines
}

func summaryItems() []SummaryItem {
	mu.Lock()
	defer mu.Unlock()
	out := make([]SummaryItem, 0, len(summary))
	for _, i := range summary {
		out = append(out, *i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Code < out[b].Code })
	return out
}

func PrintSummary() {
	items := summaryItems()
	if len(items) == 0 {
		return
	}
	if path := os.Getenv(SummaryFileEnv); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			defer f.Close()
			step := os.Getenv(StepEnv)
			for _, i := range items {
				i.Step = step
				b, _ := json.Marshal(i)
				fmt.Fprintln(f, string(b))
			}
		}
		return
	}
	fmt.Fprintln(os.Stderr, "\n"+strings.Join(FormatSummary(items, logfile), "\n"))
}

func MergeSummaries(path string) []SummaryItem {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	merged := map[string]*SummaryItem{}
	var order []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var item SummaryItem
		if json.Unmarshal([]byte(line), &item) != nil {
			continue
		}
		out, ok := merged[item.Code]
		if !ok {
			c := item
			c.Count, c.Steps = 0, nil
			out = &c
			merged[item.Code] = out
			order = append(order, item.Code)
		}
		out.Count += item.Count
		out.Level = max(out.Level, item.Level)
		if item.Step != "" && !slices.Contains(out.Steps, item.Step) {
			out.Steps = append(out.Steps, item.Step)
		}
	}
	res := make([]SummaryItem, 0, len(order))
	for _, code := range order {
		m := *merged[code]
		m.Step = ""
		res = append(res, m)
	}
	return res
}

func RunCLI(main func() error) (code int) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("%v", r)
			if file != nil {
				Errorf("panic: %s", msg)
			} else {
				fmt.Fprintln(os.Stderr, Paint("panic: "+msg, Error))
			}
			code = 1
		}
		PrintSummary()
		Close()
	}()
	if err := main(); err != nil {
		if file != nil {
			Errorf("%s", err.Error())
		} else {
			fmt.Fprintln(os.Stderr, Paint(err.Error(), Error))
		}
		return 1
	}
	return 0
}

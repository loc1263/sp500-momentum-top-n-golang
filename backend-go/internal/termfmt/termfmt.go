// Package termfmt es el formato de terminal: ancho máximo de línea y paleta de colores ANSI.
package termfmt

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxWidth es el ancho máximo de una línea de la consola: más ancho se desborda y se corta feo.
const MaxWidth = 130

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// VisibleLen es el largo de text en caracteres, sin contar las secuencias de color.
func VisibleLen(text string) int { return utf8.RuneCountInString(ansi.ReplaceAllString(text, "")) }

// Color envuelve text en los códigos dados.
func Color(text string, codes ...string) string { return strings.Join(codes, "") + text + Reset }

// Wrap parte text en líneas de hasta width caracteres, cortando en espacios. Las líneas siguientes a la primera se
// sangran con indent (si es nil, la sangría de la línea original + 4). Un texto que ya trae colores no se parte: el color
// abierto en una línea quedaría sin cerrar en la otra (quien llama parte primero y pinta después).
func Wrap(text string, width int, indent *string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if VisibleLen(line) <= width || ansi.MatchString(line) {
			out = append(out, line)
			continue
		}
		base := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		sub := base + "    "
		if indent != nil {
			sub = *indent
		}
		out = append(out, wrapLine(line, width, sub)...)
	}
	return out
}

// WrapDefault es Wrap con el ancho máximo y la sangría por defecto.
func WrapDefault(text string) []string { return Wrap(text, MaxWidth, nil) }

func wrapLine(line string, width int, subsequent string) []string {
	words := strings.Fields(line)
	lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	var lines []string
	cur := lead
	curLen := utf8.RuneCountInString(lead)
	empty := true // la línea en curso todavía no tiene palabras
	flush := func() {
		lines = append(lines, cur)
		cur, curLen, empty = subsequent, utf8.RuneCountInString(subsequent), true
	}
	for _, w := range words {
		for {
			wl := utf8.RuneCountInString(w)
			sep := 1
			if empty {
				sep = 0
			}
			if curLen+sep+wl <= width {
				if !empty {
					cur += " "
				}
				cur += w
				curLen += sep + wl
				empty = false
				break
			}
			if !empty {
				flush()
				continue
			}
			// la palabra sola no entra: se corta
			room := max(width-curLen, 1)
			runes := []rune(w)
			cur += string(runes[:min(room, len(runes))])
			w = string(runes[min(room, len(runes)):])
			empty = false
			flush()
			if w == "" {
				break
			}
		}
	}
	if !empty || len(lines) == 0 {
		lines = append(lines, cur)
	}
	return lines
}

// Truncate recorta text a width caracteres con «…» al final si no entraba.
func Truncate(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	return string(runes[:max(width-1, 0)]) + "…"
}

// FitWidths reduce los anchos de columna (de la más ancha a la más angosta) hasta que la tabla entre en limit.
func FitWidths(widths []int, separator, limit, minimum int) []int {
	w := append([]int(nil), widths...)
	total := func() int {
		s := separator * (len(w) - 1)
		for _, x := range w {
			s += x
		}
		return s
	}
	for total() > limit {
		widest := 0
		for i := range w {
			if w[i] > w[widest] {
				widest = i
			}
		}
		if w[widest] <= minimum {
			break
		}
		w[widest]--
	}
	return w
}

// PadLeft y PadRight rellenan con espacios hasta width caracteres (no bytes).
func PadLeft(s string, width int) string {
	if n := width - utf8.RuneCountInString(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

func PadRight(s string, width int) string {
	if n := width - utf8.RuneCountInString(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

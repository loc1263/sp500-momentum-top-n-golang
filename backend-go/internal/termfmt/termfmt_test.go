package termfmt

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapKeepsLinesWithinWidthAndIndentsContinuations(t *testing.T) {
	text := "  " + strings.Repeat("palabra ", 30)
	lines := Wrap(strings.TrimRight(text, " "), 40, nil)
	if len(lines) < 2 {
		t.Fatalf("se esperaban varias líneas: %q", lines)
	}
	for i, l := range lines {
		if utf8.RuneCountInString(l) > 40 {
			t.Errorf("línea %d de %d caracteres: %q", i, len(l), l)
		}
		if i > 0 && !strings.HasPrefix(l, "      ") { // sangría original (2) + 4
			t.Errorf("la línea %d no está sangrada: %q", i, l)
		}
	}
}

func TestWrapLeavesShortAndColoredLines(t *testing.T) {
	if got := Wrap("corta", 40, nil); len(got) != 1 || got[0] != "corta" {
		t.Fatalf("got %q", got)
	}
	colored := Color(strings.Repeat("x ", 50), Red)
	if got := Wrap(colored, 40, nil); len(got) != 1 {
		t.Fatalf("una línea con color no se parte: %q", got)
	}
}

func TestWrapBreaksLongWords(t *testing.T) {
	lines := Wrap(strings.Repeat("a", 95), 40, nil)
	for _, l := range lines {
		if utf8.RuneCountInString(l) > 40 {
			t.Fatalf("línea demasiado larga: %q", l)
		}
	}
	if strings.ReplaceAll(strings.Join(lines, ""), " ", "") != strings.Repeat("a", 95) {
		t.Fatalf("se perdieron caracteres: %q", lines)
	}
}

func TestTruncateAndFit(t *testing.T) {
	if got := Truncate("abcdef", 4); got != "abc…" {
		t.Fatalf("got %q", got)
	}
	if got := Truncate("abc", 4); got != "abc" {
		t.Fatalf("got %q", got)
	}
	w := FitWidths([]int{50, 80, 30}, 3, 100, 8)
	total := w[0] + w[1] + w[2] + 6
	if total > 100 {
		t.Fatalf("no entra: %v", w)
	}
	if w[2] != 30 {
		t.Fatalf("la columna angosta no se toca: %v", w)
	}
}

func TestVisibleLenIgnoresColors(t *testing.T) {
	if VisibleLen(Color("hola ≥", Green, Bold)) != 6 {
		t.Fatal("VisibleLen cuenta mal")
	}
}

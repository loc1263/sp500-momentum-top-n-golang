package market

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Row es una fila de una tabla HTML ya expandida (colspan y rowspan repetidos en cada celda que cubren).
type Row struct {
	Cells  []string
	Header bool // todas sus celdas son <th>
}

// Table es una tabla HTML como grilla de texto.
type Table struct{ Rows []Row }

var (
	footnote = regexp.MustCompile(`\[[^\]]*\]`)
	spaces   = regexp.MustCompile(`\s+`)
)

// ParseTables lee todas las tablas de una página HTML, en orden de aparición (las anidadas se ignoran).
func ParseTables(r io.Reader) ([]Table, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var tables []Table
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables = append(tables, parseTable(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables, nil
}

func parseTable(table *html.Node) Table {
	var trs []*html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type != html.ElementNode:
			case c.Data == "tr":
				trs = append(trs, c)
			case c.Data == "thead" || c.Data == "tbody" || c.Data == "tfoot":
				find(c)
			}
		}
	}
	find(table)

	type span struct {
		left int
		text string
	}
	pending := map[int]*span{} // columna -> celda con rowspan que sigue cubriéndola
	var out Table
	for _, tr := range trs {
		var cells []string
		headers, total := 0, 0
		col := 0
		fillPending := func() {
			for {
				p, ok := pending[col]
				if !ok || p.left == 0 {
					return
				}
				cells = append(cells, p.text)
				p.left--
				col++
			}
		}
		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
				continue
			}
			fillPending()
			total++
			if c.Data == "th" {
				headers++
			}
			text := cellText(c)
			colspan, rowspan := attrInt(c, "colspan"), attrInt(c, "rowspan")
			for k := 0; k < colspan; k++ {
				cells = append(cells, text)
				if rowspan > 1 {
					pending[col] = &span{rowspan - 1, text}
				}
				col++
			}
		}
		fillPending()
		if total == 0 && len(cells) == 0 {
			continue
		}
		out.Rows = append(out.Rows, Row{Cells: cells, Header: total > 0 && headers == total})
	}
	return out
}

func attrInt(n *html.Node, name string) int {
	for _, a := range n.Attr {
		if a.Key == name {
			if v, err := strconv.Atoi(strings.TrimSpace(a.Val)); err == nil && v > 0 {
				return v
			}
		}
	}
	return 1
}

func cellText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "style" || n.Data == "script") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode && n.Data == "br" {
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(spaces.ReplaceAllString(footnote.ReplaceAllString(b.String(), ""), " "))
}

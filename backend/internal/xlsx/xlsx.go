// Package xlsx escribe una hoja de Excel (.xlsx) con formato de número y color de fondo por celda, usando solo la biblioteca estándar.
// Cubre lo que necesita el sitio: textos y números, encabezado fijo con filtros y colores verde, amarillo y rojo.
//
// Fecha: 2026-11-20
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Tone es el color de una celda: el de un nivel (Good, Warn, Bad) o el de un encabezado (Head, HeadGreen).
type Tone int

const (
	None Tone = iota
	Good
	Warn
	Bad
	Head
	HeadGreen
)

// Cell es una celda: Value es string, float64 o nil (vacía); Format es un código de número de Excel («0.0"%"»), vacío para el general.
type Cell struct {
	Value  any
	Format string
	Tone   Tone
}

// Sheet es la hoja completa. La primera fila es el encabezado: queda fija y con filtros. FreezeCols deja fijas las primeras columnas.
type Sheet struct {
	Name       string
	Widths     []float64
	Rows       [][]Cell
	FreezeCols int
}

// Colores de fondo y de letra de cada tono (los mismos que usa Excel en «Bueno», «Neutro» y «Incorrecto»).
var tones = map[Tone]struct{ fill, font string }{
	Good: {"FFC6EFCE", "FF006100"},
	Warn: {"FFFFEB9C", "FF9C5700"},
	Bad:  {"FFFFC7CE", "FF9C0006"},
	Head: {"FFD9E1F2", "FF000000"},
	// Verde claro de Excel (Verde, Énfasis 6, claro 40 %): se distingue del verde de las celdas «Bien».
	HeadGreen: {"FFC6E0B4", "FF000000"},
}

var tonePos = map[Tone]int{None: 0, Good: 1, Warn: 2, Bad: 3, Head: 4, HeadGreen: 5}

type styleKey struct {
	format string
	tone   Tone
}

// styles reparte un índice de estilo (cellXfs) a cada combinación de formato de número y tono.
type styles struct {
	keys    []styleKey
	index   map[styleKey]int
	formats map[string]int
	order   []string
}

func newStyles() *styles {
	s := &styles{index: map[styleKey]int{}, formats: map[string]int{}}
	s.of("", None) // el estilo 0 es el predeterminado
	return s
}

func (s *styles) of(format string, tone Tone) int {
	k := styleKey{format, tone}
	if i, ok := s.index[k]; ok {
		return i
	}
	if _, ok := s.formats[format]; !ok && format != "" {
		s.formats[format] = 164 + len(s.order)
		s.order = append(s.order, format)
	}
	s.index[k] = len(s.keys)
	s.keys = append(s.keys, k)
	return s.index[k]
}

func (s *styles) xml() []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	if len(s.order) > 0 {
		fmt.Fprintf(&b, `<numFmts count="%d">`, len(s.order))
		for _, f := range s.order {
			fmt.Fprintf(&b, `<numFmt numFmtId="%d" formatCode="%s"/>`, s.formats[f], attr(f))
		}
		b.WriteString(`</numFmts>`)
	}
	// Fuentes, rellenos y bordes: la posición de cada uno es la de tonePos (el relleno 1 es «gray125», que Excel exige).
	b.WriteString(`<fonts count="6"><font><sz val="11"/><name val="Calibri"/></font>`)
	for _, t := range []Tone{Good, Warn, Bad, Head, HeadGreen} {
		bold := ""
		if t == Head || t == HeadGreen {
			bold = `<b/>`
		}
		fmt.Fprintf(&b, `<font>%s<sz val="11"/><color rgb="%s"/><name val="Calibri"/></font>`, bold, tones[t].font)
	}
	b.WriteString(`</fonts><fills count="7"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>`)
	for _, t := range []Tone{Good, Warn, Bad, Head, HeadGreen} {
		fmt.Fprintf(&b, `<fill><patternFill patternType="solid"><fgColor rgb="%s"/><bgColor indexed="64"/></patternFill></fill>`, tones[t].fill)
	}
	b.WriteString(`</fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>`)
	b.WriteString(`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>`)
	fmt.Fprintf(&b, `<cellXfs count="%d">`, len(s.keys))
	for _, k := range s.keys {
		numFmt := s.formats[k.format]
		pos := tonePos[k.tone]
		fill := 0
		if pos > 0 {
			fill = pos + 1
		}
		fmt.Fprintf(&b, `<xf numFmtId="%d" fontId="%d" fillId="%d" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1"`, numFmt, pos, fill)
		if k.tone == Head || k.tone == HeadGreen {
			b.WriteString(` applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf>`)
		} else {
			b.WriteString(`/>`)
		}
	}
	b.WriteString(`</cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`)
	return b.Bytes()
}

// attr escapa un valor para un atributo XML.
func attr(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return strings.ReplaceAll(b.String(), `"`, "&quot;")
}

func text(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// column devuelve la letra de la columna de Excel (0 = A, 26 = AA).
func column(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

func (sh Sheet) xml(st *styles) []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	width := 0
	for _, r := range sh.Rows {
		width = max(width, len(r))
	}
	if width > 0 && len(sh.Rows) > 0 {
		fmt.Fprintf(&b, `<dimension ref="A1:%s%d"/>`, column(width-1), len(sh.Rows))
	}
	b.WriteString(`<sheetViews><sheetView workbookViewId="0">`)
	if len(sh.Rows) > 1 {
		top := fmt.Sprintf("%s2", column(sh.FreezeCols))
		pane := "bottomRight"
		xs := ""
		if sh.FreezeCols > 0 {
			xs = fmt.Sprintf(` xSplit="%d"`, sh.FreezeCols)
		} else {
			pane = "bottomLeft"
		}
		fmt.Fprintf(&b, `<pane%s ySplit="1" topLeftCell="%s" activePane="%s" state="frozen"/>`, xs, top, pane)
	}
	b.WriteString(`</sheetView></sheetViews><sheetFormatPr defaultRowHeight="15"/>`)
	if len(sh.Widths) > 0 {
		b.WriteString(`<cols>`)
		for i, w := range sh.Widths {
			fmt.Fprintf(&b, `<col min="%d" max="%d" width="%s" customWidth="1"/>`, i+1, i+1, strconv.FormatFloat(w, 'f', -1, 64))
		}
		b.WriteString(`</cols>`)
	}
	b.WriteString(`<sheetData>`)
	for ri, row := range sh.Rows {
		if ri == 0 {
			b.WriteString(`<row r="1" ht="62" customHeight="1">`)
		} else {
			fmt.Fprintf(&b, `<row r="%d">`, ri+1)
		}
		for ci, c := range row {
			ref := fmt.Sprintf("%s%d", column(ci), ri+1)
			s := st.of(c.Format, c.Tone)
			switch v := c.Value.(type) {
			case float64:
				fmt.Fprintf(&b, `<c r="%s" s="%d"><v>%s</v></c>`, ref, s, strconv.FormatFloat(v, 'f', -1, 64))
			case string:
				fmt.Fprintf(&b, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, s, text(v))
			default:
				if c.Tone != None {
					fmt.Fprintf(&b, `<c r="%s" s="%d"/>`, ref, s)
				}
			}
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData>`)
	if width > 0 && len(sh.Rows) > 0 {
		fmt.Fprintf(&b, `<autoFilter ref="A1:%s%d"/>`, column(width-1), len(sh.Rows))
	}
	b.WriteString(`</worksheet>`)
	return b.Bytes()
}

// Write arma el archivo .xlsx de la hoja.
func Write(sh Sheet) ([]byte, error) {
	name := sh.Name
	if name == "" {
		name = "Hoja1"
	}
	st := newStyles()
	sheetXML := sh.xml(st) // arma primero la hoja: así los estilos usados quedan registrados
	files := []struct{ name, body string }{
		{"[Content_Types].xml", xml.Header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", xml.Header + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + attr(name) +
			`" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", xml.Header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", string(sheetXML)},
		{"xl/styles.xml", string(st.xml())},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

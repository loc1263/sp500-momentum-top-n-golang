package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func sample(t *testing.T) []byte {
	t.Helper()
	data, err := Write(Sheet{
		Name:       "Empresas",
		Widths:     []float64{10, 20, 12},
		FreezeCols: 1,
		Rows: [][]Cell{
			{{Value: "Ticker", Tone: Head}, {Value: "Empresa <&>", Tone: Head}, {Value: "ROE (%)", Tone: Head}},
			{{Value: "KO"}, {Value: "Coca-Cola \"The\" & Co"}, {Value: 40.6, Format: `0.0"%"`, Tone: Good}},
			{{Value: "ZZZ"}, {}, {Tone: Bad}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestWriteIsValidPackage(t *testing.T) {
	data := sample(t)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
		if err := xml.Unmarshal(b, new(any)); err != nil {
			t.Errorf("%s no es XML válido: %v", f.Name, err)
		}
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/worksheets/sheet1.xml", "xl/styles.xml"} {
		if parts[name] == "" {
			t.Errorf("falta %s", name)
		}
	}
	sheet := parts["xl/worksheets/sheet1.xml"]
	for _, want := range []string{`<dimension ref="A1:C3"/>`, `<autoFilter ref="A1:C3"/>`, `state="frozen"`, `xSplit="1"`, `<v>40.6</v>`,
		`Empresa &lt;&amp;&gt;`, `Coca-Cola &#34;The&#34; &amp; Co`} {
		if !strings.Contains(sheet, want) {
			t.Errorf("la hoja no contiene %q", want)
		}
	}
	if !strings.Contains(parts["xl/styles.xml"], `formatCode="0.0&#34;%&#34;"`) {
		t.Errorf("falta el formato de porcentaje: %s", parts["xl/styles.xml"])
	}
}

func TestHeadGreenStyle(t *testing.T) {
	data, err := Write(Sheet{Rows: [][]Cell{{{Value: "A", Tone: Head}, {Value: "B", Tone: HeadGreen}}, {{Value: 1.0}, {Value: 2.0}}}})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != "xl/styles.xml" {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		if err := xml.Unmarshal(b, new(any)); err != nil {
			t.Fatalf("styles.xml inválido: %v", err)
		}
		for _, want := range []string{`<fonts count="6">`, `<fills count="7">`, `FFC6E0B4`, `FFD9E1F2`} {
			if !strings.Contains(string(b), want) {
				t.Errorf("styles.xml no contiene %q", want)
			}
		}
		return
	}
	t.Fatal("falta xl/styles.xml")
}

func TestColumnLetters(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA", 701: "ZZ", 702: "AAA"} {
		if got := column(i); got != want {
			t.Errorf("column(%d) = %s, quiero %s", i, got, want)
		}
	}
}

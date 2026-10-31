package model

import (
	"math"
	"testing"
)

func TestFromRowsOrdersDatesAndTickers(t *testing.T) {
	p := FromRows([]PriceRow{
		{Ticker: "B", Date: "2026-01-05", Close: 5},
		{Ticker: "A", Date: "2026-01-02", Close: 1},
		{Ticker: "B", Date: "2026-01-02", Close: 2},
	})
	if p.Rows() != 2 || p.Cols() != 2 || p.Tickers[0] != "A" {
		t.Fatalf("forma: %dx%d, tickers %v", p.Rows(), p.Cols(), p.Tickers)
	}
	if !p.Dates[0].Before(p.Dates[1]) {
		t.Error("las fechas deben quedar ordenadas")
	}
	ja, _ := p.Col("A")
	jb, _ := p.Col("B")
	if p.At(0, ja) != 1 || p.At(0, jb) != 2 || p.At(1, jb) != 5 {
		t.Error("valores mal ubicados")
	}
	if v := p.At(1, ja); !math.IsNaN(v) { // A no tiene precio el 5: NaN
		t.Errorf("A el 5 de enero debería ser NaN, es %v", v)
	}
}

func TestFromRowsDropsRowsWithInvalidDates(t *testing.T) {
	p := FromRows([]PriceRow{
		{Ticker: "A", Date: "2026-01-02", Close: 1},
		{Ticker: "A", Date: "", Close: 99},            // sin fecha: entraría como el año 1 y quedaría primera
		{Ticker: "BAD", Date: "02/01/2026", Close: 7}, // formato equivocado: su ticker tampoco debe aparecer
	})
	if p.Rows() != 1 || p.Cols() != 1 || p.Tickers[0] != "A" {
		t.Fatalf("forma: %dx%d, tickers %v", p.Rows(), p.Cols(), p.Tickers)
	}
	if p.Dates[0].Year() != 2026 {
		t.Errorf("fecha: %v", p.Dates[0])
	}
	if p.At(0, 0) != 1 {
		t.Errorf("valor: %v", p.At(0, 0))
	}
}

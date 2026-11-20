// Criterio «sólida y barata» del listado del evaluador, calculado a partir de los niveles ya clasificados de cada empresa.
// Es un filtro para encontrar candidatas a revisar, no una recomendación.
//
// Fecha: 2026-11-20

package evaluator

// cheapMetrics son los múltiplos que deciden si una empresa es barata: no se usa el veredicto de Valoración porque también
// promedia beta, potencial al precio objetivo y consenso, que no miden el precio.
var cheapMetrics = []string{"pe", "ev_ebitda", "ev_fcf"}

// Screen dice si la empresa es sólida financieramente y si es barata.
//
//   - Sólida: Rentabilidad, Caja y Deuda y liquidez en «Bien». Los bancos y aseguradoras no tienen Deuda ni Caja, así que no califican.
//   - Barata: al menos dos de P/E, EV/EBITDA y EV/FCF con dato y ninguno en «Caro» (ni sin utilidad, sin caja libre).
func Screen(c Company) (solid, cheap bool) {
	verdicts := map[string]Level{}
	levels := map[string]Level{}
	for _, k := range c.Categories {
		verdicts[k.ID] = k.Verdict
		if k.ID == "val" {
			for _, m := range k.Metrics {
				levels[m.ID] = m.Level
			}
		}
	}
	solid = verdicts["prof"] == Good && verdicts["cash"] == Good && verdicts["debt"] == Good

	have, anyBad := 0, false
	for _, id := range cheapMetrics {
		switch levels[id] {
		case Good, Warn:
			have++
		case Bad:
			have++
			anyBad = true
		}
	}
	return solid, have >= 2 && !anyBad
}

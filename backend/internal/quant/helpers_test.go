// Ayudas compartidas por las pruebas de Markowitz.
//
// Fecha: 2026-11-19

package quant

// Optimize resuelve las carteras sin piso de peso ni paso de redondeo (los de OptimizeMin en cero).
func Optimize(tickers []string, mu []float64, sigma [][]float64, volTarget, wMax, riskFree float64, sectors []string, sectorMax float64) (Frontier, error) {
	return OptimizeMin(tickers, mu, sigma, volTarget, wMax, riskFree, sectors, sectorMax, 0, 0)
}

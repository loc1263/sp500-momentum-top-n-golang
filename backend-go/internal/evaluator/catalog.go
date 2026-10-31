package evaluator

// MetricDef es lo que no cambia de una métrica entre empresas: nombre, unidad y qué mide. Va una sola vez en el JSON.
// Unidades: pct (%), x (veces), money (monto en la moneda de la empresa), years (años), num (número simple), text.
type MetricDef struct {
	Name string `json:"name"`
	Unit string `json:"unit"`
	Why  string `json:"why"`
}

// SectionDef es una categoría del análisis. Labels son los textos del distintivo para bien, atención y alerta.
type SectionDef struct {
	Title  string    `json:"title"`
	Intro  string    `json:"intro,omitempty"`
	Why    string    `json:"why"`
	Labels [3]string `json:"labels"`
}

// Sections son las categorías, en el orden en que se muestran.
var Sections = []struct {
	ID string
	SectionDef
}{
	{"prof", SectionDef{"Rentabilidad", "", "Mide cuánto de lo que vende la empresa se convierte en ganancia y qué tan bien usa el dinero de sus accionistas.", [3]string{"Bien", "Atención", "Alerta"}}},
	{"grow", SectionDef{"Crecimiento", "", "Mide si las ventas y las ganancias de la empresa están aumentando frente al año anterior.", [3]string{"Bien", "Atención", "Alerta"}}},
	{"cash", SectionDef{"Caja", "", "Mira el efectivo real que entra y sale del negocio, que es más difícil de maquillar que la utilidad contable.", [3]string{"Bien", "Atención", "Alerta"}}},
	{"debt", SectionDef{"Deuda y liquidez", "", "Evalúa cuánto debe la empresa, si puede pagar los intereses y si tiene efectivo para cubrir lo que vence pronto.", [3]string{"Bien", "Atención", "Alerta"}}},
	{"val", SectionDef{"Valoración", "", "Compara el precio de la acción con lo que gana la empresa y con lo que esperan los analistas, para saber si está cara o barata.", [3]string{"Barata", "Media", "Cara"}}},
	{"trend", SectionDef{"Tendencia (5 años)", "", "Evolución de las cifras clave en los últimos años fiscales, para ver si el negocio mejora o empeora con el tiempo.", [3]string{"Bien", "Atención", "Alerta"}}},
	{"peers", SectionDef{"Vs. competidores", "Verde = mejor valor del grupo. «Mejor» es más de 10 % mejor que la mediana de los competidores (en P/E, más barato). Los competidores son empresas de la misma industria y mercado con capitalización parecida.",
		"Compara las métricas de la empresa con las de otras parecidas. Solo tiene sentido entre empresas del mismo sector.", [3]string{"Mejor", "Similar", "Peor"}}},
}

// Metrics es el diccionario de métricas por ID.
var Metrics = map[string]MetricDef{
	"net_margin":       {"Margen neto", "pct", "Ganancia final por cada 100 de ventas. Puede incluir ganancias por inversiones que no se repiten."},
	"op_margin":        {"Margen operativo (último trimestre)", "pct", "Ganancia del negocio en sí, antes de intereses e impuestos. Es una medida más limpia que la utilidad neta."},
	"roe":              {"ROE (retorno sobre patrimonio)", "pct", "Ganancia anual por cada 100 de patrimonio. Sobre 15 % se considera bueno. Si pasa de 60 %, suele deberse a recompras o pérdidas que dejaron un patrimonio muy pequeño, no solo a eficiencia: queda en «Atención»."},
	"rev_growth":       {"Crecimiento de ingresos", "pct", "Cuánto crecieron las ventas frente al año anterior."},
	"eps_growth":       {"Crecimiento del EPS", "pct", "Cuánto creció la ganancia por acción. Si crece mucho más que los ingresos, revisa si hay ganancias puntuales."},
	"earnings_quality": {"Calidad de la utilidad (Forward P/E ÷ P/E)", "x", "Compara lo que se espera ganar con lo ganado."},
	"fcf":              {"Flujo de caja libre (12 m)", "money", "Efectivo que queda después de pagar las inversiones. Si es negativo, la empresa gasta más de lo que genera y debe financiarse con deuda o caja."},
	"fcf_margin":       {"Margen de caja libre", "pct", "Caja libre por cada 100 de ventas."},
	"capex_cover":      {"Cobertura de la inversión (flujo operativo ÷ capex)", "x", "Cuántas veces el flujo operativo cubre la inversión en equipos e infraestructura. Menos de 1 significa que hay que financiar la diferencia."},
	"fcf_yield":        {"Rendimiento de caja libre (FCF ÷ Market Cap)", "pct", "Qué porcentaje de lo que vale la empresa en bolsa se convierte en caja libre cada año."},
	"net_debt":         {"Deuda neta (deuda − efectivo)", "money", "Lo que se debe después de usar el efectivo disponible. Se evalúa contra lo que el negocio genera al año."},
	"net_debt_years":   {"Años de flujo operativo para pagar la deuda neta", "years", "Menos de 1 año es cómodo; más de 3 años es una carga pesada."},
	"interest_cover":   {"Cobertura de intereses (resultado operativo ÷ intereses)", "x", "Cuántas veces alcanza la ganancia operativa para pagar los intereses. Bajo 3× es riesgoso."},
	"current_ratio":    {"Liquidez corriente", "x", "Activos de corto plazo ÷ deudas de corto plazo. Bajo 1× puede haber problemas para pagar lo que vence pronto."},
	"debt_equity":      {"Deuda ÷ patrimonio", "x", "Cuánta deuda hay por cada unidad que pertenece a los accionistas."},
	"pe":               {"P/E", "x", "Años de ganancias actuales que pagas por la acción. Menos de 15 suele ser barato y más de 30 caro, pero depende del sector."},
	"fwd_pe":           {"Forward P/E", "x", "Lo mismo, pero con la utilidad que se espera para los próximos 12 meses."},
	"target_upside":    {"Potencial hasta el precio objetivo", "pct", "Distancia entre el precio actual y el promedio que proyectan los analistas. Es una opinión, no una garantía."},
	"range_52w":        {"Posición en el rango de 52 semanas", "pct", "Dónde está el precio entre el mínimo (0 %) y el máximo (100 %) de las últimas 52 semanas."},
	"beta":             {"Beta (volatilidad frente al mercado)", "num", "1 = se mueve igual que el mercado. Más de 1 sube y baja más que el índice."},
	"dividend":         {"Dividendo", "pct", "Porcentaje del precio que se reparte cada año a los accionistas."},
	"rating":           {"Consenso de analistas", "text", "Recomendación promedio de los analistas que cubren la acción."},
	"tr_revenue":       {"Ingresos", "money", "Total de ventas del año fiscal, con su crecimiento anual compuesto (CAGR)."},
	"tr_net_income":    {"Utilidad neta", "money", "Ganancia final del año fiscal después de costos, intereses e impuestos."},
	"tr_fcf":           {"Flujo de caja libre", "money", "Efectivo que queda cada año después de invertir."},
	"tr_op_margin":     {"Margen operativo", "pct", "Ganancia operativa por cada 100 de ventas, año a año."},
	"tr_debt":          {"Deuda total", "money", "Deuda financiera al cierre de cada año fiscal. Según la fuente, puede incluir arrendamientos."},
}

// Glossary son las definiciones de los términos que se explican con el botón «?».
var Glossary = map[string]string{
	"Market cap": "Capitalización de mercado: lo que vale la empresa entera en bolsa (precio × acciones en circulación).",
	"EPS":        "Ganancia por acción: utilidad neta ÷ acciones en circulación.",
	"CAGR":       "Crecimiento anual compuesto: la tasa por año que lleva del valor inicial al final.",
	"Capex":      "Inversión en equipos e infraestructura. Aquí se deduce de flujo operativo − flujo de caja libre.",
}

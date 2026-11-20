// Página «About»: qué muestra cada sección, los conceptos clave, las fuentes de datos y los límites del backtest.
//
// Fecha: 2026-11-19

package site

type aboutItem struct{ Name, Text string }

type aboutPage struct {
	TopN, Months int
	Capital      string
	Commission   string
	Sections     []aboutItem
	Concepts     []aboutItem
	Sources      []aboutItem
	Limits       []string
}

func newAboutPage(m Meta) aboutPage {
	commission := 0.0
	if m.Commission != nil {
		commission = *m.Commission
	}
	return aboutPage{
		TopN: m.TopN, Months: m.Months, Capital: esNum(m.InitialCapital, 0), Commission: share(commission, 2),
		Sections: []aboutItem{
			{"Top actual", "Las acciones que hoy lideran el ranking de momentum (N-0), con suplentes, qué entra y sale, próximo rebalanceo y seguimiento de selecciones pasadas."},
			{"ETFs por gestora", "Los ETFs de iShares, Vanguard, SPDR, Invesco y Schwab: momentum ajustado por riesgo y calidad (costo y tamaño) dentro de cada categoría. Es una foto de hoy, sin backtest."},
			{"Modelo por score", "Scores de momentum, salud financiera y técnico para todo el S&P 500, su promedio y Markowitz sobre las mejores. Es una foto de hoy, sin backtest."},
			{"Evaluador de empresas", "Análisis fundamental de cada acción del S&P 500: rentabilidad, crecimiento, caja, deuda, valoración, tendencia y comparación con competidores, con un veredicto por categoría. Es independiente de los scores y usa reglas generales que dependen del sector."},
		},
		Concepts: []aboutItem{
			{"Momentum", "La idea de que las acciones que más subieron en los últimos meses tienden a seguir subiendo un tiempo. Se mide con el retorno del período."},
			{"Backtest", "Simula qué habría pasado en el pasado aplicando las mismas reglas. Sirve para evaluar la estrategia, pero no asegura lo que pasará."},
			{"Rebalanceo", "Cada N meses se revisa la cartera: salen las acciones que dejaron de liderar y entran las nuevas. Cada cambio paga comisión."},
			{"Rendimiento anual (CAGR)", "Cuánto creció el dinero por año, en promedio compuesto. Un 40% anual multiplica el capital por 1,4 cada año."},
			{"Resultado típico (mediana)", "El resultado del medio entre todas las fechas de arranque posibles: la mitad terminó mejor y la mitad peor."},
			{"Volatilidad", "Cuánto oscila el precio en un año (desviación estándar anual). Más alta = más riesgo, para bien y para mal."},
			{"Caída máxima", "La mayor baja desde un máximo hasta el mínimo que le siguió. Muestra lo que habría dolido en el peor momento."},
			{"Tasa libre de riesgo", "Lo que rinde una inversión sin riesgo (por ejemplo, bonos del Tesoro). Es el punto de comparación: 4% en el modelo por score."},
			{"Sharpe", "Retorno por unidad de riesgo: (retorno esperado − tasa libre de riesgo) ÷ volatilidad. Más alto = más retorno para el riesgo asumido."},
			{"Markowitz", "Reparte el dinero entre varias acciones según su retorno esperado, su riesgo y cómo se mueven juntas. Sin posiciones cortas y con tope por acción y por sector."},
			{"Máximo retorno", "Cartera de Markowitz con el mayor retorno esperado cuya volatilidad no supera el tope fijado (por ejemplo, 20% anual)."},
			{"Máximo Sharpe", "Cartera de Markowitz con el mejor Sharpe: la que más retorno da por cada unidad de riesgo."},
			{"Mínima varianza", "Cartera de Markowitz de menor volatilidad posible con esas acciones, sin mirar el retorno. La más tranquila."},
			{"Score (0-100)", "Posición de la acción frente a las demás en un indicador: 100 = la mejor, 0 = la peor. El score final promedia momentum, salud y técnico."},
			{"Salud financiera", "Score de los últimos balances: rentabilidad, solvencia (deuda), liquidez y generación de caja. La caja pesa el doble."},
			{"Score técnico", "Score del precio y el volumen: tendencia (medias móviles, ADX), impulso (MACD, RSI), volumen y posición en el rango de 52 semanas."},
			{"TER y AUM", "En los ETFs: el TER es el costo anual de mantenerlos (menor es mejor) y el AUM es el patrimonio que administran (mayor da más liquidez)."},
		},
		Sources: []aboutItem{
			{"Precios y datos financieros", "Yahoo Finance (precios ajustados, volumen y últimos estados financieros)."},
			{"Composición del S&P 500", "CSV de hanshof/sp500_constituents y altas y bajas de Wikipedia: en cada fecha solo compiten las empresas que estaban en el índice (Point-in-Time)."},
		},
		Limits: []string{
			"Sesgo de supervivencia: las empresas que ya no cotizan quedan fuera y los retornos probablemente están inflados.",
			"La capitalización large-cap y mega-cap usa las acciones en circulación de hoy, lo que adelanta la entrada de empresas hoy grandes.",
			"Pocas decisiones: en 5 años hay entre 5 y 20 rebalanceos por plazo.",
			"Modelo por score: datos financieros de los últimos publicados, sin histórico ni backtest.",
			"Sin impuestos ni deslizamiento (diferencia entre el precio de cierre y el obtenido).",
		},
	}
}

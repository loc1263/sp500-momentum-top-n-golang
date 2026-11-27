// Punto de entrada de Alpine: registra los componentes de la ficha de empresa y del Lab antes de que Alpine arranque.
// chart.js deja window.initCharts, que los gráficos del Lab llaman desde x-effect.
//
// Fecha: 2026-11-27

import './scripts/chart.js';
import lab from './scripts/lab.js';
import empresa from './scripts/empresa.js';

export default (Alpine) => {
  Alpine.data('lab', lab);
  Alpine.data('empresa', empresa);
};

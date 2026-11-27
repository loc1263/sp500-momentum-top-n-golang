// Espera a que carguen los scripts de datos (empresas.js, lab-datos.js), que son .js clásicos con defer: window 'load' ocurre
// después de que todos corrieron, sin importar en qué orden quedaron respecto de los módulos de Astro.
//
// Fecha: 2026-11-27

export const dataReady = () =>
  document.readyState === 'complete' ? Promise.resolve() : new Promise((resolve) => window.addEventListener('load', resolve, { once: true }));

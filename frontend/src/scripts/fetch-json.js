// Baja un JSON de public/ (ruta relativa a la página) y lo recuerda mientras la página siga abierta. null si no existe o falla.
//
// Fecha: 2026-11-27

const cache = new Map();

export const fetchJSON = (url) => {
  if (!cache.has(url)) {
    cache.set(
      url,
      fetch(url)
        .then((r) => (r.ok ? r.json() : null))
        .catch(() => null),
    );
  }
  return cache.get(url);
};

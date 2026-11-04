// La página no pide nada al backend: lee el JSON estático que deja el backend en Go al terminar
export async function loadReport() {
  try {
    const res = await fetch('data/report.json', { cache: 'no-cache' });
    if (res.ok && (res.headers.get('content-type') ?? '').includes('json')) return await res.json();
  } catch {
    /* se informa abajo */
  }
  throw new Error('No se pudo cargar data/report.json. Ejecute go run ./cmd/top3 pipeline en backend-go/ y recargue.');
}

// Universo identificado por su capitalización mínima (0 = índice completo)
export const capKeyOf = (u) => String(u.min_cap_billions ?? 0);

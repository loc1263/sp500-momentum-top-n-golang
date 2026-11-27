// Comportamiento común a todas las páginas: el botón de tema claro / oscuro (se recuerda en localStorage).
//
// Fecha: 2026-11-19

(() => {
  const btn = document.getElementById('theme');
  const root = document.documentElement;
  const paint = () => (btn.textContent = root.dataset.theme === 'dark' ? '☀️ Claro' : '🌙 Oscuro');
  btn.addEventListener('click', () => {
    const dark = root.dataset.theme !== 'dark';
    root.dataset.theme = dark ? 'dark' : 'light';
    try {
      localStorage.setItem('theme', dark ? 'dark' : 'light');
    } catch {}
    paint();
  });
  paint();
})();

// Las ayudas (?) y los puntos de color llevan su texto en title, que el tacto no muestra: un toque lo abre en un globo.
(() => {
  let pop = null;
  const close = () => {
    pop?.remove();
    pop = null;
  };
  document.addEventListener('click', (e) => {
    const t = e.target.closest?.('.hint, .dotv');
    const again = pop && pop.dataset.for === t?.title;
    close();
    if (!t?.title || again || !window.matchMedia('(hover: none)').matches) return;
    pop = document.createElement('div');
    pop.className = 'hint-pop';
    pop.setAttribute('role', 'status');
    pop.dataset.for = t.title;
    pop.textContent = t.title;
    document.body.append(pop);
    const r = t.getBoundingClientRect();
    pop.style.left = `${Math.max(12, Math.min(r.left, window.innerWidth - pop.offsetWidth - 12))}px`;
    pop.style.top = `${r.bottom + 6 + pop.offsetHeight > window.innerHeight ? r.top - pop.offsetHeight - 6 : r.bottom + 6}px`;
  });
  window.addEventListener('scroll', close, { passive: true });
})();

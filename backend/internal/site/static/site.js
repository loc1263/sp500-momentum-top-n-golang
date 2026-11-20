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

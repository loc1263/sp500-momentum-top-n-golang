// Conejito del pie de la pantalla: pasea solo, se aleja despacio del mouse (se deja alcanzar) y saluda si le hacen clic.
// Se oculta con «reducir movimiento» (y al imprimir, por CSS).
//
// Fecha: 2026-11-27

(() => {
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;

  const WIDTH = 40; // px, ancho aproximado del emoji
  const WANDER = 40; // px/s paseando
  const FLEE = 95; // px/s huyendo del mouse: lento, para poder alcanzarlo
  const SCARE = 200; // px: a esta distancia del cursor empieza a alejarse
  const GREETING_MS = 2500;

  const el = document.createElement('div');
  el.className = 'bunny';
  el.innerHTML = '<button type="button" class="body" aria-label="Conejito"><span class="turn"><span class="hop">🐇</span></span></button>';
  document.body.append(el);
  const body = el.querySelector('.body');
  const turn = el.querySelector('.turn');
  const hop = el.querySelector('.hop');

  let x = 0;
  let facing = 1; // 1 = mira a la derecha
  let greeting = false;
  let hovered = false; // con el cursor encima no huye: así se le puede hacer clic
  let mouse = null;
  let last = performance.now();
  let timer;

  window.addEventListener('mousemove', (e) => (mouse = { x: e.clientX, y: e.clientY }));
  body.addEventListener('mouseenter', () => (hovered = true));
  body.addEventListener('mouseleave', () => (hovered = false));
  body.addEventListener('click', () => {
    greeting = true;
    let sign = el.querySelector('.sign');
    if (!sign) {
      sign = document.createElement('div');
      sign.className = 'sign';
      sign.setAttribute('role', 'status');
      sign.textContent = '¡Hola! 👋';
      el.prepend(sign);
    }
    clearTimeout(timer);
    timer = setTimeout(() => {
      greeting = false;
      sign.remove();
    }, GREETING_MS);
  });

  const tick = (now) => {
    const dt = Math.min((now - last) / 1000, 0.1);
    last = now;
    const max = Math.max(window.innerWidth - WIDTH, 0);

    let step = 0;
    if (!greeting) {
      const dx = mouse ? x + WIDTH / 2 - mouse.x : 0;
      const dy = mouse ? window.innerHeight - 20 - mouse.y : 0;
      if (mouse && !hovered && Math.hypot(dx, dy) < SCARE) {
        // Se aleja del cursor; arrinconado contra un borde se queda quieto y se puede alcanzar
        const away = dx >= 0 ? 1 : -1;
        if ((away > 0 && x < max) || (away < 0 && x > 0)) step = away * FLEE * dt;
        else facing = -away;
      } else {
        if (x >= max) facing = -1;
        else if (x <= 0) facing = 1;
        step = facing * WANDER * dt;
      }
    }
    if (step) facing = Math.sign(step);
    x = Math.max(0, Math.min(x + step, max));
    hop.classList.toggle('moving', step !== 0);
    turn.style.transform = `scaleX(${-facing})`;
    el.style.transform = `translateX(${x}px)`;
    requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
})();

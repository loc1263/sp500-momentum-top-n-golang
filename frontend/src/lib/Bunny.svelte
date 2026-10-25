<!-- Conejito del pie de la pantalla: pasea solo, se aleja despacio del mouse (se deja alcanzar) y saluda si le hacés clic -->
<script>
  const WIDTH = 40; // px, ancho aproximado del emoji
  const WANDER = 40; // px/s paseando
  const FLEE = 95; // px/s huyendo del mouse: lento, para poder alcanzarlo
  const SCARE = 200; // px: a esta distancia del cursor empieza a alejarse
  const GREETING_MS = 2500;

  let el;
  let facing = $state(1); // 1 = mira a la derecha
  let moving = $state(false);
  let greeting = $state(false);
  let hovered = false; // con el cursor encima no huye: así se le puede hacer clic

  $effect(() => {
    if (typeof requestAnimationFrame !== 'function') return;
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;

    let x = 0;
    let mouse = null;
    let last = performance.now();
    let frame;

    const onMove = (e) => (mouse = { x: e.clientX, y: e.clientY });
    window.addEventListener('mousemove', onMove);

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
      moving = step !== 0;
      if (el) el.style.transform = `translateX(${x}px)`;
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);

    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('mousemove', onMove);
    };
  });

  let timer;
  function greet() {
    greeting = true;
    clearTimeout(timer);
    timer = setTimeout(() => (greeting = false), GREETING_MS);
  }
</script>

<div class="bunny" bind:this={el}>
  {#if greeting}<div class="sign" role="status">¡Hola! 👋</div>{/if}
  <button type="button" class="body" aria-label="Conejito" onclick={greet}
    onmouseenter={() => (hovered = true)} onmouseleave={() => (hovered = false)}>
    <span class="turn" style:transform="scaleX({-facing})"><span class="hop" class:moving>🐇</span></span>
  </button>
</div>

<style>
  .bunny { position: fixed; bottom: 0; left: 0; z-index: 50; pointer-events: none; font-size: 2rem; line-height: 1; }
  .body {
    pointer-events: auto; cursor: pointer; background: none; border: 0; padding: 0; font: inherit; line-height: inherit;
    display: block;
  }
  .turn, .hop { display: block; }
  .moving { animation: hop .3s ease-out infinite alternate; }
  .sign {
    position: absolute; bottom: 100%; left: 50%; transform: translateX(-50%); margin-bottom: 4px; white-space: nowrap;
    background: var(--card); color: var(--fg); border: 1px solid var(--line); border-radius: 8px; padding: 4px 10px;
    font-size: .9rem; box-shadow: 0 2px 6px rgb(0 0 0 / .2);
  }
  @keyframes hop { from { transform: translateY(0); } to { transform: translateY(-.5rem); } }

  @media (prefers-reduced-motion: reduce) { .bunny { display: none; } }
  @media print { .bunny { display: none; } }
</style>

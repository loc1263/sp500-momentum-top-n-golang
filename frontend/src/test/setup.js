import '@testing-library/jest-dom/vitest';

// jsdom no trae ResizeObserver (lo usa bind:clientWidth del gráfico)
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

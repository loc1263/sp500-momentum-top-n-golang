// Pruebas de los scripts del navegador (src/scripts/*.test.js) sobre jsdom.
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: { environment: 'jsdom', include: ['src/**/*.test.js'] },
});

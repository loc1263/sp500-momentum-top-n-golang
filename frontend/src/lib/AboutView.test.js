import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import AboutView from './AboutView.svelte';
import { report } from '../test/fixtures.js';

describe('AboutView', () => {
  it('explica en tarjetas los conceptos de Markowitz', () => {
    render(AboutView, { settings: report.settings });
    expect(screen.getByRole('heading', { name: 'Conceptos clave' })).toBeInTheDocument();
    for (const name of ['Sharpe', 'Mínima varianza', 'Máximo retorno', 'Markowitz', 'Volatilidad']) {
      expect(screen.getByText(name, { selector: 'strong' })).toBeInTheDocument();
    }
  });

  it('ya no trae la sección «Cómo se actualiza»', () => {
    render(AboutView, { settings: report.settings });
    expect(screen.queryByRole('heading', { name: 'Cómo se actualiza' })).toBeNull();
  });
});

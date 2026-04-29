import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CategoryBadge } from './CategoryBadge';
import type { FlagCategory } from '../types';

describe('CategoryBadge', () => {
  it('renders the category label', () => {
    render(<CategoryBadge category="release" />);
    expect(screen.getByText('release')).toBeInTheDocument();
  });

  it('encodes category into data-category', () => {
    render(<CategoryBadge category="feature" />);
    expect(screen.getByText('feature')).toHaveAttribute('data-category', 'feature');
  });

  it('renders all FlagCategory values with matching text and data-category', () => {
    const categories: FlagCategory[] = ['release', 'feature', 'experiment', 'ops', 'permission'];
    categories.forEach((c) => {
      const { unmount } = render(<CategoryBadge category={c} />);
      const el = screen.getByText(c);
      expect(el).toBeInTheDocument();
      expect(el).toHaveAttribute('data-category', c);
      unmount();
    });
  });
});

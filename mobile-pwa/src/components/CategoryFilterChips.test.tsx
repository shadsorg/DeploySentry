import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CategoryFilterChips } from './CategoryFilterChips';

describe('CategoryFilterChips', () => {
  it('renders an All chip plus one chip per category', () => {
    render(<CategoryFilterChips value={[]} onChange={() => {}} />);
    ['All', 'release', 'feature', 'experiment', 'ops', 'permission'].forEach((label) => {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument();
    });
  });

  it('marks All as pressed when value is empty', () => {
    render(<CategoryFilterChips value={[]} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true');
    ['release', 'feature', 'experiment', 'ops', 'permission'].forEach((label) => {
      expect(screen.getByRole('button', { name: label })).toHaveAttribute('aria-pressed', 'false');
    });
  });

  it('marks active categories as pressed and All as unpressed when value is non-empty', () => {
    render(<CategoryFilterChips value={['release', 'ops']} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'release' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'ops' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'feature' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('tapping a category chip adds it to value', async () => {
    const onChange = vi.fn();
    render(<CategoryFilterChips value={[]} onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'release' }));
    expect(onChange).toHaveBeenCalledWith(['release']);
  });

  it('tapping a second chip extends the set', async () => {
    const onChange = vi.fn();
    render(<CategoryFilterChips value={['release']} onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'feature' }));
    expect(onChange).toHaveBeenCalledWith(['release', 'feature']);
  });

  it('tapping a chip already in the set removes it', async () => {
    const onChange = vi.fn();
    render(<CategoryFilterChips value={['release', 'feature']} onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'release' }));
    expect(onChange).toHaveBeenCalledWith(['feature']);
  });

  it('tapping All clears the value to []', async () => {
    const onChange = vi.fn();
    render(<CategoryFilterChips value={['release', 'feature']} onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'All' }));
    expect(onChange).toHaveBeenCalledWith([]);
  });
});

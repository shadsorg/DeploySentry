import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StatusFilterChips } from './StatusFilterChips';

describe('StatusFilterChips', () => {
  it('renders five chips: All / Pending / Running / Completed / Failed', () => {
    render(<StatusFilterChips value="" onChange={() => {}} />);
    ['All', 'Pending', 'Running', 'Completed', 'Failed'].forEach((label) => {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument();
    });
  });

  it('marks the active chip with aria-pressed=true', () => {
    render(<StatusFilterChips value="failed" onChange={() => {}} />);
    expect(screen.getByRole('button', { name: 'Failed' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('emits the canonical status string on click (empty for All)', async () => {
    const onChange = vi.fn();
    render(<StatusFilterChips value="" onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'Failed' }));
    expect(onChange).toHaveBeenCalledWith('failed');
    await userEvent.click(screen.getByRole('button', { name: 'All' }));
    expect(onChange).toHaveBeenCalledWith('');
  });
});

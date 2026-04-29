import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ToggleSwitch } from './ToggleSwitch';

describe('ToggleSwitch', () => {
  it('renders an <input type="checkbox" role="switch"> with the supplied aria-label', () => {
    render(<ToggleSwitch checked={false} onChange={() => {}} ariaLabel="Enable feature" />);
    const input = screen.getByRole('switch', { name: 'Enable feature' });
    expect(input).toBeInTheDocument();
    expect(input.tagName).toBe('INPUT');
    expect(input).toHaveAttribute('type', 'checkbox');
  });

  it('reflects the checked prop', () => {
    render(<ToggleSwitch checked onChange={() => {}} ariaLabel="On toggle" />);
    const input = screen.getByRole('switch', { name: 'On toggle' }) as HTMLInputElement;
    expect(input.checked).toBe(true);
  });

  it('calls onChange(true) when clicked while unchecked', async () => {
    const onChange = vi.fn();
    render(<ToggleSwitch checked={false} onChange={onChange} ariaLabel="Click me" />);
    await userEvent.click(screen.getByRole('switch', { name: 'Click me' }));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('does not fire onChange when disabled', async () => {
    const onChange = vi.fn();
    render(
      <ToggleSwitch checked={false} onChange={onChange} ariaLabel="Disabled toggle" disabled />,
    );
    const input = screen.getByRole('switch', { name: 'Disabled toggle' });
    expect(input).toBeDisabled();
    await userEvent.click(input);
    expect(onChange).not.toHaveBeenCalled();
  });

  it('marks data-loading="true" and blocks onChange when loading', async () => {
    const onChange = vi.fn();
    const { container } = render(
      <ToggleSwitch checked={false} onChange={onChange} ariaLabel="Loading toggle" loading />,
    );
    const wrapper = container.querySelector('.m-toggle');
    expect(wrapper).toHaveAttribute('data-loading', 'true');
    await userEvent.click(screen.getByRole('switch', { name: 'Loading toggle' }));
    expect(onChange).not.toHaveBeenCalled();
  });
});

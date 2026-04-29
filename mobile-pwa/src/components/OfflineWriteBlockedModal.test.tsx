import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { OfflineWriteBlockedModal } from './OfflineWriteBlockedModal';

describe('OfflineWriteBlockedModal', () => {
  it('renders nothing when open is false', () => {
    const { container } = render(
      <OfflineWriteBlockedModal open={false} onClose={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders an alertdialog with the "You\'re offline" heading when open', () => {
    render(<OfflineWriteBlockedModal open onClose={() => {}} />);
    const dialog = screen.getByRole('alertdialog');
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveAccessibleName("You're offline");
    expect(screen.getByText("You're offline")).toBeInTheDocument();
  });

  it('tapping "Got it" calls onClose', async () => {
    const onClose = vi.fn();
    render(<OfflineWriteBlockedModal open onClose={onClose} />);
    await userEvent.click(screen.getByRole('button', { name: /Got it/ }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('tapping the backdrop calls onClose', async () => {
    const onClose = vi.fn();
    render(<OfflineWriteBlockedModal open onClose={onClose} />);
    await userEvent.click(screen.getByTestId('m-offline-modal-backdrop'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

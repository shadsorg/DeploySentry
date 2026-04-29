import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StaleBadge } from './StaleBadge';

describe('StaleBadge', () => {
  it('renders nothing when lastSuccess is null (first ever load)', () => {
    const { container } = render(<StaleBadge lastSuccess={null} inflight={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when data is fresher than the threshold', () => {
    const { container } = render(
      <StaleBadge lastSuccess={Date.now() - 5_000} inflight={true} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders a "Showing data from <relative time>" pill when older than threshold', () => {
    render(<StaleBadge lastSuccess={Date.now() - 60_000} inflight={false} />);
    expect(screen.getByText(/Showing data from/i)).toBeInTheDocument();
    expect(screen.getByText(/ago/i)).toBeInTheDocument();
  });

  it('marks the pill with data-refreshing when inflight is true', () => {
    render(<StaleBadge lastSuccess={Date.now() - 60_000} inflight={true} />);
    const pill = screen.getByText(/Showing data from/i);
    expect(pill).toHaveAttribute('data-refreshing', 'true');
  });

  it('omits data-refreshing when not inflight', () => {
    render(<StaleBadge lastSuccess={Date.now() - 60_000} inflight={false} />);
    const pill = screen.getByText(/Showing data from/i);
    expect(pill).not.toHaveAttribute('data-refreshing');
  });

  it('respects a custom thresholdMs', () => {
    // 10s old, threshold 5s -> should render
    render(
      <StaleBadge lastSuccess={Date.now() - 10_000} inflight={false} thresholdMs={5_000} />,
    );
    expect(screen.getByText(/Showing data from/i)).toBeInTheDocument();
  });
});

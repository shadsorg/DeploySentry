import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StagedBadge } from './StagedBadge';

describe('StagedBadge', () => {
  it('renders "pending create" when action is create', () => {
    render(<StagedBadge marker={{ action: 'create', staged_at: '2026-05-03T10:00:00Z' }} />);
    expect(screen.getByText(/pending create/i)).toBeInTheDocument();
  });

  it('renders "pending update" when action is update', () => {
    render(<StagedBadge marker={{ action: 'update', staged_at: '2026-05-03T10:00:00Z' }} />);
    expect(screen.getByText(/pending update/i)).toBeInTheDocument();
  });

  it('renders nothing when marker is null', () => {
    const { container } = render(<StagedBadge marker={null} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when marker is undefined', () => {
    const { container } = render(<StagedBadge marker={undefined} />);
    expect(container.firstChild).toBeNull();
  });
});

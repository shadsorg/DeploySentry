import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProjectFilterSheet } from './ProjectFilterSheet';
import type { Project } from '../types';

const PROJECTS: Project[] = [
  { id: 'p1', slug: 'pay', name: 'Payments', org_id: 'o1' },
  { id: 'p2', slug: 'web', name: 'Website', org_id: 'o1' },
];

describe('ProjectFilterSheet', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <ProjectFilterSheet open={false} projects={PROJECTS} value="" onSelect={() => {}} onClose={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders All projects + each project name when open', () => {
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="" onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByRole('button', { name: /all projects/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Payments' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Website' })).toBeInTheDocument();
  });

  it('marks the selected project with aria-pressed=true', () => {
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="p2" onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByRole('button', { name: 'Website' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('calls onSelect + onClose when a project is tapped', async () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="" onSelect={onSelect} onClose={onClose} />,
    );
    await userEvent.click(screen.getByRole('button', { name: 'Payments' }));
    expect(onSelect).toHaveBeenCalledWith('p1');
    expect(onClose).toHaveBeenCalled();
  });

  it('All projects emits empty string', async () => {
    const onSelect = vi.fn();
    render(
      <ProjectFilterSheet open projects={PROJECTS} value="p1" onSelect={onSelect} onClose={() => {}} />,
    );
    await userEvent.click(screen.getByRole('button', { name: /all projects/i }));
    expect(onSelect).toHaveBeenCalledWith('');
  });
});

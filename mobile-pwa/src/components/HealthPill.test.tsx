import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { HealthPill } from './HealthPill';

describe('HealthPill', () => {
  it('renders HEALTHY for healthy state', () => {
    render(<HealthPill state="healthy" />);
    expect(screen.getByText(/healthy/i)).toBeInTheDocument();
  });
  it('renders UNHEALTHY for unhealthy state', () => {
    render(<HealthPill state="unhealthy" />);
    expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
  });
  it('renders UNKNOWN for unknown state', () => {
    render(<HealthPill state="unknown" />);
    expect(screen.getByText(/unknown/i)).toBeInTheDocument();
  });
  it('applies data-state attribute for CSS hooks', () => {
    render(<HealthPill state="degraded" />);
    expect(screen.getByText(/degraded/i)).toHaveAttribute('data-state', 'degraded');
  });
});

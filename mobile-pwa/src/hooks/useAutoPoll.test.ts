import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAutoPoll } from './useAutoPoll';

describe('useAutoPoll', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
  });

  it('calls the tick function immediately on mount', () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    expect(tick).toHaveBeenCalledTimes(1);
  });

  it('calls tick on the interval while visible', async () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    expect(tick).toHaveBeenCalledTimes(1);
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    expect(tick).toHaveBeenCalledTimes(2);
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });
    expect(tick).toHaveBeenCalledTimes(3);
  });

  it('pauses ticking when the page becomes hidden', async () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    expect(tick).toHaveBeenCalledTimes(1);
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(tick).toHaveBeenCalledTimes(1);
  });

  it('resumes + immediately ticks when visibility returns', async () => {
    const tick = vi.fn();
    renderHook(() => useAutoPoll(tick, 1000));
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    tick.mockClear();
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    expect(tick).toHaveBeenCalledTimes(1);
  });
});

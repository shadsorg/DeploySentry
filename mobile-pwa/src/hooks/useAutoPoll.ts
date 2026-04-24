import { useEffect, useRef } from 'react';

/**
 * Calls tick() immediately, then every intervalMs while the page is visible.
 * Pauses on document.visibilitychange → hidden; resumes (with an immediate
 * tick) on → visible. Cleans up on unmount.
 */
export function useAutoPoll(tick: () => void, intervalMs: number): void {
  const tickRef = useRef(tick);
  tickRef.current = tick;

  useEffect(() => {
    let timer: number | null = null;

    const schedule = () => {
      if (timer !== null) return;
      timer = window.setInterval(() => tickRef.current(), intervalMs);
    };
    const stop = () => {
      if (timer === null) return;
      window.clearInterval(timer);
      timer = null;
    };

    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        stop();
      } else {
        tickRef.current();
        schedule();
      }
    };

    tickRef.current();
    if (document.visibilityState !== 'hidden') {
      schedule();
    }
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      stop();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [intervalMs]);
}

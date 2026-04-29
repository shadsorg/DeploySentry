import { describe, it, expect, vi } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('../registerSW', () => {
  type Listener = (state: { needRefresh: boolean; offlineReady: boolean }) => void;
  let listener: Listener | null = null;
  return {
    subscribeServiceWorker: (l: Listener) => {
      listener = l;
      l({ needRefresh: false, offlineReady: false });
      return () => {
        listener = null;
      };
    },
    applyServiceWorkerUpdate: vi.fn().mockResolvedValue(undefined),
    __triggerNeedRefresh: () => listener?.({ needRefresh: true, offlineReady: false }),
  };
});

import { SwUpdateBanner } from './SwUpdateBanner';
import * as registerSWModule from '../registerSW';

const mocked = registerSWModule as unknown as {
  applyServiceWorkerUpdate: ReturnType<typeof vi.fn>;
  __triggerNeedRefresh: () => void;
};

describe('SwUpdateBanner', () => {
  it('renders nothing when needRefresh is false', () => {
    const { container } = render(<SwUpdateBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders banner with text and Reload button when needRefresh becomes true', () => {
    render(<SwUpdateBanner />);
    act(() => {
      mocked.__triggerNeedRefresh();
    });
    expect(screen.getByText(/update available/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /reload/i })).toBeInTheDocument();
  });

  it('calls applyServiceWorkerUpdate when Reload is tapped', async () => {
    render(<SwUpdateBanner />);
    act(() => {
      mocked.__triggerNeedRefresh();
    });
    await userEvent.click(screen.getByRole('button', { name: /reload/i }));
    expect(mocked.applyServiceWorkerUpdate).toHaveBeenCalled();
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { stageOrCall } from './stageOrCall';

const mockStage = vi.fn();

vi.mock('@/api', () => ({
  stagingApi: {
    stage: (...args: unknown[]) => mockStage(...args),
  },
}));

beforeEach(() => {
  mockStage.mockReset();
});

describe('stageOrCall', () => {
  it('calls the direct fn when staged is false', async () => {
    const direct = vi.fn().mockResolvedValue({ ok: true });
    const result = await stageOrCall({
      staged: false,
      orgSlug: 'acme',
      stage: { resource_type: 'flag', resource_id: 'f', action: 'archive' },
      direct,
    });
    expect(direct).toHaveBeenCalled();
    expect(mockStage).not.toHaveBeenCalled();
    expect(result).toEqual({ mode: 'direct', result: { ok: true } });
  });

  it('posts to the staging endpoint when staged is true', async () => {
    mockStage.mockResolvedValue({ id: 'staged-1' });
    const direct = vi.fn();
    const result = await stageOrCall({
      staged: true,
      orgSlug: 'acme',
      stage: { resource_type: 'flag', resource_id: 'f', action: 'archive' },
      direct,
    });
    expect(direct).not.toHaveBeenCalled();
    expect(mockStage).toHaveBeenCalledWith('acme', {
      resource_type: 'flag',
      resource_id: 'f',
      action: 'archive',
    });
    expect(result).toEqual({ mode: 'staged', row: { id: 'staged-1' } });
  });

  it('propagates errors from the direct path', async () => {
    const direct = vi.fn().mockRejectedValue(new Error('boom'));
    await expect(
      stageOrCall({
        staged: false,
        orgSlug: 'acme',
        stage: { resource_type: 'flag', resource_id: 'f', action: 'archive' },
        direct,
      }),
    ).rejects.toThrow('boom');
  });

  it('propagates errors from the staged path', async () => {
    mockStage.mockRejectedValue(new Error('staging api 500'));
    await expect(
      stageOrCall({
        staged: true,
        orgSlug: 'acme',
        stage: { resource_type: 'flag', resource_id: 'f', action: 'archive' },
        direct: vi.fn(),
      }),
    ).rejects.toThrow('staging api 500');
  });
});

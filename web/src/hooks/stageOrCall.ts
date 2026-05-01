import { stagingApi, type StageRequest, type StagedChange } from '@/api';

/**
 * Outcome of a staged-or-direct mutation. `mode === 'staged'` means the
 * change was queued via `/deploy-changes/stage`; the caller can use this
 * to render a "queued" affordance (toast, banner pulse) instead of the
 * usual success state. `mode === 'direct'` means the production path ran.
 */
export type StageOrCallResult<T> =
  | { mode: 'staged'; row: StagedChange }
  | { mode: 'direct'; result: T };

/**
 * stageOrCall routes a UI mutation through the staging layer when the
 * caller's per-org `staged-changes-enabled` flag is on, otherwise calls
 * the supplied direct-write function. Both branches reject on error so
 * the caller's existing try/catch shape works unchanged.
 *
 * Caller is responsible for:
 *   - resolving the staged-enabled flag (use `useStagingEnabled(orgSlug)`)
 *   - any optimistic UI / rollback semantics — the staged path doesn't
 *     change production, so rolling back optimistic state on stage success
 *     would be wrong; the caller should leave the optimistic update in
 *     place even when staging.
 *
 * The orgSlug must be passed; staging is per-org by definition.
 */
export async function stageOrCall<T>(opts: {
  staged: boolean;
  orgSlug: string;
  stage: StageRequest;
  direct: () => Promise<T>;
}): Promise<StageOrCallResult<T>> {
  if (opts.staged) {
    const row = await stagingApi.stage(opts.orgSlug, opts.stage);
    return { mode: 'staged', row };
  }
  const result = await opts.direct();
  return { mode: 'direct', result };
}

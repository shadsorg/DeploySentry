import type { AuditLogEntry } from '@/types';

/**
 * Humanize an audit entry's action string. The page-level resolver supplies
 * the resource label (flag key, env name, etc.); this helper returns just the
 * verb portion. Falls back to the raw action when unknown.
 */
export function actionLabel(entry: AuditLogEntry): string {
  switch (entry.action) {
    case 'flag.created':                  return 'Created flag';
    case 'flag.updated':                  return 'Updated flag';
    case 'flag.archived':                 return 'Archived flag';
    case 'flag.toggled':                  return 'Toggled flag';
    case 'flag.rule.created':             return 'Added rule';
    case 'flag.rule.deleted':             return 'Removed rule';
    case 'flag.rule.env_state.updated':   return 'Changed rule env state';
    case 'flag.env_state.updated':        return 'Changed env default value';

    // Reverted variants (ship as `<original>.reverted` from the backend).
    case 'flag.created.reverted':                  return 'Reverted: Created flag';
    case 'flag.updated.reverted':                  return 'Reverted: Updated flag';
    case 'flag.archived.reverted':                 return 'Reverted: Archived flag';
    case 'flag.toggled.reverted':                  return 'Reverted: Toggled flag';
    case 'flag.rule.created.reverted':             return 'Reverted: Added rule';
    case 'flag.rule.deleted.reverted':             return 'Reverted: Removed rule';
    case 'flag.rule.env_state.updated.reverted':   return 'Reverted: Changed rule env state';
    case 'flag.env_state.updated.reverted':        return 'Reverted: Changed env default value';

    default: return entry.action;
  }
}

/**
 * Reasons we surface in the tooltip when a row's `revertible` is false.
 * Most actions hit the default fallback ("Cannot revert this action"), but
 * a few have specific copy.
 */
export const NON_REVERTIBLE_REASONS: Record<string, string> = {
  'flag.hard_deleted':  'Hard-deleted flags cannot be restored',
  'apikey.created':     'API keys are sensitive — create a new one instead',
  'apikey.revoked':     'API keys are sensitive — create a new one instead',
  'deployment.created': 'Deployments are immutable history',
  'deployment.failed':  'Deployments are immutable history',
};

export function nonRevertibleReason(entry: AuditLogEntry): string {
  return NON_REVERTIBLE_REASONS[entry.action] ?? 'Cannot revert this action';
}

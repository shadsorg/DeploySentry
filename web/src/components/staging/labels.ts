import type { StagedChange } from '@/api';

/**
 * Resource-type label shown in the review-page section headers and
 * elsewhere. Plural — one row reads "Feature flags" not "Feature flag".
 */
export function resourceTypeLabel(resourceType: string): string {
  switch (resourceType) {
    case 'flag':
      return 'Feature flags';
    case 'flag_env_state':
      return 'Per-environment flag state';
    case 'flag_rule':
      return 'Targeting rules';
    case 'setting':
      return 'Settings';
    case 'member':
      return 'Members';
    case 'strategy':
      return 'Rollout strategies';
    default:
      return resourceType;
  }
}

/**
 * Human-readable verb for a staged action. The diff shows the actual
 * before/after; this is just the row's headline.
 */
export function actionLabel(row: StagedChange): string {
  const isCreate = row.action === 'create';
  if (isCreate) return 'Create';

  switch (row.action) {
    case 'update':
      return row.field_path ? `Update ${row.field_path}` : 'Update';
    case 'toggle':
      return 'Toggle';
    case 'archive':
      return 'Archive';
    case 'restore':
      return 'Restore';
    case 'delete':
      return 'Delete';
    default:
      return row.action;
  }
}

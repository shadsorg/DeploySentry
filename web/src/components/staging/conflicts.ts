import type { StagedChange } from '@/api';

export interface ConflictMap {
  [rowId: string]: boolean;
}

/**
 * computeConflicts compares rows pairwise: when two staged changes from this
 * user (latest-edit-wins already collapsed by the backend upsert) target
 * the same (resource, field), the older one is shadowed by the newer.
 *
 * Phase B has no per-row production-state lookup — Phase C will add a
 * dashboard read endpoint for that. For now we only mark a row as a
 * conflict when its old_value disagrees with the previous row's new_value
 * for the same target. This is a conservative heuristic that catches the
 * "User A staged something, then User B committed a different value"
 * pattern as soon as A refreshes the page (the staged old_value won't
 * match what they would now see).
 */
export function computeConflicts(rows: StagedChange[]): ConflictMap {
  const seen = new Map<string, unknown>();
  const conflicts: ConflictMap = {};
  for (const r of rows) {
    const target = `${r.resource_type}:${r.resource_id ?? r.provisional_id ?? ''}:${r.field_path ?? ''}`;
    if (seen.has(target)) {
      const expectedOld = seen.get(target);
      if (JSON.stringify(expectedOld) !== JSON.stringify(r.old_value)) {
        conflicts[r.id] = true;
      }
    }
    seen.set(target, r.new_value);
  }
  return conflicts;
}

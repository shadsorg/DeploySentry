import type { FlagCategory } from '../types';

const CATEGORIES: FlagCategory[] = ['release', 'feature', 'experiment', 'ops', 'permission'];

export function CategoryFilterChips({
  value,
  onChange,
}: {
  value: FlagCategory[];
  onChange: (next: FlagCategory[]) => void;
}) {
  const allActive = value.length === 0;
  return (
    <div className="m-filter-chip-row" role="group" aria-label="Filter by category">
      <button
        type="button"
        className="m-filter-chip"
        aria-pressed={allActive}
        onClick={() => onChange([])}
      >
        All
      </button>
      {CATEGORIES.map((cat) => {
        const active = value.includes(cat);
        return (
          <button
            key={cat}
            type="button"
            className="m-filter-chip"
            aria-pressed={active}
            onClick={() =>
              onChange(active ? value.filter((c) => c !== cat) : [...value, cat])
            }
          >
            {cat}
          </button>
        );
      })}
    </div>
  );
}

import type { FlagCategory } from '../types';

export function CategoryBadge({ category }: { category: FlagCategory }) {
  return (
    <span className="m-cat-badge" data-category={category}>
      {category}
    </span>
  );
}

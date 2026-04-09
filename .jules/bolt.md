## 2024-04-09 - Unoptimized List Filtering in React
**Learning:** Found a recurring anti-pattern where `.toLowerCase()` is called repeatedly inside unmemoized `.filter()` loops during render. This causes O(N) penalties and triggers unnecessary React re-renders.
**Action:** When working with lists, always use `useMemo` for derived data like filtering, and hoist repeated string allocations (`search?.toLowerCase() ?? ''`) outside the filter loop to avoid these performance penalties.

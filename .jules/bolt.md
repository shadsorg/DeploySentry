## 2024-04-05 - Optimize Expensive Array Sorting and Filtering in React Renders
**Learning:** Found an opportunity to prevent `O(N log N)` sorting and extensive string matching functions from being called sequentially on every React render hook (even due to unrelated state changes). Memoization with `useMemo` avoids redundant re-computation of array sorting and filtering.
**Action:** When working in React rendering logic with array mutations like sorting, filtering, and `.toLowerCase().includes()`, proactively extract these heavy operations to `useMemo` hooks so they only re-run when their explicit dependent values update.

## 2024-05-18 - Hoisting Repeated String Operations in React Filtering
**Learning:** In React components like `DeploymentsPage.tsx` and `FlagListPage.tsx`, placing `search.toLowerCase()` directly inside a `.filter` method forces the JS engine to re-allocate and lowercase the same string O(N) times on every render when the filtered list recalculates.
**Action:** When filtering lists in React based on a search input, always hoist operations that depend only on external state (like `search.toLowerCase()`) outside the `.filter` loop (e.g. at the top of a `useMemo` block). Always pair this with optional chaining (e.g., `search?.toLowerCase() ?? ''`) to ensure type safety and prevent runtime crashes.

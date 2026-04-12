## 2026-04-11 - Optimize React Lists by Hoisting Out of Filter
**Learning:** Found an O(N) penalty in filtering large lists in React because `search.toLowerCase()` was recalculated inside the array filter loop on every re-render.
**Action:** Always hoist constants (like lowercasing search terms) out of the `filter` loop and wrap the entire result in `useMemo` to prevent redundant calculations during React re-renders.

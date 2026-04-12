## 2026-04-11 - Optimize React Lists by Hoisting Out of Filter
**Learning:** Found an O(N) penalty in filtering large lists in React because `search.toLowerCase()` was recalculated inside the array filter loop on every re-render.
**Action:** Always hoist constants (like lowercasing search terms) out of the `filter` loop and wrap the entire result in `useMemo` to prevent redundant calculations during React re-renders.
## 2026-04-12 - CI Environment Consistency (Go version and Node version)
**Learning:** Found that CI failed because GitHub Actions runners were using deprecated Node.js 20 versions which are no longer supported by actions, and `golangci-lint` failed because it was downloaded/built with Go 1.24 but the Go project required Go 1.25, which caused an unexpected config load error.
**Action:** Always ensure the CI workflows are kept up to date with the latest project requirements (e.g. updating `GO_VERSION: "1.25"` and `NODE_VERSION: "22"` in GitHub Actions YAML files) to prevent infrastructure-related CI failures.

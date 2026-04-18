## 1.1.0

- Fix: SSE events now treated as invalidation signals instead of flag data
- Fix: Fetch individual flag by ID on change for consistent state
- Fix: Add environment_id to flag fetch and stream URLs

## 1.0.0

- Initial release
- Feature flag evaluation with typed accessors (bool, string, int, JSON)
- Rich metadata support (category, owners, expiration, tags)
- In-memory caching with configurable TTL
- Real-time SSE streaming for flag updates
- InheritedWidget provider for widget-tree integration
- Register/dispatch pattern for flag-based operation routing

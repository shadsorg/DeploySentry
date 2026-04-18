/// DeploySentry Flutter SDK
///
/// Feature flag evaluation with rich metadata, in-memory caching,
/// real-time SSE streaming, and an InheritedWidget provider for easy
/// widget-tree integration.
library dr_sentry_flutter;

export 'src/cache.dart' show FlagCache;
export 'src/client.dart' show DeploySentryClient;
export 'src/models.dart'
    show
        EvaluationContext,
        EvaluationResult,
        Flag,
        FlagCategory,
        FlagMetadata;
export 'src/provider.dart' show DeploySentry, DeploySentryProvider;
export 'src/streaming.dart' show FlagStreamClient;

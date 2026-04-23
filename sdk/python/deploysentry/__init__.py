"""DeploySentry Python SDK -- feature flag evaluation with rich metadata."""

from .async_client import AsyncDeploySentryClient
from .cache import TTLCache
from .client import DeploySentryClient
from .models import (
    EvaluationContext,
    EvaluationResult,
    Flag,
    FlagCategory,
    FlagMetadata,
)
from .status_reporter import HealthReport, StatusReporter, resolve_version

__all__ = [
    "AsyncDeploySentryClient",
    "DeploySentryClient",
    "EvaluationContext",
    "EvaluationResult",
    "Flag",
    "FlagCategory",
    "FlagMetadata",
    "TTLCache",
    "HealthReport",
    "StatusReporter",
    "resolve_version",
]

__version__ = "1.0.0"

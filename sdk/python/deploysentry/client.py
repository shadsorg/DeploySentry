"""Synchronous DeploySentry client."""

from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Any, Callable, Dict, List, Optional

import httpx

from .cache import TTLCache
from .models import (
    EvaluationContext,
    EvaluationResult,
    Flag,
    FlagCategory,
    FlagMetadata,
)
from .status_reporter import HealthReport, StatusReporter
from .streaming import SSEClient

logger = logging.getLogger("deploysentry.client")


class DeploySentryClient:
    """Synchronous client for the DeploySentry feature-flag service.

    Parameters
    ----------
    api_key:
        API key used to authenticate with the DeploySentry API.
    base_url:
        Base URL of the DeploySentry API (no trailing slash).
    environment:
        Target environment (e.g. ``"production"``, ``"staging"``).
    project:
        Project identifier.
    cache_timeout:
        TTL in seconds for the in-memory flag cache.  Set to ``0`` to disable
        caching entirely.
    offline_mode:
        When ``True`` the client will not make any network requests.  All
        evaluations return the supplied default values.
    session_id:
        Optional session identifier for consistent flag evaluation.  When set
        the header ``X-DeploySentry-Session`` is sent with every request.
    """

    def __init__(
        self,
        api_key: str,
        base_url: str = "https://api.dr-sentry.com",
        environment: str = "production",
        project: str = "",
        cache_timeout: float = 30,
        offline_mode: bool = False,
        session_id: Optional[str] = None,
        *,
        application_id: Optional[str] = None,
        report_status: bool = False,
        report_status_interval: float = 30.0,
        report_status_version: Optional[str] = None,
        report_status_commit_sha: Optional[str] = None,
        report_status_deploy_slot: Optional[str] = None,
        report_status_tags: Optional[Dict[str, str]] = None,
        report_status_health_provider: Optional[Callable[[], HealthReport]] = None,
    ) -> None:
        self._api_key = api_key
        self._base_url = base_url.rstrip("/")
        self._environment = environment
        self._project = project
        self._offline_mode = offline_mode
        self._session_id = session_id

        self._cache = TTLCache(default_ttl=cache_timeout)
        self._flags: Dict[str, Flag] = {}
        self._registry: dict[str, list[dict]] = {}
        self._http: Optional[httpx.Client] = None
        self._sse: Optional[SSEClient] = None
        self._initialized = False

        self._application_id = application_id
        self._report_status = report_status
        self._report_status_interval = report_status_interval
        self._report_status_version = report_status_version
        self._report_status_commit_sha = report_status_commit_sha
        self._report_status_deploy_slot = report_status_deploy_slot
        self._report_status_tags = report_status_tags
        self._report_status_health_provider = report_status_health_provider
        self._status_reporter: Optional[StatusReporter] = None

    # ------------------------------------------------------------------
    # Headers
    # ------------------------------------------------------------------

    def _auth_headers(self) -> Dict[str, str]:
        headers = {
            "Authorization": f"ApiKey {self._api_key}",
            "Content-Type": "application/json",
        }
        if self._session_id:
            headers["X-DeploySentry-Session"] = self._session_id
        return headers

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def initialize(self) -> None:
        """Fetch the initial flag set and start the SSE stream.

        This must be called before evaluating any flags (unless the client is
        in *offline_mode*).
        """
        if self._offline_mode:
            self._initialized = True
            return

        self._http = httpx.Client(
            base_url=self._base_url,
            headers=self._auth_headers(),
            timeout=10.0,
        )

        # Bulk-fetch all flags for this project.
        self._fetch_flags()

        # Start SSE streaming for real-time updates.
        stream_url = f"{self._base_url}/api/v1/flags/stream"
        params: Dict[str, str] = {}
        if self._project:
            params["project_id"] = self._project
        if self._environment:
            params["environment"] = self._environment
        self._sse = SSEClient(
            url=stream_url,
            headers=self._auth_headers(),
            on_update=self._handle_stream_update,
            params=params,
        )
        self._sse.start()
        self._initialized = True
        logger.info("DeploySentryClient initialised (project=%s, env=%s)", self._project, self._environment)

        self._start_status_reporter()

    def _start_status_reporter(self) -> None:
        if not self._report_status:
            return
        if not self._application_id:
            logger.warning(
                "report_status=True but application_id is empty; status reporter disabled"
            )
            return
        self._status_reporter = StatusReporter(
            base_url=self._base_url,
            api_key=self._api_key,
            application_id=self._application_id,
            interval_s=self._report_status_interval,
            version=self._report_status_version,
            commit_sha=self._report_status_commit_sha,
            deploy_slot=self._report_status_deploy_slot,
            tags=self._report_status_tags,
            health_provider=self._report_status_health_provider,
        )
        self._status_reporter.start()

    def close(self) -> None:
        """Release resources held by the client."""
        if self._status_reporter is not None:
            self._status_reporter.stop()
            self._status_reporter = None
        if self._sse is not None:
            self._sse.stop()
            self._sse = None
        if self._http is not None:
            self._http.close()
            self._http = None
        self._cache.clear()
        self._initialized = False

    def __enter__(self) -> "DeploySentryClient":
        self.initialize()
        return self

    def __exit__(self, *exc: Any) -> None:
        self.close()

    # ------------------------------------------------------------------
    # Flag fetching
    # ------------------------------------------------------------------

    def _fetch_flags(self) -> None:
        assert self._http is not None
        params: Dict[str, str] = {}
        if self._project:
            params["project_id"] = self._project
        if self._environment:
            params["environment"] = self._environment
        resp = self._http.get("/api/v1/flags", params=params)
        resp.raise_for_status()
        data = resp.json()
        flags_list = data if isinstance(data, list) else data.get("flags", [])
        for raw in flags_list:
            flag = Flag.from_dict(raw)
            self._flags[flag.key] = flag

    # ------------------------------------------------------------------
    # Evaluation helpers
    # ------------------------------------------------------------------

    def _evaluate(self, key: str, context: Optional[EvaluationContext] = None) -> Optional[EvaluationResult]:
        """Evaluate a single flag via the API, using the cache where possible."""
        cached = self._cache.get(f"eval:{key}")
        if cached is not None:
            return cached

        if self._offline_mode or self._http is None:
            return None

        body: Dict[str, Any] = {
            "flag_key": key,
            "environment": self._environment,
        }
        if self._project:
            body["project_id"] = self._project
        if self._session_id:
            body["session_id"] = self._session_id
        if context is not None:
            body["context"] = context.to_dict()

        try:
            resp = self._http.post("/api/v1/flags/evaluate", json=body)
            resp.raise_for_status()
            result = EvaluationResult.from_dict(resp.json())
            self._cache.set(f"eval:{key}", result)
            return result
        except httpx.HTTPError:
            logger.warning("Flag evaluation failed for key=%s", key, exc_info=True)
            return None

    # ------------------------------------------------------------------
    # Public evaluation API
    # ------------------------------------------------------------------

    def bool_value(self, key: str, default: bool = False, context: Optional[EvaluationContext] = None) -> bool:
        """Evaluate a boolean flag."""
        result = self._evaluate(key, context)
        if result is None:
            return default
        if isinstance(result.value, bool):
            return result.value
        return result.enabled

    def string_value(self, key: str, default: str = "", context: Optional[EvaluationContext] = None) -> str:
        """Evaluate a string flag."""
        result = self._evaluate(key, context)
        if result is None:
            return default
        if isinstance(result.value, str):
            return result.value
        return default

    def int_value(self, key: str, default: int = 0, context: Optional[EvaluationContext] = None) -> int:
        """Evaluate an integer flag."""
        result = self._evaluate(key, context)
        if result is None:
            return default
        try:
            return int(result.value)
        except (TypeError, ValueError):
            return default

    def json_value(self, key: str, default: Any = None, context: Optional[EvaluationContext] = None) -> Any:
        """Evaluate a flag that holds a JSON value (dict, list, etc.)."""
        result = self._evaluate(key, context)
        if result is None:
            return default
        if result.value is None:
            return default
        return result.value

    def detail(self, key: str, context: Optional[EvaluationContext] = None) -> EvaluationResult:
        """Return the full ``EvaluationResult`` for a flag, including metadata."""
        result = self._evaluate(key, context)
        if result is not None:
            return result
        # Fallback: build a result from the local flag store.
        flag = self._flags.get(key)
        if flag is not None:
            return EvaluationResult(
                key=flag.key,
                enabled=flag.enabled,
                value=flag.value,
                reason="local",
                metadata=flag.metadata,
            )
        return EvaluationResult(key=key, enabled=False, value=None, reason="not_found")

    # ------------------------------------------------------------------
    # Metadata queries
    # ------------------------------------------------------------------

    def flags_by_category(self, category: FlagCategory) -> List[Flag]:
        """Return all flags that belong to the given *category*."""
        return [f for f in self._flags.values() if f.metadata.category == category]

    def expired_flags(self) -> List[Flag]:
        """Return flags whose ``expires_at`` is in the past."""
        now = datetime.now(timezone.utc)
        results: List[Flag] = []
        for flag in self._flags.values():
            if flag.metadata.expires_at is not None and flag.metadata.expires_at < now:
                results.append(flag)
        return results

    def flag_owners(self, key: str) -> List[str]:
        """Return the owners list for the flag identified by *key*."""
        flag = self._flags.get(key)
        if flag is None:
            return []
        return list(flag.metadata.owners)

    # ------------------------------------------------------------------
    # SSE handler
    # ------------------------------------------------------------------

    def _fetch_single_flag(self, flag_id: str) -> Optional[Flag]:
        """Fetch a single flag by ID from the API."""
        if self._http is None:
            return None
        params: Dict[str, str] = {}
        if self._environment:
            params["environment"] = self._environment
        try:
            resp = self._http.get(f"/api/v1/flags/{flag_id}", params=params)
            resp.raise_for_status()
            return Flag.from_dict(resp.json())
        except httpx.HTTPError:
            logger.warning("Failed to fetch flag id=%s", flag_id, exc_info=True)
            return None

    def _handle_stream_update(self, payload: Dict[str, Any]) -> None:
        """Process a real-time flag update from the SSE stream.

        The SSE notification contains event metadata (event, flag_id,
        flag_key, timestamp) — not a full Flag object.  We extract the
        flag_id, fetch the complete flag from the API, and update the
        local cache.
        """
        flag_id = payload.get("flag_id")
        flag_key = payload.get("flag_key")
        if not flag_id:
            logger.debug("SSE event missing flag_id: %s", payload)
            return

        flag = self._fetch_single_flag(flag_id)
        if flag is not None:
            self._flags[flag.key] = flag
            self._cache.delete(f"eval:{flag.key}")
            logger.debug("Flag updated via SSE: %s", flag.key)
        elif flag_key:
            # Could not fetch; at minimum invalidate stale cache entry.
            self._cache.delete(f"eval:{flag_key}")
            logger.debug("Invalidated cache for flag_key=%s (fetch failed)", flag_key)

    # ------------------------------------------------------------------
    # Register / dispatch
    # ------------------------------------------------------------------

    def register(self, operation: str, handler: Callable, flag_key: str | None = None) -> None:
        """Register a handler for an operation, optionally gated by a flag."""
        lst = self._registry.setdefault(operation, [])
        if flag_key is None:
            for i, reg in enumerate(lst):
                if reg["flag_key"] is None:
                    lst[i] = {"handler": handler, "flag_key": None}
                    return
            lst.append({"handler": handler, "flag_key": None})
        else:
            lst.append({"handler": handler, "flag_key": flag_key})

    def dispatch(self, operation: str, context: EvaluationContext | None = None) -> Callable:
        """Evaluate flags and return the matching handler for the operation."""
        lst = self._registry.get(operation)
        if not lst:
            raise RuntimeError(
                f"No handlers registered for operation '{operation}'. "
                "Call register() before dispatch()."
            )
        for reg in lst:
            if reg["flag_key"] is not None:
                flag = self._flags.get(reg["flag_key"])
                if flag and flag.enabled:
                    return reg["handler"]
        for reg in lst:
            if reg["flag_key"] is None:
                return reg["handler"]
        raise RuntimeError(
            f"No matching handler for operation '{operation}' and no default registered. "
            "Register a default handler (no flag_key) as the last registration."
        )

    # ------------------------------------------------------------------
    # Session helpers
    # ------------------------------------------------------------------

    def refresh_session(self) -> None:
        """Clear the local cache and flag store, then re-fetch all flags.

        Use this after changing session context to ensure evaluations reflect
        the latest server state.
        """
        self._cache.clear()
        self._flags.clear()
        if not self._offline_mode and self._http is not None:
            self._fetch_flags()

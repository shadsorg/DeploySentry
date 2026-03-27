"""Synchronous DeploySentry client."""

from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import httpx

from .cache import TTLCache
from .models import (
    EvaluationContext,
    EvaluationResult,
    Flag,
    FlagCategory,
    FlagMetadata,
)
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
    """

    def __init__(
        self,
        api_key: str,
        base_url: str = "https://api.deploysentry.io",
        environment: str = "production",
        project: str = "",
        cache_timeout: float = 30,
        offline_mode: bool = False,
    ) -> None:
        self._api_key = api_key
        self._base_url = base_url.rstrip("/")
        self._environment = environment
        self._project = project
        self._offline_mode = offline_mode

        self._cache = TTLCache(default_ttl=cache_timeout)
        self._flags: Dict[str, Flag] = {}
        self._http: Optional[httpx.Client] = None
        self._sse: Optional[SSEClient] = None
        self._initialized = False

    # ------------------------------------------------------------------
    # Headers
    # ------------------------------------------------------------------

    def _auth_headers(self) -> Dict[str, str]:
        return {
            "Authorization": f"ApiKey {self._api_key}",
            "Content-Type": "application/json",
        }

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

    def close(self) -> None:
        """Release resources held by the client."""
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

    def _handle_stream_update(self, payload: Dict[str, Any]) -> None:
        """Process a real-time flag update from the SSE stream."""
        key = payload.get("key") or payload.get("flag_key")
        if not key:
            return
        flag = Flag.from_dict(payload)
        self._flags[flag.key] = flag
        # Invalidate the evaluation cache for this key.
        self._cache.delete(f"eval:{flag.key}")
        logger.debug("Flag updated via SSE: %s", flag.key)

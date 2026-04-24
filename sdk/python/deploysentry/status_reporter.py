"""Agentless status reporting for the DeploySentry Python SDK.

The :class:`StatusReporter` posts periodic ``POST /applications/:id/status``
updates to DeploySentry on behalf of a client. Failures are swallowed
(logged at WARNING) so flag evaluation is never blocked by reporting.
"""

from __future__ import annotations

import logging
import os
import threading
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, Optional

import httpx

logger = logging.getLogger("deploysentry.status_reporter")


_VERSION_ENV_CHAIN = (
    "APP_VERSION",
    "GIT_SHA",
    "GIT_COMMIT",
    "SOURCE_COMMIT",
    "RAILWAY_GIT_COMMIT_SHA",
    "RENDER_GIT_COMMIT",
    "VERCEL_GIT_COMMIT_SHA",
    "HEROKU_SLUG_COMMIT",
)

_MIN_BACKOFF_S = 1.0
_MAX_BACKOFF_S = 5 * 60.0


@dataclass
class HealthReport:
    """Shape returned by a user-supplied health provider."""

    state: str = "healthy"  # healthy | degraded | unhealthy | unknown
    score: Optional[float] = None
    reason: Optional[str] = None


def resolve_version(explicit: Optional[str] = None, package_name: Optional[str] = None) -> str:
    """Pick the reported version.

    Priority: explicit override → env vars → ``importlib.metadata`` for the
    consuming package (if supplied) → ``"unknown"``.
    """
    if explicit:
        return explicit
    for name in _VERSION_ENV_CHAIN:
        val = os.environ.get(name)
        if val:
            return val
    if package_name:
        try:
            from importlib.metadata import PackageNotFoundError, version

            return version(package_name)
        except PackageNotFoundError:
            pass
        except Exception:  # pragma: no cover — defensive
            pass
    return "unknown"


@dataclass
class StatusReporter:
    """Periodically reports app status to DeploySentry."""

    base_url: str
    api_key: str
    application_id: str
    interval_s: float = 30.0
    version: Optional[str] = None
    commit_sha: Optional[str] = None
    deploy_slot: Optional[str] = None
    tags: Optional[Dict[str, str]] = None
    health_provider: Optional[Callable[[], HealthReport]] = None
    http_client: Optional[httpx.Client] = None
    package_name: Optional[str] = None

    _timer: Optional[threading.Timer] = field(default=None, init=False, repr=False)
    _stopped: threading.Event = field(default_factory=threading.Event, init=False, repr=False)
    _backoff: float = field(default=0.0, init=False, repr=False)

    def __post_init__(self) -> None:
        if self.interval_s < 0:
            raise ValueError("interval_s must be >= 0")
        self.base_url = self.base_url.rstrip("/")

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def start(self) -> None:
        """Fire one report immediately and (if interval > 0) schedule repeats."""
        self._stopped.clear()
        self._tick()

    def stop(self) -> None:
        self._stopped.set()
        if self._timer is not None:
            self._timer.cancel()
            self._timer = None

    def report_once(self) -> None:
        """Send exactly one status report. Raises on HTTP errors."""
        version = resolve_version(self.version, self.package_name)
        health = HealthReport(state="healthy")
        if self.health_provider is not None:
            try:
                health = self.health_provider()
            except Exception as err:
                health = HealthReport(state="unknown", reason=str(err))

        body: Dict[str, Any] = {"version": version, "health": health.state}
        if health.score is not None:
            body["health_score"] = health.score
        if health.reason:
            body["health_reason"] = health.reason
        if self.commit_sha:
            body["commit_sha"] = self.commit_sha
        if self.deploy_slot:
            body["deploy_slot"] = self.deploy_slot
        if self.tags:
            body["tags"] = self.tags

        url = f"{self.base_url}/api/v1/applications/{self.application_id}/status"
        headers = {
            "Authorization": f"ApiKey {self.api_key}",
            "Content-Type": "application/json",
        }
        client = self.http_client or httpx.Client(timeout=10.0)
        try:
            resp = client.post(url, json=body, headers=headers)
            if resp.status_code >= 300 or resp.status_code < 200:
                raise RuntimeError(
                    f"status report failed: {resp.status_code} {resp.reason_phrase}"
                )
        finally:
            if self.http_client is None:
                client.close()

    # ------------------------------------------------------------------
    # Internals
    # ------------------------------------------------------------------

    def _tick(self) -> None:
        if self._stopped.is_set():
            return
        try:
            self.report_once()
            self._backoff = 0.0
        except Exception as err:
            logger.warning("status report error: %s", err)
            if self._backoff >= _MAX_BACKOFF_S:
                # Clamped at max — reset so the next scheduled probe is
                # interval_s, not another 5 min. Otherwise a server that
                # recovers mid-outage is noticed up to _MAX_BACKOFF_S late
                # regardless of how tight interval_s is configured. On the
                # next failure the 1s ladder restarts.
                self._backoff = 0.0
                logger.warning(
                    "status reporter backoff reset; probing every %ss",
                    self.interval_s,
                )
            else:
                self._backoff = max(
                    _MIN_BACKOFF_S,
                    min(self._backoff * 2 or _MIN_BACKOFF_S, _MAX_BACKOFF_S),
                )
        if self._stopped.is_set() or self.interval_s == 0:
            return
        delay = self._backoff or self.interval_s
        self._timer = threading.Timer(delay, self._tick)
        self._timer.daemon = True
        self._timer.start()

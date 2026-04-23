"""Tests for the agentless status reporter."""

from __future__ import annotations

import os
from typing import Any, Dict
from unittest import mock

import httpx

from deploysentry.status_reporter import (
    HealthReport,
    StatusReporter,
    resolve_version,
)


def _mock_transport(captured: Dict[str, Any]):
    """Return an httpx MockTransport that records the last request/response."""

    def handler(request: httpx.Request) -> httpx.Response:
        captured["method"] = request.method
        captured["url"] = str(request.url)
        captured["auth"] = request.headers.get("Authorization")
        captured["body"] = request.content.decode()
        return httpx.Response(201)

    return httpx.MockTransport(handler)


def test_resolve_version_prefers_explicit(monkeypatch):
    monkeypatch.setenv("APP_VERSION", "from-env")
    assert resolve_version("1.2.3") == "1.2.3"


def test_resolve_version_falls_through_env_chain(monkeypatch):
    for name in ("APP_VERSION", "GIT_SHA", "GIT_COMMIT", "SOURCE_COMMIT",
                 "RAILWAY_GIT_COMMIT_SHA", "RENDER_GIT_COMMIT",
                 "VERCEL_GIT_COMMIT_SHA", "HEROKU_SLUG_COMMIT"):
        monkeypatch.delenv(name, raising=False)
    monkeypatch.setenv("GIT_SHA", "abc123")
    assert resolve_version() == "abc123"


def test_resolve_version_unknown_fallback(monkeypatch):
    for name in ("APP_VERSION", "GIT_SHA", "GIT_COMMIT", "SOURCE_COMMIT",
                 "RAILWAY_GIT_COMMIT_SHA", "RENDER_GIT_COMMIT",
                 "VERCEL_GIT_COMMIT_SHA", "HEROKU_SLUG_COMMIT"):
        monkeypatch.delenv(name, raising=False)
    assert resolve_version() == "unknown"


def test_report_once_posts_to_correct_url():
    captured: Dict[str, Any] = {}
    client = httpx.Client(transport=_mock_transport(captured))
    reporter = StatusReporter(
        base_url="https://api.example.com/",
        api_key="ds_test",
        application_id="f47ac10b-58cc-4372-a567-0e02b2c3d479",
        interval_s=0,
        version="1.4.2",
        commit_sha="abc123",
        deploy_slot="canary",
        tags={"region": "us-east"},
        http_client=client,
    )

    reporter.report_once()

    assert captured["method"] == "POST"
    assert captured["url"] == (
        "https://api.example.com/api/v1/applications/f47ac10b-58cc-4372-a567-0e02b2c3d479/status"
    )
    assert captured["auth"] == "ApiKey ds_test"
    import json
    body = json.loads(captured["body"])
    assert body["version"] == "1.4.2"
    assert body["health"] == "healthy"
    assert body["commit_sha"] == "abc123"
    assert body["deploy_slot"] == "canary"
    assert body["tags"] == {"region": "us-east"}


def test_report_once_uses_health_provider():
    captured: Dict[str, Any] = {}
    client = httpx.Client(transport=_mock_transport(captured))
    reporter = StatusReporter(
        base_url="http://x",
        api_key="k",
        application_id="a",
        interval_s=0,
        version="1",
        health_provider=lambda: HealthReport(state="degraded", score=0.8, reason="db slow"),
        http_client=client,
    )
    reporter.report_once()

    import json
    body = json.loads(captured["body"])
    assert body["health"] == "degraded"
    assert body["health_score"] == 0.8
    assert body["health_reason"] == "db slow"


def test_report_once_health_provider_error_is_unknown():
    captured: Dict[str, Any] = {}
    client = httpx.Client(transport=_mock_transport(captured))

    def boom() -> HealthReport:
        raise RuntimeError("oops")

    reporter = StatusReporter(
        base_url="http://x",
        api_key="k",
        application_id="a",
        interval_s=0,
        version="1",
        health_provider=boom,
        http_client=client,
    )
    reporter.report_once()

    import json
    body = json.loads(captured["body"])
    assert body["health"] == "unknown"
    assert "oops" in body["health_reason"]


def test_report_once_raises_on_non_2xx():
    def handler(_req: httpx.Request) -> httpx.Response:
        return httpx.Response(500, text="internal error")

    client = httpx.Client(transport=httpx.MockTransport(handler))
    reporter = StatusReporter(
        base_url="http://x",
        api_key="k",
        application_id="a",
        interval_s=0,
        version="1",
        http_client=client,
    )
    import pytest

    with pytest.raises(RuntimeError):
        reporter.report_once()


def test_negative_interval_rejected():
    import pytest

    with pytest.raises(ValueError):
        StatusReporter(
            base_url="http://x",
            api_key="k",
            application_id="a",
            interval_s=-1,
        )

"""Server-Sent Events (SSE) streaming client for real-time flag updates."""

from __future__ import annotations

import asyncio
import json
import logging
import random
import threading
import time
from typing import Any, Callable, Dict, Optional

import httpx

logger = logging.getLogger("deploysentry.streaming")

# Reconnection parameters
_INITIAL_RETRY_MS = 1000
_MAX_RETRY_MS = 30000
_RETRY_MULTIPLIER = 2


def _apply_jitter(delay_ms: float) -> float:
    """Apply +/- 20% jitter to a delay value."""
    jitter = delay_ms * 0.2 * (2 * random.random() - 1)
    return delay_ms + jitter


class SSEClient:
    """Synchronous SSE client that runs in a background thread.

    Connects to the DeploySentry flag-stream endpoint, parses SSE frames, and
    invokes *on_update* for every ``data:`` message received.
    """

    def __init__(
        self,
        url: str,
        headers: Dict[str, str],
        on_update: Callable[[Dict[str, Any]], None],
        params: Optional[Dict[str, str]] = None,
    ) -> None:
        self._url = url
        self._headers = headers
        self._on_update = on_update
        self._params = params or {}
        self._stop_event = threading.Event()
        self._thread: Optional[threading.Thread] = None

    def start(self) -> None:
        """Start the SSE listener in a daemon thread."""
        if self._thread is not None and self._thread.is_alive():
            return
        self._stop_event.clear()
        self._thread = threading.Thread(target=self._run, daemon=True, name="deploysentry-sse")
        self._thread.start()

    def stop(self) -> None:
        """Signal the listener to stop and wait for thread termination."""
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=5)
            self._thread = None

    @property
    def is_running(self) -> bool:
        return self._thread is not None and self._thread.is_alive()

    # ------------------------------------------------------------------
    # Internal
    # ------------------------------------------------------------------

    def _run(self) -> None:
        retry_ms = _INITIAL_RETRY_MS
        while not self._stop_event.is_set():
            try:
                self._connect_and_listen()
                retry_ms = _INITIAL_RETRY_MS  # reset on clean disconnect
            except Exception:
                jittered = _apply_jitter(retry_ms)
                logger.warning(
                    "SSE connection lost, retrying in %.0f ms", jittered, exc_info=True
                )
                if self._stop_event.wait(timeout=jittered / 1000):
                    break
                retry_ms = min(retry_ms * _RETRY_MULTIPLIER, _MAX_RETRY_MS)

    def _connect_and_listen(self) -> None:
        sse_headers = {**self._headers, "Accept": "text/event-stream"}
        with httpx.Client(timeout=None) as http:
            with http.stream("GET", self._url, headers=sse_headers, params=self._params) as resp:
                resp.raise_for_status()
                buffer = ""
                for chunk in resp.iter_text():
                    if self._stop_event.is_set():
                        return
                    buffer += chunk
                    while "\n\n" in buffer:
                        frame, buffer = buffer.split("\n\n", 1)
                        self._process_frame(frame)

    def _process_frame(self, frame: str) -> None:
        event_type = ""
        data_lines: list[str] = []
        for line in frame.split("\n"):
            if line.startswith("event:"):
                event_type = line[len("event:"):].strip()
            elif line.startswith("data:"):
                data_lines.append(line[len("data:"):].strip())
            elif line.startswith("retry:"):
                # Server-sent retry hint; ignored for simplicity.
                pass
        if not data_lines:
            return
        raw = "\n".join(data_lines)
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            logger.debug("Non-JSON SSE data: %s", raw)
            return
        self._on_update(payload)


class AsyncSSEClient:
    """Asynchronous SSE client backed by ``asyncio``."""

    def __init__(
        self,
        url: str,
        headers: Dict[str, str],
        on_update: Callable[[Dict[str, Any]], None],
        params: Optional[Dict[str, str]] = None,
    ) -> None:
        self._url = url
        self._headers = headers
        self._on_update = on_update
        self._params = params or {}
        self._task: Optional[asyncio.Task[None]] = None

    async def start(self) -> None:
        """Start the SSE listener as an asyncio task."""
        if self._task is not None and not self._task.done():
            return
        self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        """Cancel the listener task and await its completion."""
        if self._task is not None:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
            self._task = None

    @property
    def is_running(self) -> bool:
        return self._task is not None and not self._task.done()

    # ------------------------------------------------------------------
    # Internal
    # ------------------------------------------------------------------

    async def _run(self) -> None:
        retry_ms = _INITIAL_RETRY_MS
        while True:
            try:
                await self._connect_and_listen()
                retry_ms = _INITIAL_RETRY_MS
            except asyncio.CancelledError:
                raise
            except Exception:
                jittered = _apply_jitter(retry_ms)
                logger.warning(
                    "SSE connection lost, retrying in %.0f ms", jittered, exc_info=True
                )
                await asyncio.sleep(jittered / 1000)
                retry_ms = min(retry_ms * _RETRY_MULTIPLIER, _MAX_RETRY_MS)

    async def _connect_and_listen(self) -> None:
        sse_headers = {**self._headers, "Accept": "text/event-stream"}
        async with httpx.AsyncClient(timeout=None) as http:
            async with http.stream(
                "GET", self._url, headers=sse_headers, params=self._params
            ) as resp:
                resp.raise_for_status()
                buffer = ""
                async for chunk in resp.aiter_text():
                    buffer += chunk
                    while "\n\n" in buffer:
                        frame, buffer = buffer.split("\n\n", 1)
                        self._process_frame(frame)

    def _process_frame(self, frame: str) -> None:
        event_type = ""
        data_lines: list[str] = []
        for line in frame.split("\n"):
            if line.startswith("event:"):
                event_type = line[len("event:"):].strip()
            elif line.startswith("data:"):
                data_lines.append(line[len("data:"):].strip())
        if not data_lines:
            return
        raw = "\n".join(data_lines)
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            logger.debug("Non-JSON SSE data: %s", raw)
            return
        self._on_update(payload)

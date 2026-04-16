"""Unit tests for the DeploySentryClient class."""

import unittest

from deploysentry.client import DeploySentryClient


class TestClientInit(unittest.TestCase):
    """Tests for client construction and defaults."""

    def test_client_init_defaults(self):
        client = DeploySentryClient(api_key="ds_test_key123")

        self.assertEqual(client._api_key, "ds_test_key123")
        self.assertEqual(client._base_url, "https://api.dr-sentry.com")
        self.assertEqual(client._environment, "production")
        self.assertEqual(client._project, "")
        self.assertFalse(client._offline_mode)
        self.assertIsNone(client._session_id)
        self.assertFalse(client._initialized)

    def test_client_init_custom(self):
        client = DeploySentryClient(
            api_key="ds_test_key",
            base_url="http://localhost:8080",
            environment="staging",
            project="my-project",
            cache_timeout=60,
            offline_mode=True,
            session_id="sess-abc",
        )

        self.assertEqual(client._base_url, "http://localhost:8080")
        self.assertEqual(client._environment, "staging")
        self.assertEqual(client._project, "my-project")
        self.assertTrue(client._offline_mode)
        self.assertEqual(client._session_id, "sess-abc")

    def test_client_strips_trailing_slash(self):
        client = DeploySentryClient(api_key="k", base_url="http://example.com/")
        self.assertEqual(client._base_url, "http://example.com")


class TestClientAuthHeaders(unittest.TestCase):
    """Tests for _auth_headers method."""

    def test_auth_header_format(self):
        client = DeploySentryClient(api_key="ds_live_abc123")
        headers = client._auth_headers()

        self.assertEqual(headers["Authorization"], "ApiKey ds_live_abc123")
        self.assertTrue(headers["Authorization"].startswith("ApiKey "))
        self.assertEqual(headers["Content-Type"], "application/json")

    def test_client_session_id(self):
        client = DeploySentryClient(api_key="k", session_id="sess-xyz")
        headers = client._auth_headers()

        self.assertIn("X-DeploySentry-Session", headers)
        self.assertEqual(headers["X-DeploySentry-Session"], "sess-xyz")

    def test_client_no_session_id(self):
        client = DeploySentryClient(api_key="k")
        headers = client._auth_headers()

        self.assertNotIn("X-DeploySentry-Session", headers)


class TestClientOfflineMode(unittest.TestCase):
    """Tests for offline mode evaluation behaviour."""

    def setUp(self):
        self.client = DeploySentryClient(
            api_key="ds_test_key",
            offline_mode=True,
        )
        self.client.initialize()

    def test_bool_value_default_in_offline_mode(self):
        result = self.client.bool_value("nonexistent-flag", default=True)
        self.assertTrue(result)

        result = self.client.bool_value("nonexistent-flag", default=False)
        self.assertFalse(result)

    def test_string_value_default_in_offline_mode(self):
        result = self.client.string_value("nonexistent-flag", default="fallback")
        self.assertEqual(result, "fallback")

        result = self.client.string_value("nonexistent-flag")
        self.assertEqual(result, "")

    def test_int_value_default_in_offline_mode(self):
        result = self.client.int_value("nonexistent-flag", default=42)
        self.assertEqual(result, 42)

    def test_json_value_default_in_offline_mode(self):
        default_val = {"key": "value"}
        result = self.client.json_value("nonexistent-flag", default=default_val)
        self.assertEqual(result, default_val)

    def tearDown(self):
        self.client.close()


if __name__ == "__main__":
    unittest.main()


class TestRegisterDispatch:
    def setup_method(self):
        self.client = DeploySentryClient(
            api_key="test-key",
            base_url="http://localhost:8080",
            environment="test",
            project="test-project",
        )

    def _seed_flag(self, key, enabled):
        from deploysentry.models import Flag, FlagMetadata
        flag = Flag(key=key, enabled=enabled, value=str(enabled), metadata=FlagMetadata(category="feature"))
        self.client._flags[key] = flag

    def test_dispatches_flagged_handler_when_on(self):
        self.client.register("op", lambda: "feature", flag_key="my-flag")
        self.client.register("op", lambda: "default")
        self._seed_flag("my-flag", True)
        assert self.client.dispatch("op")() == "feature"

    def test_dispatches_default_when_flag_off(self):
        self.client.register("op", lambda: "feature", flag_key="my-flag")
        self.client.register("op", lambda: "default")
        self._seed_flag("my-flag", False)
        assert self.client.dispatch("op")() == "default"

    def test_first_match_wins(self):
        self.client.register("op", lambda: "first", flag_key="flag-a")
        self.client.register("op", lambda: "second", flag_key="flag-b")
        self.client.register("op", lambda: "default")
        self._seed_flag("flag-a", True)
        self._seed_flag("flag-b", True)
        assert self.client.dispatch("op")() == "first"

    def test_default_only(self):
        self.client.register("op", lambda: "default")
        assert self.client.dispatch("op")() == "default"

    def test_operations_isolated(self):
        self.client.register("cart", lambda: "cart")
        self.client.register("pay", lambda: "pay")
        assert self.client.dispatch("cart")() == "cart"
        assert self.client.dispatch("pay")() == "pay"

    def test_throws_on_unregistered(self):
        import pytest
        with pytest.raises(RuntimeError, match="No handlers registered"):
            self.client.dispatch("unknown")

    def test_throws_no_match_no_default(self):
        import pytest
        self.client.register("op", lambda: "feat", flag_key="my-flag")
        self._seed_flag("my-flag", False)
        with pytest.raises(RuntimeError, match="no default registered"):
            self.client.dispatch("op")

    def test_replaces_default(self):
        self.client.register("op", lambda: "first")
        self.client.register("op", lambda: "second")
        assert self.client.dispatch("op")() == "second"

    def test_passes_caller_args(self):
        self.client.register("add", lambda a, b: a + b)
        assert self.client.dispatch("add")(3, 4) == 7

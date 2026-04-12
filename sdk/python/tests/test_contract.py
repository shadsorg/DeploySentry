"""Contract tests that validate SDK behaviour against shared testdata fixtures."""

import json
import os
import unittest

from deploysentry.models import EvaluationResult, Flag

TESTDATA_DIR = os.path.join(os.path.dirname(__file__), "..", "..", "testdata")


def _load_fixture(name: str) -> dict:
    path = os.path.join(TESTDATA_DIR, name)
    with open(path, "r") as f:
        return json.load(f)


class TestAuthContract(unittest.TestCase):
    """Verify the auth header prefix matches the contract."""

    def test_auth_header_prefix(self):
        fixture = _load_fixture("auth_request.json")
        self.assertEqual(fixture["header_value_prefix"], "ApiKey ")

    def test_auth_header_name(self):
        fixture = _load_fixture("auth_request.json")
        self.assertEqual(fixture["header_name"], "Authorization")


class TestEvaluateResponseContract(unittest.TestCase):
    """Verify evaluate_response.json can be parsed into an EvaluationResult."""

    def test_parse_evaluate_response(self):
        fixture = _load_fixture("evaluate_response.json")
        body = fixture["body"]

        # The fixture uses "flag_key" while from_dict expects "key".
        # Normalise the key name as the real API client does.
        normalized = dict(body)
        if "flag_key" in normalized and "key" not in normalized:
            normalized["key"] = normalized.pop("flag_key")

        result = EvaluationResult.from_dict(normalized)

        self.assertEqual(result.key, "dark-mode")
        self.assertTrue(result.enabled)
        self.assertTrue(result.value)
        self.assertEqual(result.reason, "TARGETING_MATCH")
        self.assertEqual(result.metadata.category.value, "feature")
        self.assertEqual(result.metadata.owners, ["frontend-team"])

    def test_evaluate_response_has_valid_reasons(self):
        fixture = _load_fixture("evaluate_response.json")
        reasons = fixture["valid_reasons"]
        self.assertIn("TARGETING_MATCH", reasons)
        self.assertIn("DEFAULT_VALUE", reasons)
        self.assertIn("FLAG_DISABLED", reasons)


class TestListFlagsResponseContract(unittest.TestCase):
    """Verify list_flags_response.json has the expected number of flags."""

    def test_list_flags_has_3_flags(self):
        fixture = _load_fixture("list_flags_response.json")
        flags_raw = fixture["body"]["flags"]

        self.assertEqual(len(flags_raw), 3)

        flags = [Flag.from_dict(f) for f in flags_raw]
        keys = [f.key for f in flags]
        self.assertIn("dark-mode", keys)
        self.assertIn("new-checkout", keys)
        self.assertIn("max-items", keys)


class TestBatchEvaluateResponseContract(unittest.TestCase):
    """Verify batch_evaluate_response.json has 3 results."""

    def test_batch_evaluate_has_3_results(self):
        fixture = _load_fixture("batch_evaluate_response.json")
        results_raw = fixture["body"]["results"]

        self.assertEqual(len(results_raw), 3)

        results = [EvaluationResult.from_dict(r) for r in results_raw]
        self.assertEqual(len(results), 3)


if __name__ == "__main__":
    unittest.main()

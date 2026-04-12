# frozen_string_literal: true

require "minitest/autorun"
require "json"

class ContractTest < Minitest::Test
  TESTDATA_DIR = File.expand_path("../../testdata", __dir__)

  def setup
    @auth_request = JSON.parse(File.read(File.join(TESTDATA_DIR, "auth_request.json")))
    @evaluate_response = JSON.parse(File.read(File.join(TESTDATA_DIR, "evaluate_response.json")))
    @list_flags_response = JSON.parse(File.read(File.join(TESTDATA_DIR, "list_flags_response.json")))
  end

  def test_auth_header_prefix_is_apikey
    prefix = @auth_request["header_value_prefix"]
    assert_equal "ApiKey ", prefix, "Auth header prefix must be 'ApiKey '"
  end

  def test_evaluate_response_parsing
    body = @evaluate_response["body"]

    assert_equal "dark-mode", body["flag_key"]
    assert_equal true, body["value"]
    assert_equal true, body["enabled"]
    assert_equal "TARGETING_MATCH", body["reason"]
    assert_equal "boolean", body["flag_type"]

    metadata = body["metadata"]
    refute_nil metadata
    assert_equal "feature", metadata["category"]
    assert_includes metadata["owners"], "frontend-team"
    assert_equal false, metadata["is_permanent"]
    assert_includes metadata["tags"], "ui"
  end

  def test_list_flags_response_has_3_flags
    flags = @list_flags_response["body"]["flags"]
    assert_equal 3, flags.length, "list_flags_response must contain exactly 3 flags"
  end
end

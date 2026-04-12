# frozen_string_literal: true

require "minitest/autorun"
require "deploysentry"

class ClientTest < Minitest::Test
  VALID_OPTS = {
    api_key: "ds_test_key123",
    base_url: "http://localhost:8080",
    environment: "staging",
    project: "my-project"
  }.freeze

  def test_requires_api_key
    assert_raises(DeploySentry::ConfigurationError) do
      DeploySentry::Client.new(**VALID_OPTS.merge(api_key: ""))
    end
  end

  def test_auth_header_format
    client = DeploySentry::Client.new(**VALID_OPTS)
    headers = client.send(:auth_headers)

    assert_equal "ApiKey ds_test_key123", headers["Authorization"]
    assert headers["Authorization"].start_with?("ApiKey "), "Authorization header must start with 'ApiKey '"
  end

  def test_session_header_included_when_set
    client = DeploySentry::Client.new(**VALID_OPTS, session_id: "sess-abc-123")
    headers = client.send(:auth_headers)

    assert_equal "sess-abc-123", headers["X-DeploySentry-Session"]
  end

  def test_session_header_absent_when_not_set
    client = DeploySentry::Client.new(**VALID_OPTS)
    headers = client.send(:auth_headers)

    refute headers.key?("X-DeploySentry-Session"), "X-DeploySentry-Session header must not be present when session_id is nil"
  end
end

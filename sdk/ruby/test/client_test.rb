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

  # ---------------------------------------------------------------------------
  # register / dispatch tests
  # ---------------------------------------------------------------------------

  def setup
    @client = DeploySentry::Client.new(**VALID_OPTS)
  end

  # 1. flagged-on: when the flag is enabled, the flag-specific handler is returned
  def test_dispatch_returns_flagged_handler_when_flag_enabled
    flag_handler = -> { "flagged" }
    default_handler = -> { "default" }
    @client.register("send_email", default_handler)
    @client.register("send_email", flag_handler, flag_key: "new-mailer")
    @client.instance_variable_get(:@flags)["new-mailer"] = DeploySentry::Flag.new(key: "new-mailer", enabled: true)

    result = @client.dispatch("send_email")
    assert_equal "flagged", result.call
  end

  # 2. flagged-off/default: when the flag is disabled, the default handler is returned
  def test_dispatch_returns_default_when_flag_disabled
    flag_handler = -> { "flagged" }
    default_handler = -> { "default" }
    @client.register("send_email", default_handler)
    @client.register("send_email", flag_handler, flag_key: "new-mailer")
    @client.instance_variable_get(:@flags)["new-mailer"] = DeploySentry::Flag.new(key: "new-mailer", enabled: false)

    result = @client.dispatch("send_email")
    assert_equal "default", result.call
  end

  # 3. first-match-wins: first enabled flag in registration order wins
  def test_dispatch_first_match_wins
    handler_a = -> { "handler_a" }
    handler_b = -> { "handler_b" }
    default_handler = -> { "default" }
    @client.register("op", default_handler)
    @client.register("op", handler_a, flag_key: "flag-a")
    @client.register("op", handler_b, flag_key: "flag-b")
    flags = @client.instance_variable_get(:@flags)
    flags["flag-a"] = DeploySentry::Flag.new(key: "flag-a", enabled: true)
    flags["flag-b"] = DeploySentry::Flag.new(key: "flag-b", enabled: true)

    result = @client.dispatch("op")
    assert_equal "handler_a", result.call
  end

  # 4. default-only: works fine when no flag-specific handlers are registered
  def test_dispatch_default_only
    default_handler = -> { "only_default" }
    @client.register("render", default_handler)

    result = @client.dispatch("render")
    assert_equal "only_default", result.call
  end

  # 5. isolation: registrations for different operations are independent
  def test_dispatch_operations_are_isolated
    handler_a = -> { "operation_a" }
    handler_b = -> { "operation_b" }
    @client.register("op_a", handler_a)
    @client.register("op_b", handler_b)

    assert_equal "operation_a", @client.dispatch("op_a").call
    assert_equal "operation_b", @client.dispatch("op_b").call
  end

  # 6. throw unregistered: raises when no handlers exist for the operation
  def test_dispatch_raises_when_operation_not_registered
    err = assert_raises(RuntimeError) do
      @client.dispatch("unknown_op")
    end
    assert_match "No handlers registered for operation 'unknown_op'", err.message
  end

  # 7. throw no-match-no-default: raises when a flag is enabled but no default exists
  def test_dispatch_raises_when_no_default_and_flag_disabled
    flag_handler = -> { "flagged" }
    @client.register("op", flag_handler, flag_key: "my-flag")
    @client.instance_variable_get(:@flags)["my-flag"] = DeploySentry::Flag.new(key: "my-flag", enabled: false)

    err = assert_raises(RuntimeError) do
      @client.dispatch("op")
    end
    assert_match "no default registered", err.message
  end

  # 8. replace default: re-registering without flag_key replaces the previous default
  def test_register_replaces_default_handler
    old_default = -> { "old" }
    new_default = -> { "new" }
    @client.register("op", old_default)
    @client.register("op", new_default)

    result = @client.dispatch("op")
    assert_equal "new", result.call
  end

  # 9. pass-through args: returned handler can be called with arguments
  def test_dispatch_handler_accepts_arguments
    handler = ->(x, y) { x + y }
    @client.register("add", handler)

    result = @client.dispatch("add")
    assert_equal 7, result.call(3, 4)
  end
end

# frozen_string_literal: true

require "json"
require "minitest/autorun"
require "ostruct"

$LOAD_PATH.unshift File.expand_path("../lib", __dir__)
require "deploysentry"

class StatusReporterTest < Minitest::Test
  def make_http_stub(status: 201, &on_request)
    lambda do |uri, req|
      on_request&.call(uri, req)
      OpenStruct.new(code: status.to_s, message: "")
    end
  end

  def test_resolve_version_prefers_explicit
    ENV["APP_VERSION"] = "from-env"
    assert_equal "1.2.3", DeploySentry::StatusReporter.resolve_version("1.2.3")
  ensure
    ENV.delete("APP_VERSION")
  end

  def test_resolve_version_env_chain
    %w[APP_VERSION GIT_SHA GIT_COMMIT SOURCE_COMMIT RAILWAY_GIT_COMMIT_SHA
       RENDER_GIT_COMMIT VERCEL_GIT_COMMIT_SHA HEROKU_SLUG_COMMIT].each { |k| ENV.delete(k) }
    ENV["GIT_SHA"] = "abc123"
    assert_equal "abc123", DeploySentry::StatusReporter.resolve_version
  ensure
    ENV.delete("GIT_SHA")
  end

  def test_resolve_version_unknown_fallback
    %w[APP_VERSION GIT_SHA GIT_COMMIT SOURCE_COMMIT RAILWAY_GIT_COMMIT_SHA
       RENDER_GIT_COMMIT VERCEL_GIT_COMMIT_SHA HEROKU_SLUG_COMMIT].each { |k| ENV.delete(k) }
    assert_equal "unknown", DeploySentry::StatusReporter.resolve_version
  end

  def test_report_once_posts_to_correct_url_with_body
    captured = {}
    http = make_http_stub do |uri, req|
      captured[:path] = uri.path
      captured[:host] = uri.host
      captured[:auth] = req["Authorization"]
      captured[:content_type] = req["Content-Type"]
      captured[:body] = JSON.parse(req.body)
    end

    reporter = DeploySentry::StatusReporter.new(
      base_url: "https://api.example.com",
      api_key: "ds_test",
      application_id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      interval_s: 0,
      version: "1.4.2",
      commit_sha: "abc123",
      deploy_slot: "canary",
      tags: { "region" => "us-east" },
      http: http,
    )
    reporter.report_once

    assert_equal "/api/v1/applications/f47ac10b-58cc-4372-a567-0e02b2c3d479/status", captured[:path]
    assert_equal "api.example.com", captured[:host]
    assert_equal "ApiKey ds_test", captured[:auth]
    assert_equal "application/json", captured[:content_type]
    assert_equal "1.4.2", captured[:body]["version"]
    assert_equal "healthy", captured[:body]["health"]
    assert_equal "abc123", captured[:body]["commit_sha"]
    assert_equal "canary", captured[:body]["deploy_slot"]
    assert_equal({ "region" => "us-east" }, captured[:body]["tags"])
  end

  def test_report_once_invokes_health_provider
    captured = {}
    http = make_http_stub { |_u, req| captured[:body] = JSON.parse(req.body) }
    reporter = DeploySentry::StatusReporter.new(
      base_url: "http://x",
      api_key: "k",
      application_id: "a",
      interval_s: 0,
      version: "1",
      health_provider: -> { DeploySentry::StatusReporter::HealthReport.new(state: "degraded", score: 0.8, reason: "db slow") },
      http: http,
    )
    reporter.report_once
    assert_equal "degraded", captured[:body]["health"]
    assert_equal 0.8, captured[:body]["health_score"]
    assert_equal "db slow", captured[:body]["health_reason"]
  end

  def test_report_once_health_provider_error_is_unknown
    captured = {}
    http = make_http_stub { |_u, req| captured[:body] = JSON.parse(req.body) }
    reporter = DeploySentry::StatusReporter.new(
      base_url: "http://x",
      api_key: "k",
      application_id: "a",
      interval_s: 0,
      version: "1",
      health_provider: -> { raise "boom" },
      http: http,
    )
    reporter.report_once
    assert_equal "unknown", captured[:body]["health"]
    assert_includes captured[:body]["health_reason"], "boom"
  end

  def test_report_once_raises_on_5xx
    http = make_http_stub(status: 500)
    reporter = DeploySentry::StatusReporter.new(
      base_url: "http://x",
      api_key: "k",
      application_id: "a",
      interval_s: 0,
      version: "1",
      http: http,
    )
    assert_raises(RuntimeError) { reporter.report_once }
  end

  def test_negative_interval_rejected
    assert_raises(ArgumentError) do
      DeploySentry::StatusReporter.new(
        base_url: "http://x",
        api_key: "k",
        application_id: "a",
        interval_s: -1,
      )
    end
  end
end

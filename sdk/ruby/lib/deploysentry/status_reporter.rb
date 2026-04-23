# frozen_string_literal: true

require "json"
require "net/http"
require "uri"

module DeploySentry
  # StatusReporter posts periodic POST /applications/:id/status updates to
  # DeploySentry on behalf of the Client. Failures are swallowed so flag
  # evaluation is never blocked by reporting.
  class StatusReporter
    VERSION_ENV_CHAIN = %w[
      APP_VERSION
      GIT_SHA
      GIT_COMMIT
      SOURCE_COMMIT
      RAILWAY_GIT_COMMIT_SHA
      RENDER_GIT_COMMIT
      VERCEL_GIT_COMMIT_SHA
      HEROKU_SLUG_COMMIT
    ].freeze

    MIN_BACKOFF_S = 1.0
    MAX_BACKOFF_S = 5 * 60.0

    # HealthReport — the shape expected from a health_provider callable.
    HealthReport = Struct.new(:state, :score, :reason, keyword_init: true) do
      def initialize(state: "healthy", score: nil, reason: nil)
        super
      end
    end

    # Pick the reported version (explicit override -> env vars -> "unknown").
    def self.resolve_version(explicit = nil, gem_name: nil)
      return explicit if explicit && !explicit.empty?

      VERSION_ENV_CHAIN.each do |name|
        v = ENV[name]
        return v if v && !v.empty?
      end

      if gem_name
        spec = Gem.loaded_specs[gem_name]
        return spec.version.to_s if spec
      end

      "unknown"
    end

    def initialize(base_url:, api_key:, application_id:, interval_s: 30.0,
                   version: nil, commit_sha: nil, deploy_slot: nil,
                   tags: nil, health_provider: nil, http: nil, logger: nil)
      raise ArgumentError, "interval_s must be >= 0" if interval_s.negative?

      @base_url = base_url.chomp("/")
      @api_key = api_key
      @application_id = application_id
      @interval_s = interval_s
      @version = version
      @commit_sha = commit_sha
      @deploy_slot = deploy_slot
      @tags = tags
      @health_provider = health_provider
      @http = http # optional injection seam for tests
      @logger = logger
      @stopped = false
      @thread = nil
      @backoff = 0.0
    end

    def start
      @stopped = false
      @thread = Thread.new { run_loop }
      @thread.abort_on_exception = false
    end

    def stop
      @stopped = true
      @thread&.wakeup if @thread&.alive?
      @thread = nil
    end

    # Send exactly one report. Raises on HTTP errors.
    def report_once
      version = self.class.resolve_version(@version)
      health =
        if @health_provider
          begin
            @health_provider.call
          rescue StandardError => e
            HealthReport.new(state: "unknown", reason: e.message)
          end
        else
          HealthReport.new(state: "healthy")
        end

      body = { "version" => version, "health" => health.state }
      body["health_score"] = health.score unless health.score.nil?
      body["health_reason"] = health.reason if health.reason && !health.reason.empty?
      body["commit_sha"] = @commit_sha if @commit_sha
      body["deploy_slot"] = @deploy_slot if @deploy_slot
      body["tags"] = @tags if @tags && !@tags.empty?

      uri = URI.parse("#{@base_url}/api/v1/applications/#{@application_id}/status")
      req = Net::HTTP::Post.new(uri.request_uri, {
        "Authorization" => "ApiKey #{@api_key}",
        "Content-Type" => "application/json",
      })
      req.body = JSON.generate(body)

      response =
        if @http
          @http.call(uri, req)
        else
          Net::HTTP.start(uri.hostname, uri.port, use_ssl: uri.scheme == "https") { |h| h.request(req) }
        end

      code = response.code.to_i
      raise "status report failed: #{response.code} #{response.message}" if code < 200 || code >= 300

      response
    end

    private

    def run_loop
      loop do
        begin
          report_once
          @backoff = 0.0
        rescue StandardError => e
          @logger&.warn("deploysentry: status report error: #{e.message}")
          @backoff = [@backoff.zero? ? MIN_BACKOFF_S : @backoff * 2, MAX_BACKOFF_S].min
        end
        return if @stopped
        return if @interval_s.zero?

        sleep(@backoff.positive? ? @backoff : @interval_s)
        return if @stopped
      end
    end
  end
end

# frozen_string_literal: true

require "net/http"
require "uri"
require "json"
require "time"

module DeploySentry
  class Client
    attr_reader :environment, :project

    def initialize(api_key:, base_url:, environment:, project:, cache_timeout: 30, offline_mode: false, session_id: nil,
                   application_id: nil, report_status: false, report_status_interval: 30.0,
                   report_status_version: nil, report_status_commit_sha: nil,
                   report_status_deploy_slot: nil, report_status_tags: nil,
                   report_status_health_provider: nil)
      raise ConfigurationError, "api_key is required" if api_key.nil? || api_key.empty?
      raise ConfigurationError, "base_url is required" if base_url.nil? || base_url.empty?
      raise ConfigurationError, "environment is required" if environment.nil? || environment.empty?
      raise ConfigurationError, "project is required" if project.nil? || project.empty?

      @api_key = api_key
      @base_url = base_url.chomp("/")
      @environment = environment
      @project = project
      @offline_mode = offline_mode
      @session_id = session_id

      @flags = {}
      @flags_mutex = Mutex.new
      @cache = Cache.new(ttl: cache_timeout)
      @sse_client = nil
      @initialized = false
      @registry = {}
      @registry_mutex = Mutex.new

      @application_id = application_id
      @report_status = report_status
      @report_status_interval = report_status_interval
      @report_status_version = report_status_version
      @report_status_commit_sha = report_status_commit_sha
      @report_status_deploy_slot = report_status_deploy_slot
      @report_status_tags = report_status_tags
      @report_status_health_provider = report_status_health_provider
      @status_reporter = nil
    end

    def initialize!
      fetch_flags
      start_streaming unless @offline_mode
      start_status_reporter
      @initialized = true
      self
    end

    def close
      @status_reporter&.stop
      @status_reporter = nil
      @sse_client&.stop
      @sse_client = nil
      @cache.clear
      @initialized = false
    end

    private def start_status_reporter
      return unless @report_status
      if @application_id.nil? || @application_id.empty?
        warn "[DeploySentry] report_status=true but application_id is empty; status reporter disabled"
        return
      end
      @status_reporter = StatusReporter.new(
        base_url: @base_url,
        api_key: @api_key,
        application_id: @application_id,
        interval_s: @report_status_interval,
        version: @report_status_version,
        commit_sha: @report_status_commit_sha,
        deploy_slot: @report_status_deploy_slot,
        tags: @report_status_tags,
        health_provider: @report_status_health_provider,
      )
      @status_reporter.start
    end

    public

    def initialized?
      @initialized
    end

    def refresh_session
      @cache.clear
      @flags_mutex.synchronize { @flags.clear }
      fetch_flags
    end

    # ----- Value accessors -----

    def bool_value(key, default:, context: nil)
      result = evaluate(key, context: context)
      return default unless result&.success? && !result.value.nil?

      case result.value
      when true, false then result.value
      when "true" then true
      when "false" then false
      else default
      end
    end

    def string_value(key, default:, context: nil)
      result = evaluate(key, context: context)
      return default unless result&.success? && !result.value.nil?

      result.value.to_s
    end

    def int_value(key, default:, context: nil)
      result = evaluate(key, context: context)
      return default unless result&.success? && !result.value.nil?

      Integer(result.value)
    rescue ArgumentError, TypeError
      default
    end

    def json_value(key, default:, context: nil)
      result = evaluate(key, context: context)
      return default unless result&.success? && !result.value.nil?

      value = result.value
      value.is_a?(String) ? JSON.parse(value) : value
    rescue JSON::ParserError
      default
    end

    def enabled?(key, context: nil)
      bool_value(key, default: false, context: context)
    end

    def detail(key, context: nil)
      evaluate(key, context: context) || EvaluationResult.new(
        key: key,
        value: nil,
        reason: "NOT_FOUND",
        error: "Flag '#{key}' not found"
      )
    end

    # ----- Metadata queries -----

    def flags_by_category(category)
      @flags_mutex.synchronize do
        @flags.values.select { |f| f.metadata&.category == category }
      end
    end

    def expired_flags
      @flags_mutex.synchronize do
        @flags.values.select { |f| f.metadata&.expired? }
      end
    end

    def flag_owners(key)
      @flags_mutex.synchronize do
        flag = @flags[key]
        return [] unless flag&.metadata

        flag.metadata.owners
      end
    end

    def register(operation, handler, flag_key: nil)
      @registry_mutex.synchronize do
        list = @registry[operation] ||= []
        if flag_key.nil?
          idx = list.index { |r| r[:flag_key].nil? }
          if idx
            list[idx] = { handler: handler, flag_key: nil }
          else
            list.push({ handler: handler, flag_key: nil })
          end
        else
          list.push({ handler: handler, flag_key: flag_key })
        end
      end
    end

    def dispatch(operation, context: nil)
      list = @registry[operation]
      if list.nil? || list.empty?
        raise "No handlers registered for operation '#{operation}'. Call register() before dispatch()."
      end
      list.each do |reg|
        next if reg[:flag_key].nil?
        flag = @flags[reg[:flag_key]]
        return reg[:handler] if flag&.enabled
      end
      default_reg = list.find { |r| r[:flag_key].nil? }
      unless default_reg
        raise "No matching handler for operation '#{operation}' and no default registered. Register a default handler (no flag_key) as the last registration."
      end
      default_reg[:handler]
    end

    private

    def auth_headers
      headers = {
        "Authorization" => "ApiKey #{@api_key}",
        "Content-Type" => "application/json",
        "Accept" => "application/json"
      }
      headers["X-DeploySentry-Session"] = @session_id if @session_id
      headers
    end

    def evaluate(key, context: nil)
      cache_key = build_cache_key(key, context)
      cached = @cache.get(cache_key)
      return cached if cached

      flag = @flags_mutex.synchronize { @flags[key] }

      if @offline_mode
        return build_offline_result(key, flag)
      end

      result = evaluate_remote(key, context)
      @cache.set(cache_key, result) if result&.success?
      result
    rescue => e
      # Fall back to local flag state on network errors
      if flag
        build_offline_result(key, flag)
      else
        EvaluationResult.new(key: key, value: nil, reason: "ERROR", error: e.message)
      end
    end

    def evaluate_remote(key, context)
      uri = URI("#{@base_url}/api/v1/flags/evaluate")
      body = {
        flag_key: key,
        environment: @environment,
        project: @project
      }
      body[:context] = context.to_h if context
      body[:session_id] = @session_id if @session_id

      response = post(uri, body)
      data = JSON.parse(response.body, symbolize_names: true)

      flag = @flags_mutex.synchronize { @flags[key] }

      EvaluationResult.new(
        key: key,
        value: data[:value],
        type: data[:type],
        reason: data[:reason] || "EVALUATED",
        flag: flag,
        metadata: flag&.metadata
      )
    end

    def build_offline_result(key, flag)
      return EvaluationResult.new(key: key, value: nil, reason: "NOT_FOUND", error: "Flag not found") unless flag

      EvaluationResult.new(
        key: key,
        value: flag.value,
        type: flag.type,
        reason: "OFFLINE",
        flag: flag,
        metadata: flag.metadata
      )
    end

    def fetch_flags
      uri = URI("#{@base_url}/api/v1/flags")
      uri.query = URI.encode_www_form(project_id: @project, environment: @environment)

      response = get(uri)
      data = JSON.parse(response.body, symbolize_names: true)
      flags_data = data.is_a?(Array) ? data : (data[:flags] || data[:data] || [])

      @flags_mutex.synchronize do
        @flags.clear
        flags_data.each do |fd|
          metadata = if fd[:metadata]
            meta = fd[:metadata]
            FlagMetadata.new(
              category: meta[:category],
              purpose: meta[:purpose],
              owners: meta[:owners],
              is_permanent: meta[:is_permanent] || false,
              expires_at: meta[:expires_at],
              tags: meta[:tags]
            )
          end

          flag = Flag.new(
            key: fd[:key].to_s,
            value: fd[:value],
            type: fd[:type] || "boolean",
            enabled: fd[:enabled] || false,
            metadata: metadata
          )
          @flags[flag.key] = flag
        end
      end
    end

    def start_streaming
      stream_url = "#{@base_url}/api/v1/flags/stream?project_id=#{@project}&environment=#{@environment}"

      @sse_client = SSEClient.new(
        url: stream_url,
        headers: auth_headers.merge("Accept" => "text/event-stream"),
        on_event: method(:handle_sse_event),
        on_error: method(:handle_sse_error)
      )
      @sse_client.start
    end

    def handle_sse_event(_event_type, data)
      return unless data.is_a?(Hash)

      flag_key = data[:flag_key]&.to_s
      flag_id = data[:flag_id]&.to_s

      if data[:event] == "flag.deleted"
        return unless flag_key
        @flags_mutex.synchronize { @flags.delete(flag_key) }
        @cache.keys.each do |cache_key|
          @cache.delete(cache_key) if cache_key.start_with?("#{flag_key}:")
        end
        return
      end

      return unless flag_id

      begin
        flag = fetch_single_flag(flag_id)
        return unless flag

        @flags_mutex.synchronize { @flags[flag.key] = flag }

        # Invalidate cached evaluations for this flag
        @cache.keys.each do |cache_key|
          @cache.delete(cache_key) if cache_key.start_with?("#{flag.key}:")
        end
      rescue => _e
        # Fetch failed; cache remains stale until next poll or event
      end
    end

    def fetch_single_flag(flag_id)
      uri = URI("#{@base_url}/api/v1/flags/#{flag_id}")
      uri.query = URI.encode_www_form(environment: @environment)

      response = get(uri)
      fd = JSON.parse(response.body, symbolize_names: true)

      metadata = if fd[:metadata]
        meta = fd[:metadata]
        FlagMetadata.new(
          category: meta[:category],
          purpose: meta[:purpose],
          owners: meta[:owners],
          is_permanent: meta[:is_permanent] || false,
          expires_at: meta[:expires_at],
          tags: meta[:tags]
        )
      end

      Flag.new(
        key: fd[:key].to_s,
        value: fd[:value],
        type: fd[:type] || "boolean",
        enabled: fd[:enabled] || false,
        metadata: metadata
      )
    end

    def handle_sse_error(error)
      # Errors are handled internally by SSEClient with reconnection logic.
      # Override this method or provide a logger for production use.
    end

    def build_cache_key(key, context)
      parts = [key]
      if context
        parts << context.user_id.to_s
        parts << context.org_id.to_s
        parts << context.attributes.sort.map { |k, v| "#{k}=#{v}" }.join(",")
      end
      parts.join(":")
    end

    def get(uri)
      http = build_http(uri)
      request = Net::HTTP::Get.new(uri)
      auth_headers.each { |k, v| request[k] = v }

      response = http.request(request)
      raise ApiError, "HTTP #{response.code}: #{response.body}" unless response.is_a?(Net::HTTPSuccess)

      response
    end

    def post(uri, body)
      http = build_http(uri)
      request = Net::HTTP::Post.new(uri)
      auth_headers.each { |k, v| request[k] = v }
      request.body = JSON.generate(body)

      response = http.request(request)
      raise ApiError, "HTTP #{response.code}: #{response.body}" unless response.is_a?(Net::HTTPSuccess)

      response
    end

    def build_http(uri)
      http = Net::HTTP.new(uri.host, uri.port)
      http.use_ssl = uri.scheme == "https"
      http.open_timeout = 10
      http.read_timeout = 15
      http
    end
  end
end

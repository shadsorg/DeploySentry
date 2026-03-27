# frozen_string_literal: true

require "net/http"
require "uri"
require "json"

module DeploySentry
  class SSEClient
    INITIAL_RETRY_DELAY = 1
    MAX_RETRY_DELAY = 30
    BACKOFF_MULTIPLIER = 2
    JITTER_FRACTION = 0.2

    def initialize(url:, headers: {}, on_event: nil, on_error: nil)
      @url = URI(url)
      @headers = headers
      @on_event = on_event
      @on_error = on_error
      @running = false
      @thread = nil
      @mutex = Mutex.new
    end

    def start
      @mutex.synchronize do
        return if @running

        @running = true
        @thread = Thread.new { run_loop }
        @thread.abort_on_exception = false
        @thread.name = "deploysentry-sse"
      end
    end

    def stop
      @mutex.synchronize do
        @running = false
      end
      @thread&.join(5)
      @thread = nil
    end

    def running?
      @mutex.synchronize { @running }
    end

    private

    def run_loop
      delay = INITIAL_RETRY_DELAY

      while running?
        begin
          connect_and_stream
          delay = INITIAL_RETRY_DELAY
        rescue => e
          notify_error(e) if running?
          if running?
            jitter = delay * JITTER_FRACTION * (2 * rand - 1)
            jittered = [delay + jitter, 0].max
            sleep(jittered)
            delay = [delay * BACKOFF_MULTIPLIER, MAX_RETRY_DELAY].min
          end
        end
      end
    end

    def connect_and_stream
      http = Net::HTTP.new(@url.host, @url.port)
      http.use_ssl = @url.scheme == "https"
      http.open_timeout = 10
      http.read_timeout = 0

      request = Net::HTTP::Get.new(@url)
      @headers.each { |k, v| request[k] = v }
      request["Accept"] = "text/event-stream"
      request["Cache-Control"] = "no-cache"

      http.request(request) do |response|
        unless response.is_a?(Net::HTTPSuccess)
          raise ConnectionError, "SSE connection failed: #{response.code} #{response.message}"
        end

        buffer = ""

        response.read_body do |chunk|
          break unless running?

          buffer += chunk
          while (idx = buffer.index("\n\n"))
            raw_event = buffer[0...idx]
            buffer = buffer[(idx + 2)..]
            parse_and_dispatch(raw_event)
          end
        end
      end
    ensure
      http&.finish if http&.started?
    end

    def parse_and_dispatch(raw)
      event_type = nil
      data_lines = []

      raw.each_line do |line|
        line = line.chomp
        if line.start_with?("event:")
          event_type = line.sub("event:", "").strip
        elsif line.start_with?("data:")
          data_lines << line.sub("data:", "").strip
        end
      end

      return if data_lines.empty?

      data = data_lines.join("\n")
      parsed = begin
        JSON.parse(data, symbolize_names: true)
      rescue JSON::ParserError
        data
      end

      @on_event&.call(event_type || "message", parsed)
    end

    def notify_error(error)
      @on_error&.call(error)
    end
  end
end

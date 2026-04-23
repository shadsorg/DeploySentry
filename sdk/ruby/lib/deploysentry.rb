# frozen_string_literal: true

module DeploySentry
  VERSION = "1.0.0"

  class Error < StandardError; end
  class ConfigurationError < Error; end
  class ApiError < Error; end
  class ConnectionError < Error; end
end

require_relative "deploysentry/models"
require_relative "deploysentry/cache"
require_relative "deploysentry/streaming"
require_relative "deploysentry/status_reporter"
require_relative "deploysentry/client"

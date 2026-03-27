# frozen_string_literal: true

Gem::Specification.new do |spec|
  spec.name          = "deploysentry"
  spec.version       = "1.0.0"
  spec.authors       = ["DeploySentry"]
  spec.email         = ["support@deploysentry.com"]

  spec.summary       = "Ruby SDK for DeploySentry feature flag management"
  spec.description   = "Evaluate feature flags with rich metadata, categories, ownership tracking, " \
                        "and real-time updates via SSE streaming. Thread-safe with in-memory caching."
  spec.homepage      = "https://github.com/shadsorg/DeploySentry"
  spec.license       = "MIT"

  spec.required_ruby_version = ">= 3.0.0"

  spec.files = Dir["lib/**/*.rb"] + ["deploysentry.gemspec", "Gemfile", "README.md"]
  spec.require_paths = ["lib"]

  spec.metadata["homepage_uri"]    = spec.homepage
  spec.metadata["source_code_uri"] = spec.homepage
end

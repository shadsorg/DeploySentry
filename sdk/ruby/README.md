# DeploySentry Ruby SDK

Official Ruby SDK for integrating with the DeploySentry platform.

## Installation

```bash
gem install deploysentry
```

Or add to your Gemfile:

```ruby
gem 'deploysentry', '~> 0.1.0'
```

## Quick Start

```ruby
require 'deploysentry'

client = DeploySentry::Client.new(api_key: 'your-api-key')

# Evaluate a feature flag
if client.flags.enabled?('my-feature', context: { user_id: 'user-123' })
  # New feature code path
end
```

## Documentation

Full documentation is available at [docs.deploysentry.io/sdk/ruby](https://docs.deploysentry.io/sdk/ruby).

## License

Apache-2.0

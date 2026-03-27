# frozen_string_literal: true

require "time"

module DeploySentry
  class Cache
    Entry = Struct.new(:value, :expires_at, keyword_init: true)

    def initialize(ttl: 30)
      @ttl = ttl
      @store = {}
      @mutex = Mutex.new
    end

    def get(key)
      @mutex.synchronize do
        entry = @store[key]
        return nil if entry.nil?

        if entry.expires_at < Time.now
          @store.delete(key)
          return nil
        end

        entry.value
      end
    end

    def set(key, value, ttl: nil)
      @mutex.synchronize do
        @store[key] = Entry.new(
          value: value,
          expires_at: Time.now + (ttl || @ttl)
        )
      end
      value
    end

    def delete(key)
      @mutex.synchronize { @store.delete(key) }
    end

    def clear
      @mutex.synchronize { @store.clear }
    end

    def size
      @mutex.synchronize do
        evict_expired
        @store.size
      end
    end

    def keys
      @mutex.synchronize do
        evict_expired
        @store.keys.dup
      end
    end

    private

    def evict_expired
      now = Time.now
      @store.delete_if { |_, entry| entry.expires_at < now }
    end
  end
end

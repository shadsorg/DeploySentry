package io.deploysentry;

import java.util.function.Supplier;

/**
 * Associates a handler {@link Supplier} with an optional feature-flag key.
 * When {@code flagKey} is {@code null} this registration acts as the default
 * (fallback) handler for the operation.
 *
 * @param <T> the return type produced by the handler
 */
class Registration<T> {
    final Supplier<T> handler;
    final String flagKey; // null = default

    Registration(Supplier<T> handler, String flagKey) {
        this.handler = handler;
        this.flagKey = flagKey;
    }
}

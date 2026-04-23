package io.deploysentry;

/**
 * Shape returned by a user-supplied health provider for the agentless
 * status reporter. A {@code null} score is permitted and omitted from the
 * wire payload.
 */
public final class HealthReport {

    private final String state; // healthy | degraded | unhealthy | unknown
    private final Double score;
    private final String reason;

    public HealthReport(String state, Double score, String reason) {
        this.state = state;
        this.score = score;
        this.reason = reason;
    }

    public HealthReport(String state) {
        this(state, null, null);
    }

    public String getState() {
        return state;
    }

    public Double getScore() {
        return score;
    }

    public String getReason() {
        return reason;
    }
}

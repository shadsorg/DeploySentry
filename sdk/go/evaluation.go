package deploysentry

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// BoolValue evaluates a boolean feature flag and returns its value. If the
// flag cannot be evaluated, defaultValue is returned.
func (c *Client) BoolValue(ctx context.Context, flagKey string, defaultValue bool, evalCtx *EvaluationContext) (bool, error) {
	result, err := c.resolve(ctx, flagKey, evalCtx)
	if err != nil {
		return defaultValue, err
	}

	var v bool
	if err := json.Unmarshal(result.Value, &v); err != nil {
		// Try parsing the string representation (API may return "true"/"false").
		s, sErr := unquote(result.Value)
		if sErr != nil {
			return defaultValue, fmt.Errorf("deploysentry: flag %q value is not a boolean: %w", flagKey, err)
		}
		parsed, pErr := strconv.ParseBool(s)
		if pErr != nil {
			return defaultValue, fmt.Errorf("deploysentry: flag %q value %q is not a boolean", flagKey, s)
		}
		return parsed, nil
	}
	return v, nil
}

// StringValue evaluates a string feature flag and returns its value. If the
// flag cannot be evaluated, defaultValue is returned.
func (c *Client) StringValue(ctx context.Context, flagKey string, defaultValue string, evalCtx *EvaluationContext) (string, error) {
	result, err := c.resolve(ctx, flagKey, evalCtx)
	if err != nil {
		return defaultValue, err
	}

	var v string
	if err := json.Unmarshal(result.Value, &v); err != nil {
		return defaultValue, fmt.Errorf("deploysentry: flag %q value is not a string: %w", flagKey, err)
	}
	return v, nil
}

// IntValue evaluates an integer feature flag and returns its value. If the
// flag cannot be evaluated, defaultValue is returned.
func (c *Client) IntValue(ctx context.Context, flagKey string, defaultValue int64, evalCtx *EvaluationContext) (int64, error) {
	result, err := c.resolve(ctx, flagKey, evalCtx)
	if err != nil {
		return defaultValue, err
	}

	var v json.Number
	if err := json.Unmarshal(result.Value, &v); err != nil {
		return defaultValue, fmt.Errorf("deploysentry: flag %q value is not a number: %w", flagKey, err)
	}
	n, err := v.Int64()
	if err != nil {
		return defaultValue, fmt.Errorf("deploysentry: flag %q value %q is not an integer: %w", flagKey, v, err)
	}
	return n, nil
}

// JSONValue evaluates a JSON feature flag and unmarshals the result into
// dest. If the flag cannot be evaluated, dest is left unchanged and the
// error is returned.
func (c *Client) JSONValue(ctx context.Context, flagKey string, dest interface{}, evalCtx *EvaluationContext) error {
	result, err := c.resolve(ctx, flagKey, evalCtx)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(result.Value, dest); err != nil {
		return fmt.Errorf("deploysentry: flag %q value is not valid JSON for target type: %w", flagKey, err)
	}
	return nil
}

// Detail evaluates a flag and returns the full EvaluationResult including
// metadata, reason, and the resolved value.
func (c *Client) Detail(ctx context.Context, flagKey string, evalCtx *EvaluationContext) (*EvaluationResult, error) {
	resp, err := c.resolve(ctx, flagKey, evalCtx)
	if err != nil {
		return nil, err
	}

	var value interface{}
	_ = json.Unmarshal(resp.Value, &value)

	return &EvaluationResult{
		FlagKey:  resp.FlagKey,
		Value:    value,
		Reason:   resp.Reason,
		Metadata: resp.Metadata,
		FlagType: resp.FlagType,
		Enabled:  resp.Enabled,
	}, nil
}

// resolve is the internal evaluation path. It first attempts a live API call.
// On failure it falls back to the cache (returning stale data when offline
// mode is enabled).
func (c *Client) resolve(ctx context.Context, flagKey string, evalCtx *EvaluationContext) (*evaluateResponse, error) {
	resp, err := c.doEvaluate(ctx, flagKey, evalCtx)
	if err == nil {
		return resp, nil
	}

	// API call failed -- try the cache.
	flag, found, fresh := c.cache.get(flagKey)
	if found && (fresh || c.offlineMode) {
		reason := "CACHED"
		if !fresh {
			reason = "STALE_CACHE"
		}
		return &evaluateResponse{
			FlagKey:  flag.Key,
			Value:    json.RawMessage(fmt.Sprintf("%q", flag.DefaultValue)),
			Reason:   reason,
			FlagType: flag.FlagType,
			Enabled:  flag.Enabled,
			Metadata: flag.Metadata,
		}, nil
	}

	return nil, fmt.Errorf("deploysentry: failed to evaluate flag %q: %w", flagKey, err)
}

// unquote attempts to JSON-unquote a raw message that is expected to be a
// quoted string.
func unquote(raw json.RawMessage) (string, error) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}

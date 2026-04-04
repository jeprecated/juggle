package provider

import (
	"testing"
	"time"
)

func TestParseQuotaExhausted_Patterns(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		wantQuota       bool
		wantRateLimited bool
	}{
		{"daily limit", "You have reached your daily limit", true, true},
		{"daily quota", "Daily quota exceeded", true, true},
		{"usage limit reached", "usage limit reached for this account", true, true},
		{"monthly limit", "monthly limit exceeded", true, true},
		{"insufficient_quota", "insufficient_quota for this request", true, true},
		{"quota_exceeded keyword", "quota_exceeded: daily tokens exhausted", true, true},
		{"usage_limit_reached", "usage_limit_reached error", true, true},
		// Transient rate limits should NOT set QuotaExhausted
		{"transient rate limit", "rate limit exceeded, try again in 30 seconds", false, true},
		{"429 transient", "HTTP 429 Too Many Requests", false, true},
		{"throttled", "Request was throttled", false, true},
		{"overloaded transient", "Server is overloaded please try again", false, true},
		// Normal output
		{"normal", "Task completed successfully", false, false},
		{"empty", "", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &RunResult{Output: tc.output}
			parseRateLimit(result)
			if result.QuotaExhausted != tc.wantQuota {
				t.Errorf("QuotaExhausted = %v, want %v for %q", result.QuotaExhausted, tc.wantQuota, tc.output)
			}
			if result.RateLimited != tc.wantRateLimited {
				t.Errorf("RateLimited = %v, want %v for %q", result.RateLimited, tc.wantRateLimited, tc.output)
			}
		})
	}
}

func TestOpenCodeParseQuotaExhausted_Patterns(t *testing.T) {
	p := NewOpenCodeProvider()

	tests := []struct {
		name            string
		output          string
		wantQuota       bool
		wantRateLimited bool
	}{
		{"daily limit", "You have reached your daily limit", true, true},
		{"quota exceeded", "quota exceeded for this period", true, true},
		{"usage limit", "usage limit reached", true, true},
		{"insufficient_quota", "insufficient_quota error", true, true},
		// Transient
		{"rate limit transient", "rate limit exceeded", false, true},
		{"throttled", "Request was throttled", false, true},
		{"normal", "Task completed", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &RunResult{Output: tc.output}
			p.parseRateLimit(result)
			if result.QuotaExhausted != tc.wantQuota {
				t.Errorf("QuotaExhausted = %v, want %v for %q", result.QuotaExhausted, tc.wantQuota, tc.output)
			}
			if result.RateLimited != tc.wantRateLimited {
				t.Errorf("RateLimited = %v, want %v for %q", result.RateLimited, tc.wantRateLimited, tc.output)
			}
		})
	}
}

func TestParseQuotaResetTime_RelativeDurations(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantSet bool
	}{
		{"resets in 2 hours", "Quota resets in 2 hours", true},
		{"resets in 30 minutes", "quota resets in 30 minutes", true},
		{"resets in 1 hour", "Your limit resets in 1 hour", true},
		{"no time info", "Daily quota exceeded", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseQuotaResetTime(tc.output)
			if tc.wantSet && got.IsZero() {
				t.Errorf("parseQuotaResetTime(%q) = zero time, want non-zero", tc.output)
			}
			if !tc.wantSet && !got.IsZero() {
				t.Errorf("parseQuotaResetTime(%q) = %v, want zero", tc.output, got)
			}
			// Non-zero times should be in the future
			if !got.IsZero() && got.Before(time.Now()) {
				t.Errorf("parseQuotaResetTime(%q) = %v is in the past", tc.output, got)
			}
		})
	}
}

func TestParseQuotaResetTime_AbsoluteTimes(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantSet bool
	}{
		{"resets at midnight UTC", "Your quota resets at 00:00 UTC", true},
		{"try again at time", "Please try again at 16:00", true},
		{"available at time", "Service available at 08:00 UTC", true},
		{"no time", "quota exhausted for period", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseQuotaResetTime(tc.output)
			if tc.wantSet && got.IsZero() {
				t.Errorf("parseQuotaResetTime(%q) = zero time, want non-zero", tc.output)
			}
			if !tc.wantSet && !got.IsZero() {
				t.Errorf("parseQuotaResetTime(%q) = %v, want zero", tc.output, got)
			}
		})
	}
}

func TestParseRateLimit_SetsQuotaResetsAt(t *testing.T) {
	result := &RunResult{Output: "Daily quota exceeded. Your quota resets in 2 hours."}
	parseRateLimit(result)

	if !result.QuotaExhausted {
		t.Error("expected QuotaExhausted=true")
	}
	if result.QuotaResetsAt.IsZero() {
		t.Error("expected QuotaResetsAt to be set")
	}
	if result.QuotaResetsAt.Before(time.Now()) {
		t.Error("expected QuotaResetsAt to be in the future")
	}
}

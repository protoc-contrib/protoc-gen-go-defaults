package generator

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"1ms", time.Millisecond},
		{"30s", 30 * time.Second},
		{"2m", 2 * time.Minute},
		{"1h", time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"42w", 42 * 7 * 24 * time.Hour},
	}
	for _, tc := range cases {
		got, err := parseDuration(tc.in)
		if err != nil {
			t.Errorf("parseDuration(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	if _, err := parseDuration("nonsense"); err == nil {
		t.Error("parseDuration(\"nonsense\") expected error, got nil")
	}
}

func TestParseTime(t *testing.T) {
	good := []string{
		"1952-03-11T00:00:00Z",                        // RFC3339
		"Mon, 02 Jan 2006 15:04:05 MST",               // RFC1123
		"Mon, 02 Jan 2006 15:04:05 -0700",             // RFC1123Z
		"Monday, 02-Jan-06 15:04:05 MST",              // RFC850
	}
	for _, s := range good {
		if _, err := parseTime(s); err != nil {
			t.Errorf("parseTime(%q) returned error: %v", s, err)
		}
	}

	if _, err := parseTime("not a timestamp"); err == nil {
		t.Error("parseTime(\"not a timestamp\") expected error, got nil")
	}
}

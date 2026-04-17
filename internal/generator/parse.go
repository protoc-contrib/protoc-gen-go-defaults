package generator

import (
	"errors"
	"time"

	"github.com/prometheus/common/model"
)

// parseDuration accepts the same format as prometheus/common/model.ParseDuration:
// a sequence of number+unit pairs where unit is one of y, w, d, h, m, s, ms.
// This is a superset of time.ParseDuration so expressions like "42w" work.
func parseDuration(s string) (time.Duration, error) {
	d, err := model.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return time.Duration(d), nil
}

// parseTime attempts several RFC variants, returning the first successful
// parse. The accepted formats match the historical behavior of the plugin.
func parseTime(s string) (time.Time, error) {
	for _, format := range []string{
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
	} {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("cannot parse timestamp; supported formats: RFC822 / RFC822Z / RFC850 / RFC1123 / RFC1123Z / RFC3339")
}

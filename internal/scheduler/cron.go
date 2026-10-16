package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cron is a parsed 5-field expression: minute hour day-of-month month day-of-week.
// Supported syntax per field: *, n, a-b, */s, a-b/s and comma lists; day of
// week 0-7 (0 and 7 are Sunday). As in standard cron, when both day fields
// are restricted a day matches if either does.
type Cron struct {
	minute, hour, dom, month, dow [64]bool
	domAny, dowAny                bool
	expr                          string
}

// ParseCron parses an expression.
func ParseCron(expr string) (*Cron, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron needs 5 fields (minute hour day month weekday), got %d", len(fields))
	}
	c := &Cron{expr: strings.Join(fields, " ")}
	specs := []struct {
		name     string
		set      *[64]bool
		min, max int
	}{
		{"minute", &c.minute, 0, 59},
		{"hour", &c.hour, 0, 23},
		{"day of month", &c.dom, 1, 31},
		{"month", &c.month, 1, 12},
		{"day of week", &c.dow, 0, 7},
	}
	for i, s := range specs {
		if err := parseField(fields[i], s.set, s.min, s.max); err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
	}
	if c.dow[7] {
		c.dow[0] = true
	}
	c.domAny = fields[2] == "*"
	c.dowAny = fields[4] == "*"
	return c, nil
}

func parseField(f string, set *[64]bool, min, max int) error {
	for _, part := range strings.Split(f, ",") {
		step := 1
		if i := strings.IndexByte(part, '/'); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid step in %q", part)
			}
			step = n
			part = part[:i]
		}
		lo, hi := min, max
		switch {
		case part == "*":
		case strings.Contains(part, "-"):
			a, b, _ := strings.Cut(part, "-")
			var err1, err2 error
			lo, err1 = strconv.Atoi(a)
			hi, err2 = strconv.Atoi(b)
			if err1 != nil || err2 != nil || lo > hi {
				return fmt.Errorf("invalid range %q", part)
			}
		default:
			n, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("invalid value %q", part)
			}
			lo, hi = n, n
			if step > 1 {
				hi = max // "5/15" means from 5 every 15
			}
		}
		if lo < min || hi > max {
			return fmt.Errorf("%q is outside %d-%d", part, min, max)
		}
		for v := lo; v <= hi; v += step {
			set[v] = true
		}
	}
	return nil
}

// String returns the normalized expression.
func (c *Cron) String() string { return c.expr }

func (c *Cron) dayMatches(t time.Time) bool {
	dom, dow := c.dom[t.Day()], c.dow[int(t.Weekday())]
	switch {
	case c.domAny && c.dowAny:
		return true
	case c.domAny:
		return dow
	case c.dowAny:
		return dom
	}
	return dom || dow
}

// Next returns the first matching minute strictly after t (in t's location).
// It returns the zero time if nothing matches within five years (e.g. Feb 30).
func (c *Cron) Next(t time.Time) time.Time {
	t = t.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		if !c.month[int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !c.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !c.hour[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if !c.minute[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

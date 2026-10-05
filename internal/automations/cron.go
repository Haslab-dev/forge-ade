package automations

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronExpr is a parsed 5-field cron expression (minute hour dom month dow,
// local time — the standard subset ZCode automations use).
type cronExpr struct {
	minute map[int]bool
	hour   map[int]bool
	dom    map[int]bool
	month  map[int]bool
	dow    map[int]bool
	// domStar/dowStar remember whether a field was "*" — needed for the
	// standard cron rule that a restricted dom AND dow match on either.
	domStar bool
	dowStar bool
}

// parseCron parses a 5-field cron expression. Supports "*", lists, ranges,
// and "*/step" / "a-b/step" per field. Day of week: 0-7 (0 and 7 = Sunday).
func parseCron(expr string) (*cronExpr, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}

	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	dom, domStar, err := parseCronFieldStar(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day of month: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	dow, dowStar, err := parseCronFieldStar(fields[4], 0, 7)
	if err != nil {
		return nil, fmt.Errorf("day of week: %w", err)
	}
	// Normalize 7 → 0 (both Sunday).
	if dow[7] {
		dow[0] = true
	}

	return &cronExpr{
		minute: minute, hour: hour, dom: dom, month: month, dow: dow,
		domStar: domStar, dowStar: dowStar,
	}, nil
}

// matches reports whether t falls on this schedule (second resolution ignored).
func (c *cronExpr) matches(t time.Time) bool {
	if !c.minute[t.Minute()] || !c.hour[t.Hour()] || !c.month[int(t.Month())] {
		return false
	}
	domMatch := c.dom[t.Day()]
	dowMatch := c.dow[int(t.Weekday())]
	if c.domStar && c.dowStar {
		return true
	}
	if c.domStar {
		return dowMatch
	}
	if c.dowStar {
		return domMatch
	}
	// Both restricted: standard cron ORs them.
	return domMatch || dowMatch
}

// next returns the next matching time strictly after `after`.
// Scans minute by minute (bounded to one leap year) — called rarely (on
// schedule changes and after each fire) so the simple scan wins over a
// hand-rolled calendar jump.
func (c *cronExpr) next(after time.Time) time.Time {
	t := after.Truncate(time.Minute).Add(time.Minute)
	bound := t.AddDate(1, 0, 0)
	for t.Before(bound) {
		if c.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}

// parseCronField parses one cron field into the set of matching values.
func parseCronField(spec string, min, max int) (map[int]bool, error) {
	set, _, err := parseCronFieldStar(spec, min, max)
	return set, err
}

// parseCronFieldStar also reports whether the field was a bare "*".
func parseCronFieldStar(spec string, min, max int) (map[int]bool, bool, error) {
	star := spec == "*"
	set := make(map[int]bool)
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false, fmt.Errorf("empty list item")
		}
		step := 1
		rangeSpec := part
		if i := strings.Index(part, "/"); i >= 0 {
			rangeSpec = part[:i]
			parsedStep, convErr := strconv.Atoi(part[i+1:])
			if convErr != nil || parsedStep < 1 {
				return nil, false, fmt.Errorf("invalid step in %q", part)
			}
			step = parsedStep
		}
		lo, hi := min, max
		switch {
		case rangeSpec == "*":
			// full range
		case strings.Contains(rangeSpec, "-"):
			bounds := strings.SplitN(rangeSpec, "-", 2)
			var err error
			lo, err = strconv.Atoi(bounds[0])
			if err != nil {
				return nil, false, fmt.Errorf("invalid range in %q", part)
			}
			hi, err = strconv.Atoi(bounds[1])
			if err != nil {
				return nil, false, fmt.Errorf("invalid range in %q", part)
			}
		default:
			parsed, convErr := strconv.Atoi(rangeSpec)
			if convErr != nil {
				return nil, false, fmt.Errorf("invalid value %q", rangeSpec)
			}
			lo, hi = parsed, parsed
		}
		if lo < min || hi > max || lo > hi {
			return nil, false, fmt.Errorf("value out of range [%d,%d] in %q", min, max, part)
		}
		for v := lo; v <= hi; v += step {
			set[v] = true
		}
	}
	if len(set) == 0 {
		return nil, false, fmt.Errorf("empty field")
	}
	return set, star, nil
}

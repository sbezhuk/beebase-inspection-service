package inspection

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseHealthHistoryRange_ValidRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/?from=2026-06-01&to=2026-06-03&interval=day", nil)
	from, to, fields := parseHealthHistoryRange(req)
	if len(fields) != 0 {
		t.Fatalf("fields = %v, want none", fields)
	}
	if want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC); !from.Equal(want) {
		t.Fatalf("from = %v, want %v", from, want)
	}
	if want := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC); !to.Equal(want) {
		t.Fatalf("to = %v, want %v", to, want)
	}
}

func TestParseHealthHistoryRange_IntervalDefaultsAndIsCaseInsensitive(t *testing.T) {
	for _, raw := range []string{"", "day", "DAY", "Day"} {
		req := httptest.NewRequest("GET", "/?from=2026-06-01&to=2026-06-01&interval="+raw, nil)
		_, _, fields := parseHealthHistoryRange(req)
		if len(fields) != 0 {
			t.Fatalf("interval=%q: fields = %v, want none", raw, fields)
		}
	}
}

func TestParseHealthHistoryRange_UnsupportedInterval(t *testing.T) {
	req := httptest.NewRequest("GET", "/?from=2026-06-01&to=2026-06-01&interval=week", nil)
	_, _, fields := parseHealthHistoryRange(req)
	if fields["interval"] != CodeIntervalUnsupported {
		t.Fatalf(`fields["interval"] = %q, want %q`, fields["interval"], CodeIntervalUnsupported)
	}
}

func TestParseHealthHistoryRange_MissingFromAndTo(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	_, _, fields := parseHealthHistoryRange(req)
	if fields["from"] != CodeFromRequired {
		t.Fatalf(`fields["from"] = %q, want %q`, fields["from"], CodeFromRequired)
	}
	if fields["to"] != CodeToRequired {
		t.Fatalf(`fields["to"] = %q, want %q`, fields["to"], CodeToRequired)
	}
}

func TestParseHealthHistoryRange_InvalidDateFormat(t *testing.T) {
	cases := map[string]string{
		"from": "?from=06-01-2026&to=2026-06-01",
		"to":   "?from=2026-06-01&to=not-a-date",
	}
	for field, query := range cases {
		t.Run(field, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+query, nil)
			_, _, fields := parseHealthHistoryRange(req)
			wantCode := map[string]string{"from": CodeFromInvalid, "to": CodeToInvalid}[field]
			if fields[field] != wantCode {
				t.Fatalf("fields[%q] = %q, want %q", field, fields[field], wantCode)
			}
		})
	}
}

func TestParseHealthHistoryRange_FromAfterTo(t *testing.T) {
	req := httptest.NewRequest("GET", "/?from=2026-06-10&to=2026-06-01", nil)
	_, _, fields := parseHealthHistoryRange(req)
	if fields["to"] != CodeFromAfterTo {
		t.Fatalf(`fields["to"] = %q, want %q`, fields["to"], CodeFromAfterTo)
	}
}

func TestParseHealthHistoryRange_RangeTooLong(t *testing.T) {
	// 366 inclusive days - one past the 365-point cap.
	req := httptest.NewRequest("GET", "/?from=2026-01-01&to=2027-01-01", nil)
	_, _, fields := parseHealthHistoryRange(req)
	if fields["to"] != CodeHistoryRangeTooLong {
		t.Fatalf(`fields["to"] = %q, want %q`, fields["to"], CodeHistoryRangeTooLong)
	}
}

func TestParseHealthHistoryRange_ExactlyMaxRangeAccepted(t *testing.T) {
	// 365 inclusive days (2026 is not a leap year) - exactly at the cap.
	req := httptest.NewRequest("GET", "/?from=2026-01-01&to=2026-12-31", nil)
	_, _, fields := parseHealthHistoryRange(req)
	if len(fields) != 0 {
		t.Fatalf("fields = %v, want none (365 points is exactly the cap)", fields)
	}
}

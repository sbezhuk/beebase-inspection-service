package inspection

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

const validInspectedAt = "2026-03-15T09:00:00Z"

func TestCreateRequest_Validate(t *testing.T) {
	validHiveID := uuid.New().String()

	tests := []struct {
		name string
		req  CreateRequest
		want map[string]string
	}{
		{
			name: "valid",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "all good", Type: "ROUTINE"},
			want: map[string]string{},
		},
		{
			name: "missing hive_id",
			req:  CreateRequest{HiveID: "", InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"hive_id": CodeHiveIDRequired},
		},
		{
			name: "malformed hive_id",
			req:  CreateRequest{HiveID: "not-a-uuid", InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"hive_id": CodeHiveIDInvalid},
		},
		{
			name: "missing inspected_at",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: "", Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"inspected_at": CodeInspectedAtRequired},
		},
		{
			name: "malformed inspected_at",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: "not-a-date", Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"inspected_at": CodeInspectedAtInvalid},
		},
		{
			name: "missing notes",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "", Type: "ROUTINE"},
			want: map[string]string{"notes": CodeNotesRequired},
		},
		{
			name: "missing type",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "ok", Type: ""},
			want: map[string]string{"type": CodeTypeRequired},
		},
		{
			name: "invalid type",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "ok", Type: "swarm"},
			want: map[string]string{"type": CodeTypeInvalid},
		},
		{
			name: "everything wrong at once",
			req:  CreateRequest{HiveID: "bad", InspectedAt: "", Notes: "", Type: "bad"},
			want: map[string]string{
				"hive_id":      CodeHiveIDInvalid,
				"inspected_at": CodeInspectedAtRequired,
				"notes":        CodeNotesRequired,
				"type":         CodeTypeInvalid,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Validate()
			if len(got) != len(tt.want) {
				t.Fatalf("Validate() = %v, want %v", got, tt.want)
			}
			for field, wantCode := range tt.want {
				if gotCode, ok := got[field]; !ok || gotCode != wantCode {
					t.Errorf("field %q: got code %q, want %q", field, gotCode, wantCode)
				}
			}
		})
	}
}

func TestCreateRequest_Validate_Images(t *testing.T) {
	validHiveID := uuid.New().String()

	if fields := (&CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE", Images: nil}).Validate(); len(fields) != 0 {
		t.Errorf("nil images: expected no errors, got %v", fields)
	}
	if fields := (&CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE", Images: []string{}}).Validate(); len(fields) != 0 {
		t.Errorf("empty images: expected no errors, got %v", fields)
	}
	if fields := (&CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE", Images: []string{uuid.New().String()}}).Validate(); len(fields) != 0 {
		t.Errorf("valid image id: expected no errors, got %v", fields)
	}

	fields := (&CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE", Images: []string{"not-a-uuid"}}).Validate()
	if code := fields["images"]; code != CodeImagesInvalid {
		t.Errorf("images code = %q, want %q", code, CodeImagesInvalid)
	}
}

func TestUpdateRequest_Validate(t *testing.T) {
	if fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: "QUEEN"}).Validate(); len(fields) != 0 {
		t.Errorf("expected no errors, got %v", fields)
	}

	fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "", Type: "QUEEN"}).Validate()
	if code := fields["notes"]; code != CodeNotesRequired {
		t.Errorf("notes code = %q, want %q", code, CodeNotesRequired)
	}

	fields = (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: ""}).Validate()
	if code := fields["type"]; code != CodeTypeRequired {
		t.Errorf("type code = %q, want %q", code, CodeTypeRequired)
	}

	fields = (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: "not-a-type"}).Validate()
	if code := fields["type"]; code != CodeTypeInvalid {
		t.Errorf("type code = %q, want %q", code, CodeTypeInvalid)
	}
}

func TestUpdateRequest_Validate_Images(t *testing.T) {
	if fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: "QUEEN", Images: nil}).Validate(); len(fields) != 0 {
		t.Errorf("nil images: expected no errors, got %v", fields)
	}
	if fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: "QUEEN", Images: []string{}}).Validate(); len(fields) != 0 {
		t.Errorf("empty images: expected no errors, got %v", fields)
	}
	if fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: "QUEEN", Images: []string{uuid.New().String()}}).Validate(); len(fields) != 0 {
		t.Errorf("valid image id: expected no errors, got %v", fields)
	}

	fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok", Type: "QUEEN", Images: []string{"not-a-uuid"}}).Validate()
	if code := fields["images"]; code != CodeImagesInvalid {
		t.Errorf("images code = %q, want %q", code, CodeImagesInvalid)
	}
}

func TestParseSearch(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	cases := []struct {
		name       string
		query      string
		wantSearch *string
		wantCode   string
	}{
		{
			name:       "omitted",
			query:      "",
			wantSearch: nil,
		},
		{
			name:       "empty",
			query:      "search=",
			wantSearch: nil,
		},
		{
			name:     "one char",
			query:    "search=a",
			wantCode: CodeInvalidSearch,
		},
		{
			name:     "two chars",
			query:    "search=ab",
			wantCode: CodeInvalidSearch,
		},
		{
			name:       "three chars",
			query:      "search=abc",
			wantSearch: strPtr("abc"),
		},
		{
			name:       "long search",
			query:      "search=queen%20seen",
			wantSearch: strPtr("queen seen"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
			s, fields := parseSearch(req, nil)
			if tc.wantCode != "" {
				if fields["search"] != tc.wantCode {
					t.Fatalf("fields[search] = %q, want %q", fields["search"], tc.wantCode)
				}
				if s != nil {
					t.Fatalf("search = %v, want nil", s)
				}
			} else {
				if len(fields) != 0 {
					t.Fatalf("unexpected fields: %v", fields)
				}
				if tc.wantSearch == nil && s != nil {
					t.Fatalf("search = %v, want nil", s)
				}
				if tc.wantSearch != nil {
					if s == nil || *s != *tc.wantSearch {
						t.Fatalf("search = %v, want %v", s, *tc.wantSearch)
					}
				}
			}
		})
	}
}

func TestParseType(t *testing.T) {
	type testCase struct {
		name     string
		query    string
		wantType *inspection.Type
		wantCode string
	}

	cases := []testCase{
		{name: "omitted", query: ""},
		{name: "empty", query: "type="},
		{name: "invalid value", query: "type=SWARM", wantCode: CodeTypeInvalid},
		{name: "lowercase not accepted", query: "type=queen", wantCode: CodeTypeInvalid},
	}
	for _, typ := range inspection.Types {
		typ := typ
		cases = append(cases, testCase{name: string(typ), query: "type=" + string(typ), wantType: &typ})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
			typ, fields := parseType(req, nil)
			if tc.wantCode != "" {
				if fields["type"] != tc.wantCode {
					t.Fatalf("fields[type] = %q, want %q", fields["type"], tc.wantCode)
				}
				if typ != nil {
					t.Fatalf("type = %v, want nil", typ)
				}
				return
			}
			if len(fields) != 0 {
				t.Fatalf("unexpected fields: %v", fields)
			}
			if tc.wantType == nil {
				if typ != nil {
					t.Fatalf("type = %v, want nil", typ)
				}
				return
			}
			if typ == nil || *typ != *tc.wantType {
				t.Fatalf("type = %v, want %v", typ, *tc.wantType)
			}
		})
	}
}

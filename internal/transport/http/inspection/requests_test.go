package inspection

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

const validInspectedAt = "2026-03-15"

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
			want: map[string]string{"hiveId": CodeHiveIDRequired},
		},
		{
			name: "malformed hive_id",
			req:  CreateRequest{HiveID: "not-a-uuid", InspectedAt: validInspectedAt, Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"hiveId": CodeHiveIDInvalid},
		},
		{
			name: "missing inspected_at",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: "", Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"inspectedAt": CodeInspectedAtRequired},
		},
		{
			name: "malformed inspected_at",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: "not-a-date", Notes: "ok", Type: "ROUTINE"},
			want: map[string]string{"inspectedAt": CodeInspectedAtInvalid},
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
				"hiveId":      CodeHiveIDInvalid,
				"inspectedAt": CodeInspectedAtRequired,
				"notes":       CodeNotesRequired,
				"type":        CodeTypeInvalid,
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

func TestCreateRequest_Validate_RejectsForeignAssessmentFields(t *testing.T) {
	stringPtr := func(value string) *string { return &value }
	validHiveID := uuid.New().String()
	cases := []struct {
		name       string
		typ        string
		assessment AssessmentRequest
	}{
		{
			name: "ROUTINE with HEALTH field",
			typ:  "ROUTINE",
			assessment: AssessmentRequest{
				ColonyStrength:     stringPtr("STRONG"),
				HealthConcernLevel: stringPtr("HIGH"),
			},
		},
		{
			name: "QUEEN with FEEDING field",
			typ:  "QUEEN",
			assessment: AssessmentRequest{
				QueenObserved:    stringPtr("OBSERVED"),
				FeedingPerformed: stringPtr("YES"),
			},
		},
		{
			name: "BROOD with SEASONAL field",
			typ:  "BROOD",
			assessment: AssessmentRequest{
				BroodPattern: stringPtr("SOLID"),
				Season:       stringPtr("WINTER"),
			},
		},
		{
			name: "HEALTH with QUEEN field",
			typ:  "HEALTH",
			assessment: AssessmentRequest{
				HealthOverallCondition: stringPtr("GOOD"),
				QueenObserved:          stringPtr("OBSERVED"),
			},
		},
		{
			name: "FEEDING with BROOD field",
			typ:  "FEEDING",
			assessment: AssessmentRequest{
				FeedingPerformed: stringPtr("NO"),
				BroodPattern:     stringPtr("SOLID"),
			},
		},
		{
			name: "SEASONAL with HEALTH field",
			typ:  "SEASONAL",
			assessment: AssessmentRequest{
				Season:                 stringPtr("SPRING"),
				SeasonalReadiness:      stringPtr("READY"),
				HealthOverallCondition: stringPtr("GOOD"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateRequest{
				HiveID:      validHiveID,
				InspectedAt: validInspectedAt,
				Notes:       "assessment",
				Type:        tc.typ,
				Assessment:  &tc.assessment,
			}
			fields := req.Validate()
			if fields["assessment"] != CodeAssessmentInvalid {
				t.Fatalf("assessment validation = %q, want %q; fields=%v", fields["assessment"], CodeAssessmentInvalid, fields)
			}
		})
	}
}

func TestCreateRequest_JSONForeignFieldIsNotSilentlyDiscarded(t *testing.T) {
	var req CreateRequest
	if err := json.Unmarshal([]byte(`{
		"hiveId":"00000000-0000-0000-0000-000000000001",
		"inspectedAt":"2026-03-15",
		"notes":"assessment",
		"type":"ROUTINE",
		"assessment":{"version":1,"colonyStrength":"STRONG","healthConcernLevel":"HIGH"}
	}`), &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if req.Assessment == nil || req.Assessment.HealthConcernLevel == nil {
		t.Fatal("foreign assessment field was discarded during decoding")
	}
	if fields := req.Validate(); fields["assessment"] != CodeAssessmentInvalid {
		t.Fatalf("assessment validation = %q, want %q; fields=%v", fields["assessment"], CodeAssessmentInvalid, fields)
	}
}

func TestCreateRequest_Validate_RejectsFlutterPayloadAfterTypeChange(t *testing.T) {
	var req CreateRequest
	err := json.Unmarshal([]byte(`{
		"hiveId":"00000000-0000-0000-0000-000000000001",
		"inspectedAt":"2026-09-18",
		"notes":"Changed type",
		"type":"QUEEN",
		"assessment":{
			"version":1,
			"colonyStrength":"STRONG",
			"queenStatus":"HEALTHY",
			"broodStatus":"HEALTHY",
			"foodStores":"ADEQUATE",
			"healthConcerns":"NONE"
		}
	}`), &req)
	if err != nil {
		t.Fatalf("decode Flutter payload: %v", err)
	}

	fields := req.Validate()
	if fields["assessment"] != CodeAssessmentInvalid {
		t.Fatalf("assessment validation = %q, want %q; fields=%v", fields["assessment"], CodeAssessmentInvalid, fields)
	}
}

func TestCreateRequest_Validate_AcceptsFullyCompletedFlutterAssessments(t *testing.T) {
	cases := map[string]string{
		"ROUTINE": `{"colonyStrength":"STRONG","queenStatus":"HEALTHY","broodStatus":"HEALTHY","foodStores":"ADEQUATE","healthConcerns":"NONE"}`,
		"QUEEN": `{"queenObserved":"OBSERVED","eggsObserved":"YES","queenCells":"NONE","queenCondition":"NORMAL"}`,
		"BROOD": `{"broodAmount":"MODERATE","broodPattern":"SOLID","broodStages":["EGGS","LARVAE","CAPPED"],"broodConcerns":"NONE"}`,
		"HEALTH": `{"healthOverallCondition":"GOOD","pestSigns":["VARROA_MITES"],"healthWarningSigns":["ABNORMAL_BROOD"],"healthConcernLevel":"LOW"}`,
		"FEEDING": `{"foodStores":"LOW","feedingNeed":"YES","feedingPerformed":"YES","feedTypes":["SUGAR_SYRUP","POLLEN_SUBSTITUTE"]}`,
		"SEASONAL": `{"season":"AUTUMN","colonyStrength":"STRONG","seasonalStoreReadiness":"SUFFICIENT","seasonalReadiness":"READY","seasonalConcerns":["FOOD_STORES"]}`,
	}

	for typ, assessment := range cases {
		t.Run(typ, func(t *testing.T) {
			body := []byte(`{"hiveId":"00000000-0000-0000-0000-000000000001","inspectedAt":"2026-09-18","notes":"Complete assessment","type":"` + typ + `","assessment":` + assessment + `}`)
			var req CreateRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("decode Flutter payload: %v", err)
			}
			if fields := req.Validate(); len(fields) != 0 {
				t.Fatalf("Validate() = %v, want no errors", fields)
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

func TestParseDateFilter(t *testing.T) {
	utcDate := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	timePtr := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		name         string
		query        string
		wantDateFrom *time.Time
		wantDateTo   *time.Time
		wantFields   map[string]string
	}{
		{name: "omitted"},
		{
			name:         "date_from only",
			query:        "dateFrom=2026-01-01",
			wantDateFrom: timePtr(utcDate(2026, 1, 1)),
		},
		{
			// date_to is returned as the exclusive start of the next day.
			name:       "date_to only",
			query:      "dateTo=2026-09-13",
			wantDateTo: timePtr(utcDate(2026, 9, 14)),
		},
		{
			name:         "both",
			query:        "dateFrom=2026-01-01&dateTo=2026-09-13",
			wantDateFrom: timePtr(utcDate(2026, 1, 1)),
			wantDateTo:   timePtr(utcDate(2026, 9, 14)),
		},
		{
			name:         "exact boundary: date_from equals date_to",
			query:        "dateFrom=2026-09-13&dateTo=2026-09-13",
			wantDateFrom: timePtr(utcDate(2026, 9, 13)),
			wantDateTo:   timePtr(utcDate(2026, 9, 14)),
		},
		{
			name:       "invalid date_from format",
			query:      "dateFrom=2026/01/01",
			wantFields: map[string]string{"dateFrom": CodeInvalidDateFrom},
		},
		{
			name:       "invalid date_to format",
			query:      "dateTo=13-09-2026",
			wantFields: map[string]string{"dateTo": CodeInvalidDateTo},
		},
		{
			name:       "date_from is a full timestamp, not a date",
			query:      "dateFrom=2026-01-01T00:00:00Z",
			wantFields: map[string]string{"dateFrom": CodeInvalidDateFrom},
		},
		{
			name:       "date_from after date_to",
			query:      "dateFrom=2026-09-14&dateTo=2026-09-13",
			wantFields: map[string]string{"dateTo": CodeInvalidDateRange},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
			dateFrom, dateTo, fields := parseDateFilter(req, nil)
			if len(tc.wantFields) > 0 {
				for field, code := range tc.wantFields {
					if fields[field] != code {
						t.Fatalf("fields[%q] = %q, want %q", field, fields[field], code)
					}
				}
				return
			}
			if len(fields) != 0 {
				t.Fatalf("unexpected fields: %v", fields)
			}
			if tc.wantDateFrom == nil {
				if dateFrom != nil {
					t.Fatalf("dateFrom = %v, want nil", *dateFrom)
				}
			} else if dateFrom == nil || !dateFrom.Equal(*tc.wantDateFrom) {
				t.Fatalf("dateFrom = %v, want %v", dateFrom, *tc.wantDateFrom)
			}
			if tc.wantDateTo == nil {
				if dateTo != nil {
					t.Fatalf("dateTo = %v, want nil", *dateTo)
				}
			} else if dateTo == nil || !dateTo.Equal(*tc.wantDateTo) {
				t.Fatalf("dateTo = %v, want %v", dateTo, *tc.wantDateTo)
			}
		})
	}
}

func TestParseSortOrder(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	cases := []struct {
		name          string
		query         string
		wantSortOrder *string
		wantCode      string
	}{
		{
			name:          "omitted",
			query:         "",
			wantSortOrder: nil,
		},
		{
			name:          "asc",
			query:         "sortOrder=asc",
			wantSortOrder: strPtr("asc"),
		},
		{
			name:          "desc",
			query:         "sortOrder=desc",
			wantSortOrder: strPtr("desc"),
		},
		{
			name:     "invalid",
			query:    "sortOrder=newest",
			wantCode: CodeInvalidSortOrder,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
			s, fields := parseSortOrder(req, nil)
			if tc.wantCode != "" {
				if fields["sortOrder"] != tc.wantCode {
					t.Fatalf("fields[sortOrder] = %q, want %q", fields["sortOrder"], tc.wantCode)
				}
				if s != nil {
					t.Fatalf("sortOrder = %v, want nil", s)
				}
				return
			}
			if len(fields) != 0 {
				t.Fatalf("unexpected fields: %v", fields)
			}
			if tc.wantSortOrder == nil && s != nil {
				t.Fatalf("sortOrder = %v, want nil", s)
			}
			if tc.wantSortOrder != nil {
				if s == nil || *s != *tc.wantSortOrder {
					t.Fatalf("sortOrder = %v, want %v", s, *tc.wantSortOrder)
				}
			}
		})
	}
}

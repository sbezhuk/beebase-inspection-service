package inspection

import (
	"testing"

	"github.com/google/uuid"
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
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: "all good"},
			want: map[string]string{},
		},
		{
			name: "missing hive_id",
			req:  CreateRequest{HiveID: "", InspectedAt: validInspectedAt, Notes: "ok"},
			want: map[string]string{"hive_id": CodeHiveIDRequired},
		},
		{
			name: "malformed hive_id",
			req:  CreateRequest{HiveID: "not-a-uuid", InspectedAt: validInspectedAt, Notes: "ok"},
			want: map[string]string{"hive_id": CodeHiveIDInvalid},
		},
		{
			name: "missing inspected_at",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: "", Notes: "ok"},
			want: map[string]string{"inspected_at": CodeInspectedAtRequired},
		},
		{
			name: "malformed inspected_at",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: "not-a-date", Notes: "ok"},
			want: map[string]string{"inspected_at": CodeInspectedAtInvalid},
		},
		{
			name: "missing notes",
			req:  CreateRequest{HiveID: validHiveID, InspectedAt: validInspectedAt, Notes: ""},
			want: map[string]string{"notes": CodeNotesRequired},
		},
		{
			name: "everything wrong at once",
			req:  CreateRequest{HiveID: "bad", InspectedAt: "", Notes: ""},
			want: map[string]string{
				"hive_id":      CodeHiveIDInvalid,
				"inspected_at": CodeInspectedAtRequired,
				"notes":        CodeNotesRequired,
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

func TestUpdateRequest_Validate(t *testing.T) {
	if fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: "ok"}).Validate(); len(fields) != 0 {
		t.Errorf("expected no errors, got %v", fields)
	}

	fields := (&UpdateRequest{InspectedAt: validInspectedAt, Notes: ""}).Validate()
	if code := fields["notes"]; code != CodeNotesRequired {
		t.Errorf("notes code = %q, want %q", code, CodeNotesRequired)
	}
}

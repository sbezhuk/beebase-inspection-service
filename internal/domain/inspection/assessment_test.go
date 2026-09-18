package inspection

import "testing"

func TestAssessmentValidation(t *testing.T) {
	queen := QueenStatusProblem
	assessment := &Assessment{QueenStatus: &queen}
	if err := assessment.ValidateFor(TypeRoutine); err != nil {
		t.Fatalf("valid partial routine assessment: %v", err)
	}
	if assessment.Version != AssessmentVersion {
		t.Fatalf("version = %d, want %d", assessment.Version, AssessmentVersion)
	}

	invalid := QueenStatus("ABSENT")
	if err := (&Assessment{QueenStatus: &invalid}).ValidateFor(TypeRoutine); err == nil {
		t.Fatal("invalid enum accepted")
	}
	if err := (&Assessment{}).ValidateFor(TypeHealth); err == nil {
		t.Fatal("assessment accepted for unsupported inspection type")
	}
}

package inspection_test

// This file covers transitive parent-hive writability enforcement: an
// inspection can only be created or updated when hive-service currently
// reports its hive as writable. See application/inspection.Service.Create
// and Service.Update, and HiveVerifier.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
)

// TestCreate_ReadOnlyHive_Rejected proves creating an inspection under a
// hive hive-service reports as read-only (whether because the hive itself
// or its parent apiary is outside Free entitlement) is rejected with
// ErrHiveReadOnly, distinct from ErrHiveNotFound.
func TestCreate_ReadOnlyHive_Rejected(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	verifier.lock(hiveID)

	_, err := svc.Create(context.Background(), userID, "token", appinspection.CreateInput{HiveID: hiveID})
	if !errors.Is(err, appinspection.ErrHiveReadOnly) {
		t.Fatalf("Create under a read-only hive: got %v, want ErrHiveReadOnly", err)
	}
}

// TestUpdate_ReadOnlyHive_Rejected is the fix for the gap the
// investigation found: Update did not previously re-verify the parent
// hive at all, meaning a Free user could keep editing inspections under a
// hive that had since become read-only. Update must now reject it, and
// leave the inspection's stored fields untouched.
func TestUpdate_ReadOnlyHive_Rejected(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	created, err := svc.Create(context.Background(), userID, "token", appinspection.CreateInput{HiveID: hiveID, Notes: "original"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The hive becomes read-only after creation (e.g. Pro expired and this
	// hive fell outside the new Free entitlement).
	verifier.lock(hiveID)

	_, err = svc.Update(context.Background(), userID, "token", created.ID, appinspection.UpdateInput{Notes: "hijacked"})
	if !errors.Is(err, appinspection.ErrHiveReadOnly) {
		t.Fatalf("Update under a now-read-only hive: got %v, want ErrHiveReadOnly", err)
	}

	got, err := svc.Get(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Notes != "original" {
		t.Errorf("Notes = %q after rejected update, want unchanged %q", got.Notes, "original")
	}
}

// TestUpdate_HiveBecomesWritableAgain_UpdateSucceeds proves a promoted or
// re-upgraded hive immediately unblocks inspection updates again, with no
// special handling needed on this side - it's just what the next Verify
// call reports.
func TestUpdate_HiveBecomesWritableAgain_UpdateSucceeds(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	created, err := svc.Create(context.Background(), userID, "token", appinspection.CreateInput{HiveID: hiveID, Notes: "original"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	verifier.lock(hiveID)

	_, err = svc.Update(context.Background(), userID, "token", created.ID, appinspection.UpdateInput{Notes: "still locked"})
	if !errors.Is(err, appinspection.ErrHiveReadOnly) {
		t.Fatalf("Update while locked: got %v, want ErrHiveReadOnly", err)
	}

	// Re-upgrade / promotion: hive-service now reports it writable.
	verifier.readOnly[hiveID] = false

	if _, err := svc.Update(context.Background(), userID, "token", created.ID, appinspection.UpdateInput{Notes: "editable again"}); err != nil {
		t.Fatalf("Update after hive becomes writable again: %v", err)
	}
}

// TestDelete_ReadOnlyHive_StillAllowed proves delete is never gated by
// hive writability - a Free user can always delete historical inspections
// under a locked hive.
func TestDelete_ReadOnlyHive_StillAllowed(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	created, err := svc.Create(context.Background(), userID, "token", appinspection.CreateInput{HiveID: hiveID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	verifier.lock(hiveID)

	if err := svc.Delete(context.Background(), userID, created.ID); err != nil {
		t.Fatalf("Delete under a read-only hive should still be allowed: %v", err)
	}
}

// TestGet_ReadOnlyHive_StillReadable proves reads are never gated by
// writability - only creates/updates are.
func TestGet_ReadOnlyHive_StillReadable(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	created, err := svc.Create(context.Background(), userID, "token", appinspection.CreateInput{HiveID: hiveID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	verifier.lock(hiveID)

	if _, err := svc.Get(context.Background(), userID, created.ID); err != nil {
		t.Fatalf("Get under a read-only hive should still succeed: %v", err)
	}
}

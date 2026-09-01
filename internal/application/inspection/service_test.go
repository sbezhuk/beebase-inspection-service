package inspection_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/pagination"
	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// --- in-memory fake repository ---

type fakeRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*inspection.Inspection
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[uuid.UUID]*inspection.Inspection{}}
}

func (f *fakeRepo) Create(_ context.Context, i *inspection.Inspection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *i
	f.byID[i.ID] = &cp
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, userID, inspectionID uuid.UUID) (*inspection.Inspection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.byID[inspectionID]
	if !ok || i.UserID != userID || i.DeletedAt != nil {
		return nil, inspection.ErrNotFound
	}
	cp := *i
	return &cp, nil
}

func (f *fakeRepo) ListByHive(_ context.Context, userID, hiveID uuid.UUID, p pagination.Params) ([]*inspection.Inspection, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []*inspection.Inspection
	for _, i := range f.byID {
		if i.UserID == userID && i.HiveID == hiveID && i.DeletedAt == nil {
			cp := *i
			all = append(all, &cp)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].InspectedAt.Equal(all[j].InspectedAt) {
			return all[i].InspectedAt.Before(all[j].InspectedAt)
		}
		return all[i].ID.String() < all[j].ID.String()
	})

	total := len(all)
	start := p.Offset()
	if start > total {
		start = total
	}
	end := start + p.Limit
	if end > total {
		end = total
	}

	return all[start:end], total, nil
}

func (f *fakeRepo) Update(_ context.Context, i *inspection.Inspection) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.byID[i.ID]
	if !ok || existing.UserID != i.UserID || existing.DeletedAt != nil {
		return inspection.ErrNotFound
	}
	cp := *i
	f.byID[i.ID] = &cp
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, userID, inspectionID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.byID[inspectionID]
	if !ok || i.UserID != userID || i.DeletedAt != nil {
		return inspection.ErrNotFound
	}
	now := i.UpdatedAt
	i.DeletedAt = &now
	return nil
}

// --- fake hive verifier ---

// fakeHiveVerifier simulates hive-service: a set of (token, hiveID) pairs
// are "owned", everything else is rejected exactly like a 404 from the
// real service would be.
type fakeHiveVerifier struct {
	owned map[string]uuid.UUID // token -> the one hive it owns
}

func newFakeHiveVerifier() *fakeHiveVerifier {
	return &fakeHiveVerifier{owned: map[string]uuid.UUID{}}
}

func (f *fakeHiveVerifier) allow(token string, hiveID uuid.UUID) {
	f.owned[token] = hiveID
}

func (f *fakeHiveVerifier) Verify(_ context.Context, accessToken string, hiveID uuid.UUID) error {
	if owned, ok := f.owned[accessToken]; ok && owned == hiveID {
		return nil
	}
	return appinspection.ErrHiveNotFound
}

// --- tests ---

func inspectedAt() time.Time {
	return time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
}

func TestCreate_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "user-token"
	verifier.allow(token, hiveID)

	i, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "queen seen, brood pattern good",
		Type:        inspection.TypeQueen,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if i.UserID != userID {
		t.Errorf("UserID = %s, want %s", i.UserID, userID)
	}
	if i.HiveID != hiveID {
		t.Errorf("HiveID = %s, want %s", i.HiveID, hiveID)
	}
	if i.Type != inspection.TypeQueen {
		t.Errorf("Type = %q, want %q", i.Type, inspection.TypeQueen)
	}
}

// TestCreate_HiveNotOwnedByCaller is the core cross-service security
// guarantee: an inspection can't be created under a hive the caller
// doesn't own (and, transitively, an apiary they don't own), even if
// they know its ID.
func TestCreate_HiveNotOwnedByCaller(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	someoneElsesHive := uuid.New()
	// Deliberately not calling verifier.allow for this token/hive pair.

	_, err := svc.Create(context.Background(), uuid.New(), "attacker-token", appinspection.CreateInput{
		HiveID:      someoneElsesHive,
		InspectedAt: inspectedAt(),
		Notes:       "snooping",
		Type:        inspection.TypeRoutine,
	})
	if !errors.Is(err, appinspection.ErrHiveNotFound) {
		t.Fatalf("Create under unowned hive: got %v, want ErrHiveNotFound", err)
	}
}

func TestCreate_UnknownHive(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)

	_, err := svc.Create(context.Background(), uuid.New(), "some-token", appinspection.CreateInput{
		HiveID:      uuid.New(),
		InspectedAt: inspectedAt(),
		Notes:       "n/a",
		Type:        inspection.TypeRoutine,
	})
	if !errors.Is(err, appinspection.ErrHiveNotFound) {
		t.Fatalf("Create under unknown hive: got %v, want ErrHiveNotFound", err)
	}
}

func TestGet_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeHealth,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("Get returned %s, want %s", got.ID, created.ID)
	}
	if got.Type != inspection.TypeHealth {
		t.Errorf("Type = %q, want %q", got.Type, inspection.TypeHealth)
	}
}

func TestGet_NotFound(t *testing.T) {
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier())

	_, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Get with unknown id: got %v, want ErrNotFound", err)
	}
}

// TestGet_WrongOwner_ReturnsNotFound proves ownership is enforced on
// every subsequent read too, not just at creation time.
func TestGet_WrongOwner_ReturnsNotFound(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()
	token := "owner-token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), owner, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "owner's inspection", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Get(context.Background(), other, created.ID)
	if !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Get by non-owner: got %v, want ErrNotFound", err)
	}
}

func TestListByHive_ReturnsOnlyOwnInspectionsForThatHive(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	userA := uuid.New()
	userB := uuid.New()
	hiveA1 := uuid.New()
	hiveB := uuid.New()
	tokenA := "token-a"
	tokenB := "token-b"
	verifier.allow(tokenA, hiveA1)
	verifier.allow(tokenB, hiveB)

	for _, notes := range []string{"first", "second"} {
		if _, err := svc.Create(context.Background(), userA, tokenA, appinspection.CreateInput{
			HiveID: hiveA1, InspectedAt: inspectedAt(), Notes: notes, Type: inspection.TypeRoutine,
		}); err != nil {
			t.Fatalf("create %s: %v", notes, err)
		}
	}
	if _, err := svc.Create(context.Background(), userB, tokenB, appinspection.CreateInput{
		HiveID: hiveB, InspectedAt: inspectedAt(), Notes: "userB's", Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("create userB's: %v", err)
	}

	list, total, err := svc.ListByHive(context.Background(), userA, hiveA1, pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
	if err != nil {
		t.Fatalf("ListByHive: %v", err)
	}
	if total != 2 {
		t.Fatalf("ListByHive total = %d, want 2", total)
	}
	if len(list) != 2 {
		t.Fatalf("ListByHive returned %d inspections, want 2", len(list))
	}
	for _, i := range list {
		if i.UserID != userA || i.HiveID != hiveA1 {
			t.Errorf("ListByHive leaked inspection %s (user %s, hive %s)", i.ID, i.UserID, i.HiveID)
		}
	}
}

func TestListByHive_Pagination(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	for i := 0; i < 5; i++ {
		if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
			HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	firstPage, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 1, Limit: 2})
	if err != nil {
		t.Fatalf("ListByHive page 1: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(firstPage) != 2 {
		t.Fatalf("page 1 returned %d inspections, want 2", len(firstPage))
	}

	lastPage, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 3, Limit: 2})
	if err != nil {
		t.Fatalf("ListByHive page 3: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(lastPage) != 1 {
		t.Fatalf("page 3 returned %d inspections, want 1", len(lastPage))
	}

	beyond, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 10, Limit: 2})
	if err != nil {
		t.Fatalf("ListByHive page 10: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(beyond) != 0 {
		t.Fatalf("page beyond available data returned %d inspections, want 0", len(beyond))
	}
}

func TestListByHive_Empty(t *testing.T) {
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier())

	list, total, err := svc.ListByHive(context.Background(), uuid.New(), uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
	if err != nil {
		t.Fatalf("ListByHive: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	if len(list) != 0 {
		t.Fatalf("ListByHive = %v, want empty", list)
	}
}

func TestListByHive_OtherUsersHiveReturnsEmpty(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()
	token := "owner-token"
	verifier.allow(token, hiveID)

	if _, err := svc.Create(context.Background(), owner, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "owner's", Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, _, err := svc.ListByHive(context.Background(), other, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
	if err != nil {
		t.Fatalf("ListByHive: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListByHive by non-owner = %v, want empty", list)
	}
}

func TestUpdate_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "old notes", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newTime := inspectedAt().Add(24 * time.Hour)
	updated, err := svc.Update(context.Background(), userID, created.ID, appinspection.UpdateInput{
		InspectedAt: newTime,
		Notes:       "new notes",
		Type:        inspection.TypeQueen,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Notes != "new notes" {
		t.Errorf("Notes = %q, want %q", updated.Notes, "new notes")
	}
	if updated.Type != inspection.TypeQueen {
		t.Errorf("Type = %q, want %q", updated.Type, inspection.TypeQueen)
	}
	if !updated.InspectedAt.Equal(newTime) {
		t.Errorf("InspectedAt = %v, want %v", updated.InspectedAt, newTime)
	}
	if updated.HiveID != hiveID {
		t.Errorf("HiveID changed to %s, want unchanged %s", updated.HiveID, hiveID)
	}
}

func TestUpdate_WrongOwner_ReturnsNotFound(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()
	token := "owner-token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), owner, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "owner's", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Update(context.Background(), other, created.ID, appinspection.UpdateInput{Notes: "hijacked", Type: inspection.TypeRoutine})
	if !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Update by non-owner: got %v, want ErrNotFound", err)
	}

	got, err := svc.Get(context.Background(), owner, created.ID)
	if err != nil {
		t.Fatalf("Get after failed hijack attempt: %v", err)
	}
	if got.Notes != "owner's" {
		t.Errorf("Notes = %q after failed hijack attempt, want unchanged", got.Notes)
	}
}

func TestDelete_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "gone soon", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(context.Background(), userID, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Get(context.Background(), userID, created.ID); !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestDelete_WrongOwner_ReturnsNotFoundAndDoesNotDelete(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()
	token := "owner-token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), owner, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "owner's", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(context.Background(), other, created.ID); !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Delete by non-owner: got %v, want ErrNotFound", err)
	}

	if _, err := svc.Get(context.Background(), owner, created.ID); err != nil {
		t.Fatalf("owner's inspection should survive a failed delete attempt by another user: %v", err)
	}
}

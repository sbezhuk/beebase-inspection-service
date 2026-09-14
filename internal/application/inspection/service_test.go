package inspection_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/inspectionwarning"
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

// matchesSearch mirrors the real repository's case-insensitive substring
// match of search against notes; a nil search matches everything.
func matchesSearch(i *inspection.Inspection, search *string) bool {
	if search == nil {
		return true
	}
	return strings.Contains(strings.ToLower(i.Notes), strings.ToLower(*search))
}

// sortInspections mirrors the real repository's ORDER BY: by default,
// InspectedAt ascending; when sortOrder is non-nil ("asc"/"desc"), by
// CreatedAt in that direction instead. Either way id is the tiebreaker,
// tied to the same direction as the primary sort.
func sortInspections(all []*inspection.Inspection, sortOrder *string) {
	if sortOrder == nil {
		sort.Slice(all, func(i, j int) bool {
			if !all[i].InspectedAt.Equal(all[j].InspectedAt) {
				return all[i].InspectedAt.Before(all[j].InspectedAt)
			}
			return all[i].ID.String() < all[j].ID.String()
		})
		return
	}
	desc := *sortOrder == "desc"
	sort.Slice(all, func(i, j int) bool {
		if desc {
			i, j = j, i
		}
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.Before(all[j].CreatedAt)
		}
		return all[i].ID.String() < all[j].ID.String()
	})
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

func (f *fakeRepo) ListByHive(_ context.Context, userID, hiveID uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []*inspection.Inspection
	for _, i := range f.byID {
		if i.UserID == userID && i.HiveID == hiveID && i.DeletedAt == nil && (typ == nil || i.Type == *typ) && matchesSearch(i, search) &&
			(dateFrom == nil || !i.InspectedAt.Before(*dateFrom)) && (dateTo == nil || i.InspectedAt.Before(*dateTo)) {
			cp := *i
			all = append(all, &cp)
		}
	}
	sortInspections(all, sortOrder)

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

func (f *fakeRepo) ListByUser(_ context.Context, userID uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []*inspection.Inspection
	for _, i := range f.byID {
		if i.UserID == userID && i.DeletedAt == nil && (typ == nil || i.Type == *typ) && matchesSearch(i, search) &&
			(dateFrom == nil || !i.InspectedAt.Before(*dateFrom)) && (dateTo == nil || i.InspectedAt.Before(*dateTo)) {
			cp := *i
			all = append(all, &cp)
		}
	}
	sortInspections(all, sortOrder)

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

func (f *fakeRepo) DeleteByHive(_ context.Context, userID, hiveID uuid.UUID) ([]uuid.UUID, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int64
	var images []uuid.UUID
	for id, i := range f.byID {
		if i.UserID == userID && i.HiveID == hiveID {
			images = append(images, i.Images...)
			delete(f.byID, id)
			count++
		}
	}
	return images, count, nil
}

// LatestInspectedAtByHive mirrors the real repository's GROUP BY
// hive_id, MAX(inspected_at): only non-deleted inspections count, and a
// hive with none is simply absent from the result.
func (f *fakeRepo) LatestInspectedAtByHive(_ context.Context, userID uuid.UUID) (map[uuid.UUID]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[uuid.UUID]time.Time)
	for _, i := range f.byID {
		if i.UserID != userID || i.DeletedAt != nil {
			continue
		}
		if latest, ok := out[i.HiveID]; !ok || i.InspectedAt.After(latest) {
			out[i.HiveID] = i.InspectedAt
		}
	}
	return out, nil
}

// --- fake media client ---

// fakeMediaClient stands in for application/inspection.MediaClient:
// ownedIDs is the set of media ids VerifyOwnership will accept as
// belonging to the caller (media-service's own ownership scoping,
// simulated in-memory rather than by an HTTP round trip).
type fakeMediaClient struct {
	mu       sync.Mutex
	ownedIDs map[uuid.UUID]bool
	deleted  []uuid.UUID
}

func newFakeMediaClient() *fakeMediaClient {
	return &fakeMediaClient{ownedIDs: map[uuid.UUID]bool{}}
}

// own registers each of ids as belonging to the caller.
func (f *fakeMediaClient) own(ids ...uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		f.ownedIDs[id] = true
	}
}

func (f *fakeMediaClient) VerifyOwnership(_ context.Context, _ string, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		if !f.ownedIDs[id] {
			return appinspection.ErrImageNotFound
		}
	}
	return nil
}

func (f *fakeMediaClient) DeleteByIDs(_ context.Context, _ string, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, ids...)
	return nil
}

func (f *fakeMediaClient) wasDeleted(id uuid.UUID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.deleted {
		if d == id {
			return true
		}
	}
	return false
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)

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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), 14)

	_, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Get with unknown id: got %v, want ErrNotFound", err)
	}
}

// TestGet_WrongOwner_ReturnsNotFound proves ownership is enforced on
// every subsequent read too, not just at creation time.
func TestGet_WrongOwner_ReturnsNotFound(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

	list, total, err := svc.ListByHive(context.Background(), userA, hiveA1, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

	firstPage, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 1, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive page 1: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(firstPage) != 2 {
		t.Fatalf("page 1 returned %d inspections, want 2", len(firstPage))
	}

	lastPage, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 3, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive page 3: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(lastPage) != 1 {
		t.Fatalf("page 3 returned %d inspections, want 1", len(lastPage))
	}

	beyond, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 10, Limit: 2}, nil, nil, nil, nil, nil)
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
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), 14)

	list, total, err := svc.ListByHive(context.Background(), uuid.New(), uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

	list, _, err := svc.ListByHive(context.Background(), other, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListByHive by non-owner = %v, want empty", list)
	}
}

func TestList_ReturnsOnlyOwnInspectionsAcrossEveryHive(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
	userA := uuid.New()
	userB := uuid.New()
	hiveA1 := uuid.New()
	hiveA2 := uuid.New()
	hiveB := uuid.New()
	tokenA1 := "token-a1"
	tokenA2 := "token-a2"
	tokenB := "token-b"
	verifier.allow(tokenA1, hiveA1)
	verifier.allow(tokenA2, hiveA2)
	verifier.allow(tokenB, hiveB)

	if _, err := svc.Create(context.Background(), userA, tokenA1, appinspection.CreateInput{
		HiveID: hiveA1, InspectedAt: inspectedAt(), Notes: "hive 1", Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("create hive1: %v", err)
	}
	if _, err := svc.Create(context.Background(), userA, tokenA2, appinspection.CreateInput{
		HiveID: hiveA2, InspectedAt: inspectedAt(), Notes: "hive 2", Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("create hive2: %v", err)
	}
	if _, err := svc.Create(context.Background(), userB, tokenB, appinspection.CreateInput{
		HiveID: hiveB, InspectedAt: inspectedAt(), Notes: "userB's", Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("create userB's: %v", err)
	}

	list, total, err := svc.List(context.Background(), userA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Fatalf("List total = %d, want 2", total)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d inspections, want 2", len(list))
	}
	for _, i := range list {
		if i.UserID != userA {
			t.Errorf("List leaked inspection %s belonging to %s", i.ID, i.UserID)
		}
	}
}

func TestList_Pagination(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

	firstPage, total, err := svc.List(context.Background(), userID, pagination.Params{Page: 1, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(firstPage) != 2 {
		t.Fatalf("page 1 returned %d inspections, want 2", len(firstPage))
	}
}

func TestList_Empty(t *testing.T) {
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), 14)

	list, total, err := svc.List(context.Background(), uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	if len(list) != 0 {
		t.Fatalf("List = %v, want empty", list)
	}
}

// TestListByHive_FilterByType proves the optional type filter is forwarded
// to the repository and narrows results to the requested InspectionType,
// while an omitted (nil) type returns every type.
func TestListByHive_FilterByType(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	for _, typ := range inspection.Types {
		if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
			HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: typ,
		}); err != nil {
			t.Fatalf("create %s: %v", typ, err)
		}
	}

	queen := inspection.TypeQueen
	list, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, &queen, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive type=QUEEN: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].Type != inspection.TypeQueen {
		t.Fatalf("ListByHive type=QUEEN: total=%d list=%v, want a single QUEEN inspection", total, list)
	}

	all, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive without type filter: %v", err)
	}
	if total != len(inspection.Types) || len(all) != len(inspection.Types) {
		t.Fatalf("ListByHive without type filter: total=%d len=%d, want %d", total, len(all), len(inspection.Types))
	}
}

// TestList_FilterByType mirrors TestListByHive_FilterByType for the
// cross-hive List, also proving the filter respects ownership.
func TestList_FilterByType(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
	userA := uuid.New()
	userB := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()
	tokenA := "token-a"
	tokenB := "token-b"
	verifier.allow(tokenA, hiveA)
	verifier.allow(tokenB, hiveB)

	if _, err := svc.Create(context.Background(), userA, tokenA, appinspection.CreateInput{
		HiveID: hiveA, InspectedAt: inspectedAt(), Notes: "userA queen", Type: inspection.TypeQueen,
	}); err != nil {
		t.Fatalf("create userA queen: %v", err)
	}
	if _, err := svc.Create(context.Background(), userA, tokenA, appinspection.CreateInput{
		HiveID: hiveA, InspectedAt: inspectedAt(), Notes: "userA routine", Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("create userA routine: %v", err)
	}
	if _, err := svc.Create(context.Background(), userB, tokenB, appinspection.CreateInput{
		HiveID: hiveB, InspectedAt: inspectedAt(), Notes: "userB queen", Type: inspection.TypeQueen,
	}); err != nil {
		t.Fatalf("create userB queen: %v", err)
	}

	queen := inspection.TypeQueen
	list, total, err := svc.List(context.Background(), userA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, &queen, nil, nil, nil)
	if err != nil {
		t.Fatalf("List type=QUEEN: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].UserID != userA {
		t.Fatalf("List type=QUEEN: total=%d list=%v, want a single QUEEN inspection owned by userA", total, list)
	}
}

// TestListByHive_FilterByTypeCombinedWithSearchAndPagination proves type
// combines with search and pagination using AND semantics end to end
// through the service.
func TestListByHive_FilterByTypeCombinedWithSearchAndPagination(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	for i := 0; i < 3; i++ {
		if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
			HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "queen seen, healthy", Type: inspection.TypeQueen,
		}); err != nil {
			t.Fatalf("create matching %d: %v", i, err)
		}
	}
	// Wrong type, same notes: excluded by the type filter.
	if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "queen seen, healthy", Type: inspection.TypeBrood,
	}); err != nil {
		t.Fatalf("create wrong type: %v", err)
	}
	// Right type, wrong notes: excluded by the search filter.
	if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "nothing notable", Type: inspection.TypeQueen,
	}); err != nil {
		t.Fatalf("create wrong notes: %v", err)
	}

	queen := inspection.TypeQueen
	search := "healthy"
	page, total, err := svc.ListByHive(context.Background(), userID, hiveID, pagination.Params{Page: 1, Limit: 2}, &search, &queen, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive type+search+pagination: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (only the matching type+search rows)", total)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d, want 2", len(page))
	}
	for _, i := range page {
		if i.Type != inspection.TypeQueen || i.Notes != "queen seen, healthy" {
			t.Errorf("unexpected result %+v", i)
		}
	}
}

func TestUpdate_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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
	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

	_, err = svc.Update(context.Background(), other, "other-token", created.ID, appinspection.UpdateInput{Notes: "hijacked", Type: inspection.TypeRoutine})
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

func TestDeleteByHive_DeletesOnlyThatHivesInspections(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
	userID := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()
	tokenA := "token-a"
	tokenB := "token-b"
	verifier.allow(tokenA, hiveA)
	verifier.allow(tokenB, hiveB)

	for _, notes := range []string{"first", "second"} {
		if _, err := svc.Create(context.Background(), userID, tokenA, appinspection.CreateInput{
			HiveID: hiveA, InspectedAt: inspectedAt(), Notes: notes, Type: inspection.TypeRoutine,
		}); err != nil {
			t.Fatalf("create %s: %v", notes, err)
		}
	}
	keep, err := svc.Create(context.Background(), userID, tokenB, appinspection.CreateInput{
		HiveID: hiveB, InspectedAt: inspectedAt(), Notes: "other hive", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("create other hive's inspection: %v", err)
	}

	count, err := svc.DeleteByHive(context.Background(), userID, tokenA, hiveA)
	if err != nil {
		t.Fatalf("DeleteByHive: %v", err)
	}
	if count != 2 {
		t.Fatalf("DeleteByHive count = %d, want 2", count)
	}

	list, total, err := svc.ListByHive(context.Background(), userID, hiveA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("hiveA inspections survived DeleteByHive: total=%d list=%v", total, list)
	}

	if _, err := svc.Get(context.Background(), userID, keep.ID); err != nil {
		t.Fatalf("other hive's inspection should survive DeleteByHive: %v", err)
	}
}

func TestDeleteByHive_ScopedToUser(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)
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

	count, err := svc.DeleteByHive(context.Background(), other, "other-token", hiveID)
	if err != nil {
		t.Fatalf("DeleteByHive by non-owner: %v", err)
	}
	if count != 0 {
		t.Fatalf("DeleteByHive by non-owner count = %d, want 0", count)
	}

	if _, err := svc.Get(context.Background(), owner, created.ID); err != nil {
		t.Fatalf("owner's inspection should survive another user's DeleteByHive: %v", err)
	}
}

func TestDeleteByHive_ZeroMatchesIsNotAnError(t *testing.T) {
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), 14)

	count, err := svc.DeleteByHive(context.Background(), uuid.New(), "some-token", uuid.New())
	if err != nil {
		t.Fatalf("DeleteByHive with no matches: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

// TestCreate_WithImages_Success proves an inspection can be created with
// photos attached in the same call, deduplicated and ownership-verified
// against media-service before the inspection is persisted.
func TestCreate_WithImages_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	photo1 := uuid.New()
	photo2 := uuid.New()
	media.own(photo1, photo2)

	i, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{photo1, photo2, photo1}, // duplicated on purpose
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(i.Images) != 2 || i.Images[0] != photo1 || i.Images[1] != photo2 {
		t.Fatalf("Images = %v, want [%s, %s] deduplicated", i.Images, photo1, photo2)
	}

	got, err := svc.Get(context.Background(), userID, i.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Images) != 2 {
		t.Fatalf("persisted Images = %v, want 2 entries", got.Images)
	}
}

// TestCreate_WithImages_RejectsForeignMedia proves Create validates
// ownership of every referenced media id, not just the hive.
func TestCreate_WithImages_RejectsForeignMedia(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient() // foreign is deliberately never own()'d
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	foreign := uuid.New()

	_, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{foreign},
	})
	if !errors.Is(err, appinspection.ErrImageNotFound) {
		t.Fatalf("Create with foreign media: got %v, want ErrImageNotFound", err)
	}
}

// TestUpdate_ImagesNil_LeavesImagesUntouched proves that omitting Images
// on Update doesn't detach photos attached at creation time.
func TestUpdate_ImagesNil_LeavesImagesUntouched(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	mediaID := uuid.New()
	media.own(mediaID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{mediaID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(), Notes: "new notes", Type: inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Images) != 1 || updated.Images[0] != mediaID {
		t.Fatalf("Images = %v, want [%s] (untouched)", updated.Images, mediaID)
	}
}

// TestUpdate_ImagesEmpty_ClearsReferencesWithoutDeletingFiles proves that
// replacing Images with an empty slice detaches every photo but doesn't
// delete the underlying media file: removing a reference and deleting a
// file are different operations (the latter only happens when a caller
// explicitly calls DELETE /media/{id}, or the whole hive is deleted).
func TestUpdate_ImagesEmpty_ClearsReferencesWithoutDeletingFiles(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	mediaID := uuid.New()
	media.own(mediaID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{mediaID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	empty := []uuid.UUID{}
	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: &empty,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Images) != 0 {
		t.Fatalf("Images = %v, want empty", updated.Images)
	}
	if media.wasDeleted(mediaID) {
		t.Error("clearing images must not delete the underlying media file")
	}
}

// TestUpdate_ImagesReplacedWholesale proves an update's images list fully
// replaces the previous set: whatever isn't listed is detached.
func TestUpdate_ImagesReplacedWholesale(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	keep := uuid.New()
	drop := uuid.New()
	media.own(keep, drop)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{keep, drop},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	desired := []uuid.UUID{keep}
	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: &desired,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Images) != 1 || updated.Images[0] != keep {
		t.Fatalf("Images = %v, want [%s] deduplicated", updated.Images, keep)
	}
	if media.wasDeleted(drop) {
		t.Error("replacing images must not delete the dropped media file")
	}
}

// TestUpdate_ImagesRejectsForeignMedia proves that an update can't
// reference a media id that isn't the caller's own, and, critically,
// leaves the previous Images completely untouched when it's rejected.
func TestUpdate_ImagesRejectsForeignMedia(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient() // foreign is deliberately never own()'d
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	kept := uuid.New()
	media.own(kept)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{kept},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	foreign := uuid.New()
	desired := []uuid.UUID{foreign}
	_, err = svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: &desired,
	})
	if !errors.Is(err, appinspection.ErrImageNotFound) {
		t.Fatalf("Update with foreign media: got %v, want ErrImageNotFound", err)
	}

	got, err := svc.Get(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Images) != 1 || got.Images[0] != kept {
		t.Errorf("Images = %v after rejected update, want unchanged [%s]", got.Images, kept)
	}
}

// TestDeleteByHive_DeletesAttachedMedia proves the DeleteByHive cascade
// hard-deletes every media file referenced by the inspections it removes.
func TestDeleteByHive_DeletesAttachedMedia(t *testing.T) {
	verifier := newFakeHiveVerifier()
	media := newFakeMediaClient()
	svc := appinspection.NewService(newFakeRepo(), verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)
	photo := uuid.New()
	media.own(photo)

	_, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveID, InspectedAt: inspectedAt(), Notes: "n/a", Type: inspection.TypeRoutine,
		Images: []uuid.UUID{photo},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.DeleteByHive(context.Background(), userID, token, hiveID); err != nil {
		t.Fatalf("DeleteByHive: %v", err)
	}
	if !media.wasDeleted(photo) {
		t.Error("DeleteByHive did not delete the inspection's attached media")
	}
}

func TestCreate_WithImages_MaxLimit_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	photos := make([]uuid.UUID, 5)
	for i := range photos {
		photos[i] = uuid.New()
		media.own(photos[i])
	}

	ins, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection 5 photos",
		Type:        inspection.TypeRoutine,
		Images:      photos,
	})
	if err != nil {
		t.Fatalf("Create with 5 photos failed: %v", err)
	}
	if len(ins.Images) != 5 {
		t.Fatalf("Images length = %d, want 5", len(ins.Images))
	}
}

func TestCreate_WithImages_ExceedsLimit_ReturnsMediaLimitReached(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	photos := make([]uuid.UUID, 6)
	for i := range photos {
		photos[i] = uuid.New()
		media.own(photos[i])
	}

	_, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection 6 photos",
		Type:        inspection.TypeRoutine,
		Images:      photos,
	})
	if !errors.Is(err, appinspection.ErrMediaLimitReached) {
		t.Fatalf("expected ErrMediaLimitReached for 6 photos, got: %v", err)
	}
}

func TestCreate_WithImages_DuplicatesCountTowardUniqueLimit(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	photos := make([]uuid.UUID, 5)
	for i := range photos {
		photos[i] = uuid.New()
		media.own(photos[i])
	}
	withDupes := append(photos, photos[0])

	ins, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection 5 unique photos with dupe",
		Type:        inspection.TypeRoutine,
		Images:      withDupes,
	})
	if err != nil {
		t.Fatalf("Create with 5 unique photos failed: %v", err)
	}
	if len(ins.Images) != 5 {
		t.Fatalf("Images length = %d, want 5", len(ins.Images))
	}
}

func TestUpdate_WithImages_MaxLimit_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection",
		Type:        inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	photos := make([]uuid.UUID, 5)
	for i := range photos {
		photos[i] = uuid.New()
		media.own(photos[i])
	}

	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(),
		Notes:       "Inspection",
		Type:        inspection.TypeRoutine,
		Images:      &photos,
	})
	if err != nil {
		t.Fatalf("Update with 5 photos failed: %v", err)
	}
	if len(updated.Images) != 5 {
		t.Fatalf("Images length = %d, want 5", len(updated.Images))
	}
}

func TestUpdate_WithImages_ExceedsLimit_ReturnsMediaLimitReached(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection",
		Type:        inspection.TypeRoutine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	photos := make([]uuid.UUID, 6)
	for i := range photos {
		photos[i] = uuid.New()
		media.own(photos[i])
	}

	_, err = svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(),
		Notes:       "Inspection",
		Type:        inspection.TypeRoutine,
		Images:      &photos,
	})
	if !errors.Is(err, appinspection.ErrMediaLimitReached) {
		t.Fatalf("expected ErrMediaLimitReached for 6 photos on update, got: %v", err)
	}
}

func TestUpdate_WithExistingImages_ExceedsLimit_PreservesExistingImages(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	initialPhotos := make([]uuid.UUID, 5)
	for i := range initialPhotos {
		initialPhotos[i] = uuid.New()
		media.own(initialPhotos[i])
	}

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection with 5 photos",
		Type:        inspection.TypeRoutine,
		Images:      initialPhotos,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tooManyPhotos := make([]uuid.UUID, 6)
	for i := range tooManyPhotos {
		tooManyPhotos[i] = uuid.New()
		media.own(tooManyPhotos[i])
	}

	_, err = svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(),
		Notes:       "Inspection updated notes",
		Type:        inspection.TypeRoutine,
		Images:      &tooManyPhotos,
	})
	if !errors.Is(err, appinspection.ErrMediaLimitReached) {
		t.Fatalf("expected ErrMediaLimitReached, got %v", err)
	}

	persisted, err := svc.Get(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(persisted.Images) != 5 {
		t.Fatalf("expected 5 photos preserved, got %d", len(persisted.Images))
	}
	for i, id := range initialPhotos {
		if persisted.Images[i] != id {
			t.Fatalf("photo %d changed: got %v, want %v", i, persisted.Images[i], id)
		}
	}
}

func TestUpdate_WithExistingImages_NilImages_PreservesExistingImages(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	initialPhotos := make([]uuid.UUID, 5)
	for i := range initialPhotos {
		initialPhotos[i] = uuid.New()
		media.own(initialPhotos[i])
	}

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection with 5 photos",
		Type:        inspection.TypeRoutine,
		Images:      initialPhotos,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(),
		Notes:       "Renamed Inspection",
		Type:        inspection.TypeRoutine,
		Images:      nil,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Images) != 5 {
		t.Fatalf("expected 5 photos preserved, got %d", len(updated.Images))
	}

	persisted, err := svc.Get(context.Background(), userID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(persisted.Images) != 5 {
		t.Fatalf("expected 5 photos in DB, got %d", len(persisted.Images))
	}
}

func TestUpdate_WithExistingImages_ReplacesUpToLimit_Success(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	initialPhotos := make([]uuid.UUID, 5)
	for i := range initialPhotos {
		initialPhotos[i] = uuid.New()
		media.own(initialPhotos[i])
	}

	created, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection with 5 initial photos",
		Type:        inspection.TypeRoutine,
		Images:      initialPhotos,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newPhotos := make([]uuid.UUID, 5)
	for i := range newPhotos {
		newPhotos[i] = uuid.New()
		media.own(newPhotos[i])
	}

	updated, err := svc.Update(context.Background(), userID, token, created.ID, appinspection.UpdateInput{
		InspectedAt: inspectedAt(),
		Notes:       "Inspection with 5 replaced photos",
		Type:        inspection.TypeRoutine,
		Images:      &newPhotos,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Images) != 5 {
		t.Fatalf("expected 5 photos, got %d", len(updated.Images))
	}
	for i, id := range newPhotos {
		if updated.Images[i] != id {
			t.Fatalf("photo %d mismatch: got %v, want %v", i, updated.Images[i], id)
		}
	}
}

func TestMediaLimit_IndependentPerInspection(t *testing.T) {
	verifier := newFakeHiveVerifier()
	repo := newFakeRepo()
	media := newFakeMediaClient()
	svc := appinspection.NewService(repo, verifier, media, 14)
	userID := uuid.New()
	hiveID := uuid.New()
	token := "token"
	verifier.allow(token, hiveID)

	photos1 := make([]uuid.UUID, 5)
	photos2 := make([]uuid.UUID, 5)
	for i := range photos1 {
		photos1[i] = uuid.New()
		media.own(photos1[i])
		photos2[i] = uuid.New()
		media.own(photos2[i])
	}

	ins1, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection 1",
		Type:        inspection.TypeRoutine,
		Images:      photos1,
	})
	if err != nil {
		t.Fatalf("Create inspection 1 with 5 photos: %v", err)
	}

	ins2, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt(),
		Notes:       "Inspection 2",
		Type:        inspection.TypeRoutine,
		Images:      photos2,
	})
	if err != nil {
		t.Fatalf("Create inspection 2 with 5 photos: %v", err)
	}

	if len(ins1.Images) != 5 || len(ins2.Images) != 5 {
		t.Fatalf("expected both inspections to have 5 photos, got %d and %d", len(ins1.Images), len(ins2.Images))
	}
}

func TestHiveInspectionStatus_LatestPerHiveAndThreshold(t *testing.T) {
	verifier := newFakeHiveVerifier()
	svc := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 21)
	userID := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()
	token := "token"
	verifier.allow(token, hiveA)

	older := inspectedAt()
	newer := inspectedAt().Add(48 * time.Hour)

	if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveA, InspectedAt: older, Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("Create (older): %v", err)
	}
	verifier.allow(token, hiveA)
	if _, err := svc.Create(context.Background(), userID, token, appinspection.CreateInput{
		HiveID: hiveA, InspectedAt: newer, Type: inspection.TypeRoutine,
	}); err != nil {
		t.Fatalf("Create (newer): %v", err)
	}

	latestByHive, thresholdDays, err := svc.HiveInspectionStatus(context.Background(), userID)
	if err != nil {
		t.Fatalf("HiveInspectionStatus: %v", err)
	}
	if thresholdDays != 21 {
		t.Errorf("thresholdDays = %d, want 21 (the configured value)", thresholdDays)
	}
	got, ok := latestByHive[hiveA]
	if !ok || !got.Equal(newer) {
		t.Errorf("latestByHive[hiveA] = %v, ok=%v, want %v (the newer of the two)", got, ok, newer)
	}
	if _, ok := latestByHive[hiveB]; ok {
		t.Error("latestByHive contains hiveB, want it absent (never inspected)")
	}
}

func TestHiveInspectionStatus_NoInspectionsYieldsEmptyMap(t *testing.T) {
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), 14)

	latestByHive, thresholdDays, err := svc.HiveInspectionStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("HiveInspectionStatus: %v", err)
	}
	if len(latestByHive) != 0 {
		t.Errorf("latestByHive = %+v, want empty", latestByHive)
	}
	if thresholdDays != 14 {
		t.Errorf("thresholdDays = %d, want 14", thresholdDays)
	}
}

func TestHiveInspectionStatus_DefaultThresholdIsFourteenDays(t *testing.T) {
	svc := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), inspectionwarning.DefaultThresholdDays)

	_, thresholdDays, err := svc.HiveInspectionStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("HiveInspectionStatus: %v", err)
	}
	if thresholdDays != 14 {
		t.Errorf("thresholdDays = %d, want 14 (the default)", thresholdDays)
	}
}

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/pagination"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
	repopostgres "github.com/sbezhuk/beebase-inspection-service/internal/repository/postgres"
)

func inspectedAt() time.Time {
	return time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
}

func TestInspectionRepository_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	i := inspection.New(userID, hiveID, inspectedAt(), "queen seen, brood pattern good", inspection.TypeQueen)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, userID, i.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.HiveID != hiveID {
		t.Errorf("HiveID = %s, want %s", got.HiveID, hiveID)
	}
	if got.Notes != i.Notes {
		t.Errorf("Notes = %q, want %q", got.Notes, i.Notes)
	}
	if got.Type != inspection.TypeQueen {
		t.Errorf("Type = %q, want %q", got.Type, inspection.TypeQueen)
	}
	if !got.InspectedAt.Equal(i.InspectedAt) {
		t.Errorf("InspectedAt = %v, want %v", got.InspectedAt, i.InspectedAt)
	}
}

func TestInspectionRepository_GetByID_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)

	_, err = repo.GetByID(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("GetByID for unknown inspection: got %v, want ErrNotFound", err)
	}
}

// TestInspectionRepository_GetByID_WrongOwner_NotFound is the
// real-database version of this module's central security guarantee: an
// inspection that exists, but belongs to someone else, must be
// indistinguishable from one that doesn't exist at all.
func TestInspectionRepository_GetByID_WrongOwner_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	owner := uuid.New()
	other := uuid.New()

	i := inspection.New(owner, uuid.New(), inspectedAt(), "owner's inspection", inspection.TypeRoutine)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = repo.GetByID(ctx, other, i.ID)
	if !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("GetByID by non-owner: got %v, want ErrNotFound", err)
	}
}

func TestInspectionRepository_ListByHive_OnlyOwnInspectionsForThatHive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userA := uuid.New()
	userB := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()

	for _, notes := range []string{"first", "second"} {
		if err := repo.Create(ctx, inspection.New(userA, hiveA, inspectedAt(), notes, inspection.TypeRoutine)); err != nil {
			t.Fatalf("create %s: %v", notes, err)
		}
	}
	// same user, different hive: must not show up when listing hiveA
	if err := repo.Create(ctx, inspection.New(userA, uuid.New(), inspectedAt(), "different hive", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create different-hive inspection: %v", err)
	}
	// different user entirely, same hive id would be impossible in
	// practice (hive_id implies one owner) but a different hive for userB
	if err := repo.Create(ctx, inspection.New(userB, hiveB, inspectedAt(), "userB's", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create userB's: %v", err)
	}

	list, total, err := repo.ListByHive(ctx, userA, hiveA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
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
		if i.UserID != userA || i.HiveID != hiveA {
			t.Errorf("ListByHive leaked inspection %s (user %s, hive %s)", i.ID, i.UserID, i.HiveID)
		}
	}
}

func TestInspectionRepository_ListByUser_AcrossEveryHive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userA := uuid.New()
	userB := uuid.New()

	if err := repo.Create(ctx, inspection.New(userA, uuid.New(), inspectedAt(), "hive 1", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create hive1: %v", err)
	}
	if err := repo.Create(ctx, inspection.New(userA, uuid.New(), inspectedAt(), "hive 2", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create hive2: %v", err)
	}
	if err := repo.Create(ctx, inspection.New(userB, uuid.New(), inspectedAt(), "userB's", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create userB's: %v", err)
	}

	list, total, err := repo.ListByUser(ctx, userA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 2 {
		t.Fatalf("ListByUser total = %d, want 2", total)
	}
	if len(list) != 2 {
		t.Fatalf("ListByUser returned %d inspections, want 2", len(list))
	}
	for _, i := range list {
		if i.UserID != userA {
			t.Errorf("ListByUser leaked inspection %s belonging to %s", i.ID, i.UserID)
		}
	}
}

// TestInspectionRepository_ListByUser_DateFilterUsesInspectedAt guards the
// important distinction between the business date and the row's creation
// timestamp: both rows are created now, but only the 7 September inspection
// belongs in the requested 7 September page and count.
func TestInspectionRepository_ListByUser_DateFilterUsesInspectedAt(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	oldDate := time.Date(2026, 9, 7, 15, 30, 0, 0, time.UTC)
	newDate := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)

	old := inspection.New(userID, uuid.New(), oldDate, "created today, inspected 7 September", inspection.TypeRoutine)
	newer := inspection.New(userID, uuid.New(), newDate, "created today, inspected 14 September", inspection.TypeRoutine)
	if err := repo.Create(ctx, old); err != nil {
		t.Fatalf("create old-date inspection: %v", err)
	}
	if err := repo.Create(ctx, newer); err != nil {
		t.Fatalf("create new-date inspection: %v", err)
	}

	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	list, total, err := repo.ListByUser(ctx, userID, pagination.Params{Page: 1, Limit: 20}, nil, nil, &from, &to, nil)
	if err != nil {
		t.Fatalf("ListByUser date filter: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("date-filtered result = %d total, %d items; want 1 and 1", total, len(list))
	}
	if list[0].ID != old.ID {
		t.Fatalf("date-filtered item = %s, want inspection %s with inspected_at %v", list[0].ID, old.ID, oldDate)
	}
}

func TestInspectionRepository_ListByUser_Empty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)

	list, total, err := repo.ListByUser(ctx, uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
	if len(list) != 0 {
		t.Fatalf("ListByUser = %v, want empty", list)
	}
}

func TestInspectionRepository_ListByHive_Pagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	const count = 5
	for i := 0; i < count; i++ {
		if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "n/a", inspection.TypeRoutine)); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	// First page.
	first, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive page 1: %v", err)
	}
	if total != count {
		t.Fatalf("total = %d, want %d", total, count)
	}
	if len(first) != 2 {
		t.Fatalf("page 1 returned %d inspections, want 2", len(first))
	}

	// Middle page.
	middle, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 2, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive page 2: %v", err)
	}
	if total != count {
		t.Fatalf("total = %d, want %d", total, count)
	}
	if len(middle) != 2 {
		t.Fatalf("page 2 returned %d inspections, want 2", len(middle))
	}

	// Last (partial) page.
	last, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 3, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive page 3: %v", err)
	}
	if total != count {
		t.Fatalf("total = %d, want %d", total, count)
	}
	if len(last) != 1 {
		t.Fatalf("page 3 returned %d inspections, want 1", len(last))
	}

	// Page beyond available data.
	beyond, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 10, Limit: 2}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive page 10: %v", err)
	}
	if total != count {
		t.Fatalf("total = %d, want %d", total, count)
	}
	if len(beyond) != 0 {
		t.Fatalf("page beyond available data returned %d inspections, want 0", len(beyond))
	}

	// Pages must not overlap and together must cover every row exactly once.
	seen := map[uuid.UUID]bool{}
	for _, i := range append(append(first, middle...), last...) {
		if seen[i.ID] {
			t.Errorf("inspection %s appeared on more than one page", i.ID)
		}
		seen[i.ID] = true
	}
	if len(seen) != count {
		t.Errorf("pages together covered %d inspections, want %d", len(seen), count)
	}
}

func TestInspectionRepository_ListByHive_Empty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)

	list, total, err := repo.ListByHive(ctx, uuid.New(), uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
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

// TestInspectionRepository_ListByHive_StableOrdering guards against equal
// inspected_at timestamps reshuffling rows between pages: the id
// tiebreaker must make ordering deterministic even when many inspections
// share a timestamp.
func TestInspectionRepository_ListByHive_StableOrdering(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	same := inspectedAt()
	ids := make([]uuid.UUID, 4)
	for i := range ids {
		insp := inspection.New(userID, hiveID, same, "n/a", inspection.TypeRoutine)
		if err := repo.Create(ctx, insp); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		ids[i] = insp.ID
	}

	firstRun, _, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 4}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive run 1: %v", err)
	}
	secondRun, _, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 4}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive run 2: %v", err)
	}

	if len(firstRun) != len(secondRun) {
		t.Fatalf("run lengths differ: %d vs %d", len(firstRun), len(secondRun))
	}
	for i := range firstRun {
		if firstRun[i].ID != secondRun[i].ID {
			t.Fatalf("ordering unstable at index %d: %s vs %s", i, firstRun[i].ID, secondRun[i].ID)
		}
	}
}

// TestInspectionRepository_ListByHive_SortOrder proves ?sortOrder switches
// the primary sort key from InspectedAt (the default) to CreatedAt: notes
// are set to the reverse of creation order, so an assertion against notes
// only passes if CreatedAt (not InspectedAt) actually drove the result.
func TestInspectionRepository_ListByHive_SortOrder(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	base := time.Now().UTC()
	notes := []string{"Oldest", "Middle", "Newest"}
	sameInspectedAt := inspectedAt()
	for i, n := range notes {
		insp := inspection.New(userID, hiveID, sameInspectedAt, n, inspection.TypeRoutine)
		insp.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		insp.UpdatedAt = insp.CreatedAt
		if err := repo.Create(ctx, insp); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}

	asc := "asc"
	ascending, _, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, &asc)
	if err != nil {
		t.Fatalf("ListByHive asc: %v", err)
	}
	if got := inspectionNotesOf(ascending); !equalInspectionStrings(got, []string{"Oldest", "Middle", "Newest"}) {
		t.Fatalf("ascending order = %v, want [Oldest Middle Newest]", got)
	}

	desc := "desc"
	descending, _, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, &desc)
	if err != nil {
		t.Fatalf("ListByHive desc: %v", err)
	}
	if got := inspectionNotesOf(descending); !equalInspectionStrings(got, []string{"Newest", "Middle", "Oldest"}) {
		t.Fatalf("descending order = %v, want [Newest Middle Oldest]", got)
	}
}

func inspectionNotesOf(inspections []*inspection.Inspection) []string {
	notes := make([]string, len(inspections))
	for i, insp := range inspections {
		notes[i] = insp.Notes
	}
	return notes
}

func equalInspectionStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestInspectionRepository_ListByHive_WrongOwnerReturnsEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()

	if err := repo.Create(ctx, inspection.New(owner, hiveID, inspectedAt(), "owner's", inspection.TypeRoutine)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, _, err := repo.ListByHive(ctx, other, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListByHive by non-owner = %v, want empty", list)
	}
}

func TestInspectionRepository_Update(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()

	i := inspection.New(userID, uuid.New(), inspectedAt(), "old notes", inspection.TypeRoutine)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newTime := inspectedAt().Add(24 * time.Hour)
	i.InspectedAt = newTime
	i.Notes = "new notes"
	i.Type = inspection.TypeHealth
	if err := repo.Update(ctx, i); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, userID, i.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Type != inspection.TypeHealth {
		t.Errorf("Type = %q, want %q", got.Type, inspection.TypeHealth)
	}
	if got.Notes != "new notes" {
		t.Errorf("Notes = %q, want %q", got.Notes, "new notes")
	}
	if !got.InspectedAt.Equal(newTime) {
		t.Errorf("InspectedAt = %v, want %v", got.InspectedAt, newTime)
	}
}

func TestInspectionRepository_Update_WrongOwner_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	owner := uuid.New()
	other := uuid.New()

	i := inspection.New(owner, uuid.New(), inspectedAt(), "owner's", inspection.TypeRoutine)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	hijack := *i
	hijack.UserID = other
	hijack.Notes = "hijacked"
	if err := repo.Update(ctx, &hijack); !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Update with mismatched owner: got %v, want ErrNotFound", err)
	}

	got, err := repo.GetByID(ctx, owner, i.ID)
	if err != nil {
		t.Fatalf("GetByID after failed hijack: %v", err)
	}
	if got.Notes != "owner's" {
		t.Errorf("Notes = %q after failed hijack, want unchanged", got.Notes)
	}
}

func TestInspectionRepository_Delete_SoftDelete(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()

	i := inspection.New(userID, uuid.New(), inspectedAt(), "gone soon", inspection.TypeRoutine)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, userID, i.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.GetByID(ctx, userID, i.ID); !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("GetByID after delete: got %v, want ErrNotFound", err)
	}

	var deletedAt *string
	err = tx.QueryRow(ctx, "SELECT deleted_at::text FROM inspections WHERE id = $1", i.ID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("query raw row: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_at is NULL after Delete; expected it to be set (soft delete)")
	}
}

func TestInspectionRepository_Delete_WrongOwner_NotFoundAndNotDeleted(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	owner := uuid.New()
	other := uuid.New()

	i := inspection.New(owner, uuid.New(), inspectedAt(), "owner's", inspection.TypeRoutine)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, other, i.ID); !errors.Is(err, inspection.ErrNotFound) {
		t.Fatalf("Delete by non-owner: got %v, want ErrNotFound", err)
	}

	if _, err := repo.GetByID(ctx, owner, i.ID); err != nil {
		t.Fatalf("owner's inspection should survive a failed delete attempt: %v", err)
	}
}

func TestInspectionRepository_DeleteByHive_HardDeletesOnlyThatHivesInspections(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()

	a1 := inspection.New(userID, hiveA, inspectedAt(), "first", inspection.TypeRoutine)
	a2 := inspection.New(userID, hiveA, inspectedAt(), "second", inspection.TypeRoutine)
	b1 := inspection.New(userID, hiveB, inspectedAt(), "other hive", inspection.TypeRoutine)
	for _, i := range []*inspection.Inspection{a1, a2, b1} {
		if err := repo.Create(ctx, i); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	// Also cover an already-soft-deleted row under hiveA: DeleteByHive must
	// still purge it, since it has no deleted_at filter.
	alreadyGone := inspection.New(userID, hiveA, inspectedAt(), "already soft-deleted", inspection.TypeRoutine)
	if err := repo.Create(ctx, alreadyGone); err != nil {
		t.Fatalf("create already-soft-deleted: %v", err)
	}
	if err := repo.Delete(ctx, userID, alreadyGone.ID); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	_, count, err := repo.DeleteByHive(ctx, userID, hiveA)
	if err != nil {
		t.Fatalf("DeleteByHive: %v", err)
	}
	if count != 3 {
		t.Fatalf("DeleteByHive count = %d, want 3", count)
	}

	for _, id := range []uuid.UUID{a1.ID, a2.ID, alreadyGone.ID} {
		var n int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM inspections WHERE id = $1", id).Scan(&n); err != nil {
			t.Fatalf("raw count: %v", err)
		}
		if n != 0 {
			t.Errorf("inspection %s still present after DeleteByHive; want fully removed", id)
		}
	}

	if _, err := repo.GetByID(ctx, userID, b1.ID); err != nil {
		t.Fatalf("other hive's inspection should survive DeleteByHive: %v", err)
	}
}

func TestInspectionRepository_DeleteByHive_ScopedToUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()

	i := inspection.New(owner, hiveID, inspectedAt(), "owner's", inspection.TypeRoutine)
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, count, err := repo.DeleteByHive(ctx, other, hiveID)
	if err != nil {
		t.Fatalf("DeleteByHive by non-owner: %v", err)
	}
	if count != 0 {
		t.Fatalf("DeleteByHive by non-owner count = %d, want 0", count)
	}

	if _, err := repo.GetByID(ctx, owner, i.ID); err != nil {
		t.Fatalf("owner's inspection should survive another user's DeleteByHive: %v", err)
	}
}

func TestInspectionRepository_DeleteByHive_ZeroMatchesIsNotAnError(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)

	images, count, err := repo.DeleteByHive(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("DeleteByHive with no matches: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
	if len(images) != 0 {
		t.Fatalf("images = %v, want empty", images)
	}
}

// TestInspectionRepository_ImagesRoundTripThroughCreateAndUpdate proves
// the images column - inspection-service's own source of truth for
// attached media - survives Create and Update, and stays a real empty
// slice (never null) when there are no photos.
func TestInspectionRepository_ImagesRoundTripThroughCreateAndUpdate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()
	img1 := uuid.New()
	img2 := uuid.New()

	withoutImages := inspection.New(userID, hiveID, inspectedAt(), "No photos yet", inspection.TypeRoutine)
	if err := repo.Create(ctx, withoutImages); err != nil {
		t.Fatalf("Create without images: %v", err)
	}
	got, err := repo.GetByID(ctx, userID, withoutImages.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Images) != 0 {
		t.Fatalf("Images = %v, want empty (not null)", got.Images)
	}

	withImages := inspection.New(userID, hiveID, inspectedAt(), "Has photos", inspection.TypeRoutine)
	withImages.Images = []uuid.UUID{img1, img2}
	if err := repo.Create(ctx, withImages); err != nil {
		t.Fatalf("Create with images: %v", err)
	}
	got, err = repo.GetByID(ctx, userID, withImages.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Images) != 2 {
		t.Fatalf("Images = %v, want [%s, %s]", got.Images, img1, img2)
	}

	got.Images = []uuid.UUID{img1}
	got.Notes = "Has photos"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, err := repo.GetByID(ctx, userID, withImages.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if len(updated.Images) != 1 || updated.Images[0] != img1 {
		t.Fatalf("Images after update = %v, want [%s]", updated.Images, img1)
	}
}

// TestInspectionRepository_DeleteByHive_ReturnsUnionOfDeletedImages proves
// DeleteByHive surfaces every deleted inspection's own Images, so the
// application layer can hard-delete them from media-service too.
func TestInspectionRepository_DeleteByHive_ReturnsUnionOfDeletedImages(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveA := uuid.New()
	hiveB := uuid.New()
	img1 := uuid.New()
	img2 := uuid.New()
	img3 := uuid.New()

	a1 := inspection.New(userID, hiveA, inspectedAt(), "first", inspection.TypeRoutine)
	a1.Images = []uuid.UUID{img1, img2}
	a2 := inspection.New(userID, hiveA, inspectedAt(), "second", inspection.TypeRoutine)
	a2.Images = []uuid.UUID{img3}
	// Different hive: its images must not leak into hiveA's cascade.
	b1 := inspection.New(userID, hiveB, inspectedAt(), "other hive", inspection.TypeRoutine)
	b1.Images = []uuid.UUID{uuid.New()}
	for _, i := range []*inspection.Inspection{a1, a2, b1} {
		if err := repo.Create(ctx, i); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	images, count, err := repo.DeleteByHive(ctx, userID, hiveA)
	if err != nil {
		t.Fatalf("DeleteByHive: %v", err)
	}
	if count != 2 {
		t.Fatalf("DeleteByHive count = %d, want 2", count)
	}

	got := map[uuid.UUID]bool{}
	for _, id := range images {
		got[id] = true
	}
	for _, want := range []uuid.UUID{img1, img2, img3} {
		if !got[want] {
			t.Errorf("DeleteByHive images = %v, missing %s", images, want)
		}
	}
	if len(images) != 3 {
		t.Errorf("DeleteByHive images = %v, want 3 entries", images)
	}
}

// TestInspectionRepository_ListByHive_FilterByType proves the optional type
// filter is applied for every supported InspectionType, and that omitting
// it (nil) returns inspections of every type.
func TestInspectionRepository_ListByHive_FilterByType(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	for _, typ := range inspection.Types {
		if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), string(typ)+" inspection", typ)); err != nil {
			t.Fatalf("create %s: %v", typ, err)
		}
	}

	for _, typ := range inspection.Types {
		typ := typ
		list, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, &typ, nil, nil, nil)
		if err != nil {
			t.Fatalf("ListByHive type=%s: %v", typ, err)
		}
		if total != 1 || len(list) != 1 {
			t.Fatalf("ListByHive type=%s: total=%d len=%d, want 1 and 1", typ, total, len(list))
		}
		if list[0].Type != typ {
			t.Fatalf("ListByHive type=%s: got %s", typ, list[0].Type)
		}
	}

	all, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive without type filter: %v", err)
	}
	if total != len(inspection.Types) || len(all) != len(inspection.Types) {
		t.Fatalf("ListByHive without type filter: total=%d len=%d, want %d", total, len(all), len(inspection.Types))
	}
}

// TestInspectionRepository_ListByUser_FilterByType mirrors
// TestInspectionRepository_ListByHive_FilterByType for the cross-hive list,
// also proving the filter only matches the calling user's own inspections.
func TestInspectionRepository_ListByUser_FilterByType(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userA := uuid.New()
	userB := uuid.New()

	if err := repo.Create(ctx, inspection.New(userA, uuid.New(), inspectedAt(), "userA queen", inspection.TypeQueen)); err != nil {
		t.Fatalf("create userA queen: %v", err)
	}
	if err := repo.Create(ctx, inspection.New(userA, uuid.New(), inspectedAt(), "userA routine", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create userA routine: %v", err)
	}
	// Different user, same type: must not leak into userA's filtered results.
	if err := repo.Create(ctx, inspection.New(userB, uuid.New(), inspectedAt(), "userB queen", inspection.TypeQueen)); err != nil {
		t.Fatalf("create userB queen: %v", err)
	}

	queen := inspection.TypeQueen
	list, total, err := repo.ListByUser(ctx, userA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, &queen, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByUser type=QUEEN: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("ListByUser type=QUEEN: total=%d len=%d, want 1 and 1", total, len(list))
	}
	if list[0].UserID != userA || list[0].Type != inspection.TypeQueen {
		t.Fatalf("ListByUser type=QUEEN returned %+v, want userA's QUEEN inspection", list[0])
	}
}

// TestInspectionRepository_ListByHive_FilterByTypeCombinedWithSearch proves
// type and search apply together with AND semantics, not OR.
func TestInspectionRepository_ListByHive_FilterByTypeCombinedWithSearch(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "queen seen, healthy", inspection.TypeQueen)); err != nil {
		t.Fatalf("create matching: %v", err)
	}
	// Same notes term, different type: must be excluded by the type filter.
	if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "queen seen, brood checked", inspection.TypeBrood)); err != nil {
		t.Fatalf("create wrong type: %v", err)
	}
	// Same type, notes that don't match search: must be excluded by search.
	if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "nothing notable", inspection.TypeQueen)); err != nil {
		t.Fatalf("create wrong notes: %v", err)
	}

	queen := inspection.TypeQueen
	search := "healthy"
	list, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, &search, &queen, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive type+search: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("ListByHive type+search: total=%d len=%d, want 1 and 1", total, len(list))
	}
	if list[0].Notes != "queen seen, healthy" {
		t.Fatalf("ListByHive type+search returned %q, want the one matching both filters", list[0].Notes)
	}
}

// TestInspectionRepository_ListByHive_FilterByTypeCombinedWithPagination
// proves the type filter narrows the total/page count used for pagination
// metadata, not just the returned rows.
func TestInspectionRepository_ListByHive_FilterByTypeCombinedWithPagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	for i := 0; i < 3; i++ {
		if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "queen", inspection.TypeQueen)); err != nil {
			t.Fatalf("create queen %d: %v", i, err)
		}
	}
	if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "routine", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create routine: %v", err)
	}

	queen := inspection.TypeQueen
	page, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 2}, nil, &queen, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive type+pagination: %v", err)
	}
	if total != 3 {
		t.Fatalf("ListByHive type+pagination: total=%d, want 3 (routine excluded)", total)
	}
	if len(page) != 2 {
		t.Fatalf("ListByHive type+pagination: page len=%d, want 2", len(page))
	}
	for _, i := range page {
		if i.Type != inspection.TypeQueen {
			t.Errorf("ListByHive type+pagination leaked type %s", i.Type)
		}
	}
}

// TestInspectionRepository_ListByHive_DateFilter proves date_from/date_to
// each apply independently, together cover an inclusive range at day
// granularity, and reject nothing when the boundary dates exactly match an
// inspection's inspected_at.
func TestInspectionRepository_ListByHive_DateFilter(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	aug1 := inspection.New(userID, hiveID, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), "aug1", inspection.TypeRoutine)
	aug15 := inspection.New(userID, hiveID, time.Date(2026, 8, 15, 12, 30, 0, 0, time.UTC), "aug15", inspection.TypeRoutine)
	sep1 := inspection.New(userID, hiveID, time.Date(2026, 9, 1, 23, 59, 59, 0, time.UTC), "sep1", inspection.TypeRoutine)
	for _, i := range []*inspection.Inspection{aug1, aug15, sep1} {
		if err := repo.Create(ctx, i); err != nil {
			t.Fatalf("create %v: %v", i, err)
		}
	}

	// date_from only: everything on or after Aug 15.
	from := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	list, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, &from, nil, nil)
	if err != nil {
		t.Fatalf("ListByHive date_from: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("ListByHive date_from=%v = %+v (total=%d), want aug15 and sep1", from, list, total)
	}

	// date_to only: everything up to and including the whole day of Aug 15
	// - passed as the exclusive start of Aug 16, matching what the HTTP
	// handler's parseDateFilter computes for a date_to of Aug 15.
	to := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	list, total, err = repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, &to, nil)
	if err != nil {
		t.Fatalf("ListByHive date_to: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("ListByHive date_to=%v = %+v (total=%d), want aug1 and aug15", to, list, total)
	}

	// Both together: only Aug 15 falls within [Aug 15, Aug 16).
	list, total, err = repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, &from, &to, nil)
	if err != nil {
		t.Fatalf("ListByHive date_from+date_to: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != aug15.ID {
		t.Fatalf("ListByHive date_from=%v date_to=%v = %+v (total=%d), want only aug15", from, to, list, total)
	}

	// Exact boundary: a date_to of Sep 1 (exclusive bound Sep 2) must still
	// include an inspection at 23:59:59 on Sep 1.
	sep1Exclusive := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	list, total, err = repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit}, nil, nil, nil, &sep1Exclusive, nil)
	if err != nil {
		t.Fatalf("ListByHive date_to boundary: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("ListByHive date_to=%v = %+v (total=%d), want all 3 (whole day of sep1 included)", sep1Exclusive, list, total)
	}
}

// TestInspectionRepository_ListByHive_DateFilterCombinedWithSearchTypeAndPagination
// proves date_from/date_to apply together with search, type, and
// pagination using AND semantics.
func TestInspectionRepository_ListByHive_DateFilterCombinedWithSearchTypeAndPagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveID := uuid.New()

	inRange := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Matches every filter below.
	match1 := inspection.New(userID, hiveID, inRange, "queen seen, healthy", inspection.TypeQueen)
	match2 := inspection.New(userID, hiveID, inRange, "queen seen, healthy", inspection.TypeQueen)
	// Right type and notes, wrong date.
	wrongDate := inspection.New(userID, hiveID, outOfRange, "queen seen, healthy", inspection.TypeQueen)
	// Right date and notes, wrong type.
	wrongType := inspection.New(userID, hiveID, inRange, "queen seen, healthy", inspection.TypeBrood)
	// Right date and type, wrong notes.
	wrongNotes := inspection.New(userID, hiveID, inRange, "nothing notable", inspection.TypeQueen)
	for _, i := range []*inspection.Inspection{match1, match2, wrongDate, wrongType, wrongNotes} {
		if err := repo.Create(ctx, i); err != nil {
			t.Fatalf("create %v: %v", i, err)
		}
	}

	queen := inspection.TypeQueen
	search := "healthy"
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	page, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 1}, &search, &queen, &from, &to, nil)
	if err != nil {
		t.Fatalf("ListByHive combined: %v", err)
	}
	if total != 2 {
		t.Fatalf("ListByHive combined: total = %d, want 2 (match1 and match2 only)", total)
	}
	if len(page) != 1 {
		t.Fatalf("ListByHive combined: page len = %d, want 1", len(page))
	}
	if page[0].Type != inspection.TypeQueen || page[0].Notes != "queen seen, healthy" {
		t.Fatalf("ListByHive combined: unexpected result %+v", page[0])
	}
}

func TestInspectionRepository_LatestInspectedAtByHive_MaxPerHiveExcludingDeleted(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	userID := uuid.New()
	hiveWithTwo := uuid.New()
	hiveWithOneDeleted := uuid.New()
	hiveNeverInspected := uuid.New()

	older := inspectedAt()
	newer := inspectedAt().Add(48 * time.Hour)

	if err := repo.Create(ctx, inspection.New(userID, hiveWithTwo, older, "first", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create older: %v", err)
	}
	newest := inspection.New(userID, hiveWithTwo, newer, "second", inspection.TypeRoutine)
	if err := repo.Create(ctx, newest); err != nil {
		t.Fatalf("create newer: %v", err)
	}

	deleted := inspection.New(userID, hiveWithOneDeleted, older, "will be deleted", inspection.TypeRoutine)
	if err := repo.Create(ctx, deleted); err != nil {
		t.Fatalf("create deleted: %v", err)
	}
	if err := repo.Delete(ctx, userID, deleted.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	latestByHive, err := repo.LatestInspectedAtByHive(ctx, userID)
	if err != nil {
		t.Fatalf("LatestInspectedAtByHive: %v", err)
	}

	got, ok := latestByHive[hiveWithTwo]
	if !ok || !got.Equal(newer) {
		t.Errorf("latestByHive[hiveWithTwo] = %v, ok=%v, want %v (the more recent of the two)", got, ok, newer)
	}
	if _, ok := latestByHive[hiveWithOneDeleted]; ok {
		t.Error("latestByHive contains hiveWithOneDeleted, want it absent - its only inspection was deleted")
	}
	if _, ok := latestByHive[hiveNeverInspected]; ok {
		t.Error("latestByHive contains hiveNeverInspected, want it absent")
	}
}

func TestInspectionRepository_LatestInspectedAtByHive_ScopedToUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)
	owner := uuid.New()
	other := uuid.New()
	hiveID := uuid.New()

	if err := repo.Create(ctx, inspection.New(owner, hiveID, inspectedAt(), "mine", inspection.TypeRoutine)); err != nil {
		t.Fatalf("create: %v", err)
	}

	latestByHive, err := repo.LatestInspectedAtByHive(ctx, other)
	if err != nil {
		t.Fatalf("LatestInspectedAtByHive: %v", err)
	}
	if _, ok := latestByHive[hiveID]; ok {
		t.Error("another user's inspection leaked into LatestInspectedAtByHive")
	}
}

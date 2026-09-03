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

	list, total, err := repo.ListByHive(ctx, userA, hiveA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
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

	list, total, err := repo.ListByUser(ctx, userA, pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
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

func TestInspectionRepository_ListByUser_Empty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	repo := repopostgres.NewInspectionRepository(tx)

	list, total, err := repo.ListByUser(ctx, uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
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
	first, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 2})
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
	middle, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 2, Limit: 2})
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
	last, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 3, Limit: 2})
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
	beyond, total, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 10, Limit: 2})
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

	list, total, err := repo.ListByHive(ctx, uuid.New(), uuid.New(), pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
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

	firstRun, _, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 4})
	if err != nil {
		t.Fatalf("ListByHive run 1: %v", err)
	}
	secondRun, _, err := repo.ListByHive(ctx, userID, hiveID, pagination.Params{Page: 1, Limit: 4})
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

	list, _, err := repo.ListByHive(ctx, other, hiveID, pagination.Params{Page: 1, Limit: pagination.DefaultLimit})
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

	count, err := repo.DeleteByHive(ctx, userID, hiveA)
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

	count, err := repo.DeleteByHive(ctx, other, hiveID)
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

	count, err := repo.DeleteByHive(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("DeleteByHive with no matches: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

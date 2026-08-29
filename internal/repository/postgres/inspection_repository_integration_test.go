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

	i := inspection.New(userID, hiveID, inspectedAt(), "queen seen, brood pattern good")
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

	i := inspection.New(owner, uuid.New(), inspectedAt(), "owner's inspection")
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
		if err := repo.Create(ctx, inspection.New(userA, hiveA, inspectedAt(), notes)); err != nil {
			t.Fatalf("create %s: %v", notes, err)
		}
	}
	// same user, different hive: must not show up when listing hiveA
	if err := repo.Create(ctx, inspection.New(userA, uuid.New(), inspectedAt(), "different hive")); err != nil {
		t.Fatalf("create different-hive inspection: %v", err)
	}
	// different user entirely, same hive id would be impossible in
	// practice (hive_id implies one owner) but a different hive for userB
	if err := repo.Create(ctx, inspection.New(userB, hiveB, inspectedAt(), "userB's")); err != nil {
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
		if err := repo.Create(ctx, inspection.New(userID, hiveID, inspectedAt(), "n/a")); err != nil {
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
		insp := inspection.New(userID, hiveID, same, "n/a")
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

	if err := repo.Create(ctx, inspection.New(owner, hiveID, inspectedAt(), "owner's")); err != nil {
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

	i := inspection.New(userID, uuid.New(), inspectedAt(), "old notes")
	if err := repo.Create(ctx, i); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newTime := inspectedAt().Add(24 * time.Hour)
	i.InspectedAt = newTime
	i.Notes = "new notes"
	if err := repo.Update(ctx, i); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, userID, i.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
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

	i := inspection.New(owner, uuid.New(), inspectedAt(), "owner's")
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

	i := inspection.New(userID, uuid.New(), inspectedAt(), "gone soon")
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

	i := inspection.New(owner, uuid.New(), inspectedAt(), "owner's")
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

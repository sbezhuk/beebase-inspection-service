package inspection

import (
	"context"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/pagination"
)

// Repository is the port through which the application persists and
// retrieves inspections. Every method that targets a specific inspection
// takes the owning userID alongside the inspection ID, so ownership is
// enforced by the query itself, not by a separate check layered on top.
//
// UserID is denormalized onto the inspection row rather than looked up
// via HiveID on every call: hive-service (a different service, a
// different database) is the only source of truth for hive ownership -
// and, transitively, apiary ownership - and is asked exactly once, at
// creation time. HiveID never changes after that, so the denormalized
// UserID stays correct without a cross-service call on every read.
type Repository interface {
	Create(ctx context.Context, i *Inspection) error
	GetByID(ctx context.Context, userID, inspectionID uuid.UUID) (*Inspection, error)
	// ListByHive returns the page of inspections described by p for
	// hiveID that belong to userID, along with the total number of
	// matching inspections (independent of p, for computing pagination
	// metadata). If hiveID belongs to someone else, the result is empty
	// (not an error): the same "not found" hides existence either way.
	ListByHive(ctx context.Context, userID, hiveID uuid.UUID, p pagination.Params) (inspections []*Inspection, total int, err error)
	// ListByUser returns the page of inspections described by p across
	// every hive belonging to userID, along with the total number of
	// matching inspections (independent of p). Used by statistics-service
	// to compute inspection stats without a per-hive fan-out.
	ListByUser(ctx context.Context, userID uuid.UUID, p pagination.Params) (inspections []*Inspection, total int, err error)
	// Update persists i.InspectedAt, i.Notes, i.Type, and i.UpdatedAt for
	// the inspection identified by i.ID, scoped to i.UserID. HiveID is
	// immutable and never updated.
	Update(ctx context.Context, i *Inspection) error
	// Delete soft-deletes the inspection (sets deleted_at) rather than
	// removing the row, per the project's synchronizable-entity plan.
	Delete(ctx context.Context, userID, inspectionID uuid.UUID) error
	// DeleteByHive hard-deletes every inspection belonging to hiveID and
	// userID, including ones a prior soft-delete already marked gone.
	// Used only when hive-service cascades a hive delete; a zero count is
	// a normal outcome (the hive may simply have no inspections), not an
	// error.
	DeleteByHive(ctx context.Context, userID, hiveID uuid.UUID) (int64, error)
}

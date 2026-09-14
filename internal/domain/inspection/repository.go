package inspection

import (
	"context"
	"time"

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
	// When search is non-nil its value is matched case-insensitively
	// against notes; a nil search means no filter. When typ is non-nil,
	// only inspections of that Type are returned; a nil typ means no
	// filter. dateFrom/dateTo restrict InspectedAt, independently of one
	// another - dateFrom is an inclusive lower bound, dateTo is an
	// exclusive upper bound that the caller has already advanced to the
	// start of the day after the requested end date, so together they
	// cover the requested date_to's whole calendar day. Every given filter
	// applies together (AND semantics). When sortOrder is non-nil ("asc"
	// or "desc") the page is ordered by creation date in that direction
	// instead of the default order (InspectedAt); a nil sortOrder keeps
	// the default order.
	ListByHive(ctx context.Context, userID, hiveID uuid.UUID, p pagination.Params, search *string, typ *Type, dateFrom, dateTo *time.Time, sortOrder *string) (inspections []*Inspection, total int, err error)
	// ListByUser returns the page of inspections described by p across
	// every hive belonging to userID, along with the total number of
	// matching inspections (independent of p). Used by statistics-service
	// to compute inspection stats without a per-hive fan-out. When search
	// is non-nil its value is matched case-insensitively against notes; a
	// nil search means no filter. When typ is non-nil, only inspections of
	// that Type are returned; a nil typ means no filter. Both filters,
	// when given, apply together (AND semantics). When sortOrder is
	// non-nil ("asc" or "desc") the page is ordered by creation date in
	// that direction instead of the default order (InspectedAt); a nil
	// sortOrder keeps the default order.
	ListByUser(ctx context.Context, userID uuid.UUID, p pagination.Params, search *string, typ *Type, dateFrom, dateTo *time.Time, sortOrder *string) (inspections []*Inspection, total int, err error)
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
	// error. images is the union of every deleted inspection's own Images,
	// so the caller can hard-delete them from media-service too - nothing
	// else purges them once their inspection is gone.
	DeleteByHive(ctx context.Context, userID, hiveID uuid.UUID) (images []uuid.UUID, count int64, err error)
	// LatestInspectedAtByHive returns the most recent InspectedAt for
	// every hive userID has at least one (non-deleted) inspection under,
	// keyed by hive id. A hive with no inspections at all is simply
	// absent from the result - not a zero-value entry - so callers can
	// tell "never inspected" apart from "inspected at the zero time"
	// unambiguously. Backs GET /api/v1/inspections/hive-status, which
	// hive-service and statistics-service both call to apply the
	// "needs inspection" rule (see beebase-common/inspectionwarning)
	// without each maintaining their own copy of inspection dates.
	LatestInspectedAtByHive(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]time.Time, error)
}

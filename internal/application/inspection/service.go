// Package inspection implements the inspection use cases: create, get,
// list (for a hive), update, and delete. It depends only on the
// domain/inspection port and the HiveVerifier port declared in this
// package, never on HTTP or PostgreSQL directly.
package inspection

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/pagination"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/health"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// Service implements the inspection use cases. Every method takes the
// requesting user's ID (extracted from their verified access token by the
// transport layer) and passes it straight through to the repository,
// which enforces ownership at the query level.
type Service struct {
	inspections          inspection.Repository
	hives                HiveVerifier
	media                MediaClient
	warningThresholdDays int
	reminders            interface {
		Cleanup(context.Context, string, uuid.UUID) error
	}
}

// NewService constructs a Service. warningThresholdDays is the
// configured "needs inspection" threshold (see
// beebase-common/inspectionwarning) - this service is the single source
// of truth for it, echoed back by HiveInspectionStatus so callers never
// need their own copy.
func NewService(inspections inspection.Repository, hives HiveVerifier, media MediaClient, warningThresholdDays int, reminders ...interface {
	Cleanup(context.Context, string, uuid.UUID) error
}) *Service {
	s := &Service{inspections: inspections, hives: hives, media: media, warningThresholdDays: warningThresholdDays}
	if len(reminders) > 0 {
		s.reminders = reminders[0]
	}
	return s
}

// Create creates a new inspection owned by userID for in.HiveID, after
// confirming with hive-service that userID actually owns that hive (and,
// transitively, its apiary). accessToken is the caller's own access
// token, forwarded to hive-service so it can run its own ownership
// check, rather than this service trusting a client-supplied
// user/hive pairing. If in.Images is non-empty, it's deduplicated
// (preserving first-seen order) and every id's ownership is verified
// against media-service (see MediaClient.VerifyOwnership) before
// anything is persisted; if verification fails, Create returns the error
// immediately, having created nothing.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, accessToken string, in CreateInput) (*inspection.Inspection, error) {
	if err := in.Assessment.ValidateFor(in.Type); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssessmentInvalid, err)
	}
	writable, err := s.hives.Verify(ctx, accessToken, in.HiveID)
	if err != nil {
		return nil, err
	}
	if !writable {
		return nil, ErrHiveReadOnly
	}

	dedup := dedupeImages(in.Images)
	if len(dedup) > MaxMediaAttachments {
		return nil, ErrMediaLimitReached
	}
	if len(dedup) > 0 {
		if err := s.media.VerifyOwnership(ctx, accessToken, dedup); err != nil {
			return nil, err
		}
	}

	i := inspection.New(userID, in.HiveID, in.InspectedAt, in.Notes, in.Type)
	i.Images = dedup
	i.Assessment = in.Assessment
	if err := s.inspections.Create(ctx, i); err != nil {
		return nil, fmt.Errorf("inspection: create: %w", err)
	}

	return i, nil
}

// dedupeImages returns ids with duplicates removed, preserving the order
// each id first appeared in - so a client submitting the same id twice
// can't cause redundant work or a spurious count mismatch against
// media-service's response.
func dedupeImages(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	dedup := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		dedup = append(dedup, id)
	}
	return dedup
}

// Get returns the inspection identified by inspectionID, if it belongs
// to userID.
func (s *Service) Get(ctx context.Context, userID, inspectionID uuid.UUID) (*inspection.Inspection, error) {
	return s.inspections.GetByID(ctx, userID, inspectionID)
}

// ListByHive returns the page of inspections described by p belonging to
// userID for hiveID. When search is non-nil its value is matched
// case-insensitively against the inspection's notes field. When typ is
// non-nil, only inspections of that Type are returned. dateFrom/dateTo are
// optional filters on InspectedAt - see inspection.Repository.ListByHive
// for how they combine with the other filters. When sortOrder is non-nil
// ("asc" or "desc") the page is ordered by creation date in that direction
// instead of the repository's default order (InspectedAt).
func (s *Service) ListByHive(ctx context.Context, userID, hiveID uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	return s.inspections.ListByHive(ctx, userID, hiveID, p, search, typ, dateFrom, dateTo, sortOrder)
}

// GetHiveHealth derives the current Colony Health snapshot from the complete
// inspection history for hiveID. asOf is supplied by the transport boundary;
// this method does not read the system clock.
func (s *Service) GetHiveHealth(ctx context.Context, userID uuid.UUID, accessToken string, hiveID uuid.UUID, asOf time.Time) (health.ColonyHealthEvaluation, error) {
	if _, err := s.hives.Verify(ctx, accessToken, hiveID); err != nil {
		return health.ColonyHealthEvaluation{}, err
	}

	reader, ok := s.inspections.(HealthInspectionReader)
	if !ok {
		return health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: repository does not support health evaluation history")
	}
	inspections, err := reader.ListAllByHive(ctx, userID, hiveID)
	if err != nil {
		return health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: list health evaluation history: %w", err)
	}

	input := health.DimensionEvaluationInput{
		AsOf:             asOf,
		RecencyPolicy:    health.DefaultRecencyPolicyV1(),
		Evidence:         []health.HealthEvidence{},
		ManagementEvents: []health.ManagementEvent{},
		ContextFacts:     []health.ContextFact{},
	}
	for _, current := range inspections {
		if current == nil {
			return health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: health evaluation history contains nil inspection")
		}
		normalized := health.NormalizeInspection(*current)
		input.Evidence = append(input.Evidence, normalized.HealthEvidence...)
		input.ManagementEvents = append(input.ManagementEvents, normalized.ManagementEvents...)
		input.ContextFacts = append(input.ContextFacts, normalized.ContextFacts...)
	}

	dimensions, err := health.EvaluateDimensions(input)
	if err != nil {
		return health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: evaluate health dimensions: %w", err)
	}
	result, err := health.EvaluateColonyHealth(dimensions)
	if err != nil {
		return health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: evaluate colony health: %w", err)
	}
	return result, nil
}

// List returns the page of inspections described by p across every hive
// belonging to userID. When search is non-nil its value is matched
// case-insensitively against the inspection's notes field. When typ is
// non-nil, only inspections of that Type are returned. Both filters, when
// given, apply together (AND semantics). When sortOrder is non-nil ("asc"
// or "desc") the page is ordered by creation date in that direction
// instead of the repository's default order (InspectedAt).
func (s *Service) List(ctx context.Context, userID uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	return s.inspections.ListByUser(ctx, userID, p, search, typ, dateFrom, dateTo, sortOrder)
}

// Update replaces the editable fields of the inspection identified by
// inspectionID, if it belongs to userID. accessToken is the caller's own
// access token, forwarded to media-service so it can run its own
// ownership check. When in.Images is non-nil, it's deduplicated
// (preserving first-seen order) and, if non-empty, every id's ownership
// is verified against media-service before anything changes; if
// verification fails, Update returns the error immediately, leaving the
// inspection's row (including its current Images) completely untouched.
// On success, Images is simply replaced with the deduplicated set - there
// is nothing external to reconcile against, since this service's own
// Images column is already the sole source of truth for what's
// referenced. When in.Images is nil, Images is left untouched entirely.
func (s *Service) Update(ctx context.Context, userID uuid.UUID, accessToken string, inspectionID uuid.UUID, in UpdateInput) (*inspection.Inspection, error) {
	i, err := s.inspections.GetByID(ctx, userID, inspectionID)
	if err != nil {
		return nil, err
	}

	// The parent hive must be re-verified on every update, not just at
	// create time: unlike ownership (fixed forever once created), a
	// hive's writability changes over time as the caller's subscription
	// and resource counts change, so it can't safely be assumed from
	// creation-time state. This brings Update up to the same per-op
	// check Create already does, and that harvest-service already does
	// for every operation.
	writable, err := s.hives.Verify(ctx, accessToken, i.HiveID)
	if err != nil {
		return nil, err
	}
	if !writable {
		return nil, ErrHiveReadOnly
	}

	if in.Images != nil {
		dedup := dedupeImages(*in.Images)
		if len(dedup) > MaxMediaAttachments {
			return nil, ErrMediaLimitReached
		}
		if len(dedup) > 0 {
			if err := s.media.VerifyOwnership(ctx, accessToken, dedup); err != nil {
				return nil, err
			}
		}
		i.Images = dedup
	}
	if in.Assessment != nil {
		if err := in.Assessment.ValidateFor(i.Type); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrAssessmentInvalid, err)
		}
		i.Assessment = in.Assessment
	}

	i.InspectedAt = in.InspectedAt
	i.Notes = in.Notes
	// Type is immutable. The field remains accepted because the released
	// client resends it on PUT; changed values are ignored for compatibility.
	i.UpdatedAt = time.Now().UTC()

	if err := s.inspections.Update(ctx, i); err != nil {
		return nil, fmt.Errorf("inspection: update: %w", err)
	}

	return i, nil
}

// Delete deletes the inspection identified by inspectionID, if it
// belongs to userID.
func (s *Service) Delete(ctx context.Context, userID, inspectionID uuid.UUID) error {
	return s.inspections.Delete(ctx, userID, inspectionID)
}

func (s *Service) DeleteLocalByUser(ctx context.Context, userID uuid.UUID) error {
	r, ok := s.inspections.(interface {
		DeleteAllByUserHard(context.Context, uuid.UUID) error
	})
	if !ok {
		return fmt.Errorf("inspection: repository does not support account cleanup")
	}
	return r.DeleteAllByUserHard(ctx, userID)
}

// HiveInspectionStatus returns the latest InspectedAt for every hive
// userID has ever inspected (a hive with none is simply absent from the
// map), along with the currently configured inspection warning
// threshold in days. It performs no ownership check of its own -
// inspections are already scoped by userID, exactly like List - so
// there's no hive-service round trip here.
func (s *Service) HiveInspectionStatus(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]time.Time, int, error) {
	latestByHive, err := s.inspections.LatestInspectedAtByHive(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("inspection: latest inspected_at by hive: %w", err)
	}
	return latestByHive, s.warningThresholdDays, nil
}

// DeleteByHive hard-deletes every inspection belonging to hiveID and
// userID, then hard-deletes every media file any of them referenced.
// accessToken is the caller's own access token, forwarded to
// media-service so it can run its own ownership check. Used when
// hive-service cascades a hive delete.
func (s *Service) DeleteByHive(ctx context.Context, userID uuid.UUID, accessToken string, hiveID uuid.UUID) (int64, error) {
	ids, err := s.inspections.ListIDsByHive(ctx, userID, hiveID)
	if err != nil {
		return 0, err
	}
	if s.reminders != nil {
		for _, id := range ids {
			if err := s.reminders.Cleanup(ctx, "inspection", id); err != nil {
				return 0, err
			}
		}
	}
	images, count, err := s.inspections.DeleteByHive(ctx, userID, hiveID)
	if err != nil {
		return 0, err
	}
	if len(images) > 0 {
		if err := s.media.DeleteByIDs(ctx, accessToken, images); err != nil {
			return 0, err
		}
	}
	return count, nil
}

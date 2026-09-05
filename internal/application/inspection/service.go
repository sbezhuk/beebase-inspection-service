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
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// Service implements the inspection use cases. Every method takes the
// requesting user's ID (extracted from their verified access token by the
// transport layer) and passes it straight through to the repository,
// which enforces ownership at the query level.
type Service struct {
	inspections inspection.Repository
	hives       HiveVerifier
	media       MediaClient
}

// NewService constructs a Service.
func NewService(inspections inspection.Repository, hives HiveVerifier, media MediaClient) *Service {
	return &Service{inspections: inspections, hives: hives, media: media}
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
	if err := s.hives.Verify(ctx, accessToken, in.HiveID); err != nil {
		return nil, err
	}

	dedup := dedupeImages(in.Images)
	if len(dedup) > 0 {
		if err := s.media.VerifyOwnership(ctx, accessToken, dedup); err != nil {
			return nil, err
		}
	}

	i := inspection.New(userID, in.HiveID, in.InspectedAt, in.Notes, in.Type)
	i.Images = dedup
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
// userID for hiveID.
func (s *Service) ListByHive(ctx context.Context, userID, hiveID uuid.UUID, p pagination.Params) ([]*inspection.Inspection, int, error) {
	return s.inspections.ListByHive(ctx, userID, hiveID, p)
}

// List returns the page of inspections described by p across every hive
// belonging to userID.
func (s *Service) List(ctx context.Context, userID uuid.UUID, p pagination.Params) ([]*inspection.Inspection, int, error) {
	return s.inspections.ListByUser(ctx, userID, p)
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

	if in.Images != nil {
		dedup := dedupeImages(*in.Images)
		if len(dedup) > 0 {
			if err := s.media.VerifyOwnership(ctx, accessToken, dedup); err != nil {
				return nil, err
			}
		}
		i.Images = dedup
	}

	i.InspectedAt = in.InspectedAt
	i.Notes = in.Notes
	i.Type = in.Type
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

// DeleteByHive hard-deletes every inspection belonging to hiveID and
// userID, then hard-deletes every media file any of them referenced.
// accessToken is the caller's own access token, forwarded to
// media-service so it can run its own ownership check. Used when
// hive-service cascades a hive delete.
func (s *Service) DeleteByHive(ctx context.Context, userID uuid.UUID, accessToken string, hiveID uuid.UUID) (int64, error) {
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

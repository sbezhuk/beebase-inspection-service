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
}

// NewService constructs a Service.
func NewService(inspections inspection.Repository, hives HiveVerifier) *Service {
	return &Service{inspections: inspections, hives: hives}
}

// Create creates a new inspection owned by userID for in.HiveID, after
// confirming with hive-service that userID actually owns that hive (and,
// transitively, its apiary). accessToken is the caller's own access
// token, forwarded to hive-service so it can run its own ownership
// check, rather than this service trusting a client-supplied
// user/hive pairing.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, accessToken string, in CreateInput) (*inspection.Inspection, error) {
	if err := s.hives.Verify(ctx, accessToken, in.HiveID); err != nil {
		return nil, err
	}

	i := inspection.New(userID, in.HiveID, in.InspectedAt, in.Notes, in.Type)
	if err := s.inspections.Create(ctx, i); err != nil {
		return nil, fmt.Errorf("inspection: create: %w", err)
	}

	return i, nil
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

// Update replaces the editable fields of the inspection identified by
// inspectionID, if it belongs to userID.
func (s *Service) Update(ctx context.Context, userID, inspectionID uuid.UUID, in UpdateInput) (*inspection.Inspection, error) {
	i, err := s.inspections.GetByID(ctx, userID, inspectionID)
	if err != nil {
		return nil, err
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

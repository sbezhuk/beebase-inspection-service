package inspection

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-health/health"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// HealthHistoryPoint is one calendar day's Colony Health v1 snapshot, as
// of that day's calendar-date boundary.
type HealthHistoryPoint struct {
	Date       time.Time
	Evaluation health.ColonyHealthEvaluation
}

// HealthHistoryResult is the full answer to a Colony Health history
// query: one point per calendar day in [From, To], plus every inspection
// that occurred within that same inclusive range - not evidence, just
// enough for a client to link a point to the inspection(s) it can open.
type HealthHistoryResult struct {
	From        time.Time
	To          time.Time
	Points      []HealthHistoryPoint
	Inspections []*inspection.Inspection
}

// GetHiveHealth derives the current Colony Health snapshot from the complete
// inspection history for hiveID. asOf is supplied by the transport boundary;
// this method does not read the system clock.
func (s *Service) GetHiveHealth(ctx context.Context, userID uuid.UUID, accessToken string, hiveID uuid.UUID, asOf time.Time) (health.ColonyHealthEvaluation, error) {
	if _, err := s.hives.Verify(ctx, accessToken, hiveID); err != nil {
		return health.ColonyHealthEvaluation{}, err
	}

	evidence, _, err := s.loadHealthEvidence(ctx, userID, hiveID)
	if err != nil {
		return health.ColonyHealthEvaluation{}, err
	}

	result, err := health.CalculateColonyHealth(evidence, asOf)
	if err != nil {
		return health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: evaluate colony health: %w", err)
	}
	return result, nil
}

// calculateHealthHistory is the canonical range calculation used by the
// trusted internal report endpoint.
func calculateHealthHistory(evidence []health.HealthEvidence, all []*inspection.Inspection, from, to time.Time) (HealthHistoryResult, error) {

	points := make([]HealthHistoryPoint, 0, int(to.Sub(from).Hours()/24)+1)
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		evaluation, err := health.CalculateColonyHealth(evidence, day)
		if err != nil {
			return HealthHistoryResult{}, fmt.Errorf("inspection: evaluate colony health history: %w", err)
		}
		points = append(points, HealthHistoryPoint{Date: day, Evaluation: evaluation})
	}

	var inRange []*inspection.Inspection
	for _, current := range all {
		if !current.InspectedAt.Before(from) && !current.InspectedAt.After(to) {
			inRange = append(inRange, current)
		}
	}
	sort.Slice(inRange, func(i, j int) bool {
		if !inRange[i].InspectedAt.Equal(inRange[j].InspectedAt) {
			return inRange[i].InspectedAt.Before(inRange[j].InspectedAt)
		}
		return inRange[i].ID.String() < inRange[j].ID.String()
	})

	return HealthHistoryResult{From: from, To: to, Points: points, Inspections: inRange}, nil
}

// GetInternalReportData returns the inspection-owned report data for a
// trusted service caller. It deliberately bypasses user entitlement and
// ownership checks: the endpoint is protected by INTERNAL_SERVICE_TOKEN.
// Health and history still use the exact same evaluator and range logic as
// the public endpoints.
func (s *Service) GetInternalReportData(ctx context.Context, hiveID uuid.UUID, from, to time.Time) (HealthHistoryResult, health.ColonyHealthEvaluation, error) {
	reader, ok := s.inspections.(InternalReportReader)
	if !ok {
		return HealthHistoryResult{}, health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: repository does not support internal report history")
	}
	all, err := reader.ListAllByHiveInternal(ctx, hiveID)
	if err != nil {
		return HealthHistoryResult{}, health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: list internal report history: %w", err)
	}

	evidence := make([]health.HealthEvidence, 0)
	for _, current := range all {
		if current == nil {
			return HealthHistoryResult{}, health.ColonyHealthEvaluation{}, fmt.Errorf("inspection: internal report history contains nil inspection")
		}
		normalized := health.NormalizeInspection(toHealthInspection(current))
		evidence = append(evidence, normalized.HealthEvidence...)
	}

	history, err := calculateHealthHistory(evidence, all, from, to)
	if err != nil {
		return HealthHistoryResult{}, health.ColonyHealthEvaluation{}, err
	}
	if len(history.Points) == 0 {
		return history, health.ColonyHealthEvaluation{}, nil
	}
	return history, history.Points[len(history.Points)-1].Evaluation, nil
}

// GetInternalHealthFacts returns only the persisted inspection facts needed
// by another trusted service to run the canonical health engine. It performs
// no ownership, entitlement, or health calculation work.
func (s *Service) GetInternalHealthFacts(ctx context.Context, hiveID uuid.UUID, to time.Time) ([]*inspection.Inspection, error) {
	reader, ok := s.inspections.(InternalHealthFactsReader)
	if !ok {
		return nil, fmt.Errorf("inspection repository does not support internal health facts")
	}
	return reader.ListAllByHiveInternalUpTo(ctx, hiveID, to)
}

// loadHealthEvidence loads hiveID's complete non-deleted inspection
// history exactly once and normalizes it into the flat evidence slice
// health.CalculateColonyHealth expects, alongside the raw inspections
// themselves (callers deriving history markers need both).
func (s *Service) loadHealthEvidence(ctx context.Context, userID, hiveID uuid.UUID) ([]health.HealthEvidence, []*inspection.Inspection, error) {
	reader, ok := s.inspections.(HealthInspectionReader)
	if !ok {
		return nil, nil, fmt.Errorf("inspection: repository does not support health evaluation history")
	}
	all, err := reader.ListAllByHive(ctx, userID, hiveID)
	if err != nil {
		return nil, nil, fmt.Errorf("inspection: list health evaluation history: %w", err)
	}

	var evidence []health.HealthEvidence
	for _, current := range all {
		if current == nil {
			return nil, nil, fmt.Errorf("inspection: health evaluation history contains nil inspection")
		}
		normalized := health.NormalizeInspection(toHealthInspection(current))
		evidence = append(evidence, normalized.HealthEvidence...)
	}
	return evidence, all, nil
}

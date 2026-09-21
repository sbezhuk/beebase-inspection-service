package inspection_test

// This file covers GetHiveHealthHistory: deriving one Colony Health v1
// point per calendar day from a hive's inspection history, gated on Pro
// entitlement. See application/inspection.Service.GetHiveHealthHistory
// and EntitlementResolver.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/health"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// --- fakes specific to this file ---

// fakeEntitlementResolver stands in for subscription-service: entitlement
// defaults to Pro for any token not explicitly set, so every test that
// isn't specifically exercising the Free gate doesn't need to configure
// one.
type fakeEntitlementResolver struct {
	mu        sync.Mutex
	byToken   map[string]string
	callCount int
}

func newFakeEntitlementResolver() *fakeEntitlementResolver {
	return &fakeEntitlementResolver{byToken: map[string]string{}}
}

func (f *fakeEntitlementResolver) setEntitlement(token, entitlement string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byToken[token] = entitlement
}

func (f *fakeEntitlementResolver) GetEntitlement(_ context.Context, accessToken string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	if entitlement, ok := f.byToken[accessToken]; ok {
		return entitlement, nil
	}
	return appinspection.EntitlementPro, nil
}

// countingRepo wraps *fakeRepo to count ListAllByHive calls, proving
// GetHiveHealthHistory loads a hive's inspection history exactly once
// regardless of how many daily points it derives from it - never once
// per day.
type countingRepo struct {
	*fakeRepo
	mu           sync.Mutex
	listAllCalls int
}

func (r *countingRepo) ListAllByHive(ctx context.Context, userID, hiveID uuid.UUID) ([]*inspection.Inspection, error) {
	r.mu.Lock()
	r.listAllCalls++
	r.mu.Unlock()
	return r.fakeRepo.ListAllByHive(ctx, userID, hiveID)
}

// fixedOrderRepo wraps *fakeRepo but answers ListAllByHive with exactly
// the slice it was constructed with, in that exact order, bypassing
// fakeRepo's own (InspectedAt, ID) sort entirely. This lets a test prove
// GetHiveHealthHistory's evidence supersession is independent of
// repository row order, rather than merely trusting fakeRepo's
// UUID-tiebreak sort to happen to agree across runs.
type fixedOrderRepo struct {
	*fakeRepo
	order []*inspection.Inspection
}

func (r *fixedOrderRepo) ListAllByHive(_ context.Context, _, _ uuid.UUID) ([]*inspection.Inspection, error) {
	return r.order, nil
}

func newTestHiveHealthHistoryService(repo *fakeRepo, verifier *fakeHiveVerifier, entitlement *fakeEntitlementResolver) *appinspection.Service {
	return appinspection.NewService(repo, verifier, newFakeMediaClient(), 14, entitlement)
}

func seedInspection(t *testing.T, repo *fakeRepo, userID, hiveID uuid.UUID, at time.Time, assessment *inspection.Assessment) *inspection.Inspection {
	t.Helper()
	created := inspection.New(userID, hiveID, at, "", inspection.TypeHealth)
	created.Assessment = assessment
	if err := repo.Create(context.Background(), created); err != nil {
		t.Fatalf("seed inspection: %v", err)
	}
	return created
}

// buildInspection constructs an inspection in memory without persisting
// it through any repository - used where a test needs to control the
// exact slice/order a fake repository answers with (see fixedOrderRepo),
// rather than relying on fakeRepo's own storage and sort order.
func buildInspection(userID, hiveID uuid.UUID, at time.Time, assessment *inspection.Assessment) *inspection.Inspection {
	created := inspection.New(userID, hiveID, at, "", inspection.TypeHealth)
	created.Assessment = assessment
	return created
}

func foodStoresAssessment(value inspection.FoodStores) *inspection.Assessment {
	return &inspection.Assessment{Version: 1, FoodStores: &value}
}

func queenStatusAssessment(value inspection.QueenStatus) *inspection.Assessment {
	return &inspection.Assessment{Version: 1, QueenStatus: &value}
}

// --- entitlement / ownership gating ---

func TestGetHiveHealthHistory_FreeUserRejected(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	entitlement := newFakeEntitlementResolver()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	entitlement.setEntitlement("token", appinspection.EntitlementFree)
	svc := newTestHiveHealthHistoryService(repo, verifier, entitlement)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.GetHiveHealthHistory(context.Background(), uuid.New(), "token", hiveID, from, from)
	if !errors.Is(err, appinspection.ErrHealthHistoryProRequired) {
		t.Fatalf("GetHiveHealthHistory error = %v, want ErrHealthHistoryProRequired", err)
	}
}

func TestGetHiveHealthHistory_ProUserAllowed(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	entitlement := newFakeEntitlementResolver()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	entitlement.setEntitlement("token", appinspection.EntitlementPro)
	svc := newTestHiveHealthHistoryService(repo, verifier, entitlement)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	result, err := svc.GetHiveHealthHistory(context.Background(), uuid.New(), "token", hiveID, from, from)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("len(Points) = %d, want 1", len(result.Points))
	}
}

// TestGetHiveHealthHistory_OwnershipRespected proves a hive the caller
// doesn't own is rejected with ErrHiveNotFound before entitlement is even
// consulted - a caller must not learn "you'd need Pro" about a hive that
// was never theirs.
func TestGetHiveHealthHistory_OwnershipRespected(t *testing.T) {
	verifier := newFakeHiveVerifier()
	entitlement := newFakeEntitlementResolver()
	entitlement.setEntitlement("attacker", appinspection.EntitlementFree)
	svc := newTestHiveHealthHistoryService(newFakeRepo(), verifier, entitlement)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.GetHiveHealthHistory(context.Background(), uuid.New(), "attacker", uuid.New(), from, from)
	if !errors.Is(err, appinspection.ErrHiveNotFound) {
		t.Fatalf("GetHiveHealthHistory error = %v, want ErrHiveNotFound", err)
	}
	if entitlement.callCount != 0 {
		t.Fatalf("entitlement resolver called %d times, want 0 (ownership must fail first)", entitlement.callCount)
	}
}

// --- shape / ordering ---

func TestGetHiveHealthHistory_InclusiveRangeChronologicalOrder(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 2)

	result, err := svc.GetHiveHealthHistory(context.Background(), uuid.New(), "token", hiveID, from, to)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	if len(result.Points) != 3 {
		t.Fatalf("len(Points) = %d, want 3 (inclusive of both endpoints)", len(result.Points))
	}
	want := []time.Time{from, from.AddDate(0, 0, 1), from.AddDate(0, 0, 2)}
	for i, point := range result.Points {
		if !point.Date.Equal(want[i]) {
			t.Fatalf("Points[%d].Date = %v, want %v (chronological, oldest first)", i, point.Date, want[i])
		}
	}
}

// TestGetHiveHealthHistory_MonthYearLeapBoundaries proves date stepping
// is correct across a leap day and a calendar year boundary, not just
// within one ordinary month.
func TestGetHiveHealthHistory_MonthYearLeapBoundaries(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	cases := []struct {
		name string
		from time.Time
		to   time.Time
		want []string
	}{
		{
			name: "leap day",
			from: time.Date(2028, 2, 28, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC),
			want: []string{"2028-02-28", "2028-02-29", "2028-03-01"},
		},
		{
			name: "year boundary",
			from: time.Date(2026, 12, 30, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
			want: []string{"2026-12-30", "2026-12-31", "2027-01-01", "2027-01-02"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.GetHiveHealthHistory(context.Background(), uuid.New(), "token", hiveID, tc.from, tc.to)
			if err != nil {
				t.Fatalf("GetHiveHealthHistory: %v", err)
			}
			if len(result.Points) != len(tc.want) {
				t.Fatalf("len(Points) = %d, want %d", len(result.Points), len(tc.want))
			}
			for i, point := range result.Points {
				got := point.Date.Format("2006-01-02")
				if got != tc.want[i] {
					t.Fatalf("Points[%d].Date = %s, want %s", i, got, tc.want[i])
				}
			}
		})
	}
}

func TestGetHiveHealthHistory_NoQualifyingInspectionsIsUnknownNone(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 4)
	result, err := svc.GetHiveHealthHistory(context.Background(), uuid.New(), "token", hiveID, from, to)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	for _, point := range result.Points {
		if point.Evaluation.State != health.DimensionUnknown || point.Evaluation.Coverage != health.CoverageNone {
			t.Fatalf("point %s = (%q, %q), want (UNKNOWN, NONE)", point.Date.Format("2006-01-02"), point.Evaluation.State, point.Evaluation.Coverage)
		}
	}
	if len(result.Inspections) != 0 {
		t.Fatalf("Inspections = %d, want 0", len(result.Inspections))
	}
}

// --- N+1 avoidance ---

func TestGetHiveHealthHistory_LoadsInspectionHistoryOnce(t *testing.T) {
	inner := newFakeRepo()
	repo := &countingRepo{fakeRepo: inner}
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	seedInspection(t, inner, userID, hiveID, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), foodStoresAssessment(inspection.FoodStoresAdequate))

	svc := appinspection.NewService(repo, verifier, newFakeMediaClient(), 14, newFakeEntitlementResolver())

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 199) // 200 daily points
	if _, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, from, to); err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	if repo.listAllCalls != 1 {
		t.Fatalf("ListAllByHive called %d times for 200 daily points, want exactly 1", repo.listAllCalls)
	}
}

// --- future inspections must not leak backward ---

func TestGetHiveHealthHistory_FutureInspectionsDoNotAffectEarlierPoints(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	day10 := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	day20 := day10.AddDate(0, 0, 10)
	seedInspection(t, repo, userID, hiveID, day10, foodStoresAssessment(inspection.FoodStoresLow))
	seedInspection(t, repo, userID, hiveID, day20, foodStoresAssessment(inspection.FoodStoresAdequate))

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	day5 := day10.AddDate(0, 0, -5)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day5, day10)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	// day5..day9: before the first inspection even exists - no evidence.
	for _, point := range result.Points[:5] {
		nutrition := findDimension(point.Evaluation.Dimensions, health.DimensionNutrition)
		if nutrition.State != health.DimensionUnknown {
			t.Fatalf("point %s nutrition state = %q, want UNKNOWN (no evidence yet)", point.Date.Format("2006-01-02"), nutrition.State)
		}
	}

	// day10: the LOW inspection is visible (inclusive), but the day20
	// ADEQUATE inspection is 10 days in the future relative to day10 and
	// must not have overridden it.
	last := result.Points[len(result.Points)-1]
	nutrition := findDimension(last.Evaluation.Dimensions, health.DimensionNutrition)
	if nutrition.State != health.DimensionConcern {
		t.Fatalf("point %s nutrition state = %q, want CONCERN (future ADEQUATE evidence must not leak backward)", last.Date.Format("2006-01-02"), nutrition.State)
	}
}

// TestGetHiveHealthHistory_AdjacentDaySupersessionNoOneDayLag is the
// direct regression for the exact scenario the date-semantics audit
// flagged as untested: two inspections on *adjacent* calendar days,
// touching the same (dimension, sourceField), with opposite signals.
// 2026-09-19 must reflect the favorable reading and 2026-09-20 must
// reflect the concerning one *on that same day* - there must be no
// off-by-one where the second inspection only becomes visible on
// 2026-09-21. This complements
// TestGetHiveHealthHistory_FutureInspectionsDoNotAffectEarlierPoints,
// which only exercised a 10-day gap.
func TestGetHiveHealthHistory_AdjacentDaySupersessionNoOneDayLag(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	day19 := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	day20 := day19.AddDate(0, 0, 1)
	seedInspection(t, repo, userID, hiveID, day19, foodStoresAssessment(inspection.FoodStoresAdequate)) // favorable
	seedInspection(t, repo, userID, hiveID, day20, foodStoresAssessment(inspection.FoodStoresLow))      // concern, same sourceField

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day19, day20)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	if len(result.Points) != 2 {
		t.Fatalf("len(Points) = %d, want 2", len(result.Points))
	}

	day19Nutrition := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionNutrition)
	day20Nutrition := findDimension(result.Points[1].Evaluation.Dimensions, health.DimensionNutrition)

	if day19Nutrition.State != health.DimensionGood {
		t.Fatalf("2026-09-19 nutrition state = %q, want GOOD (must use inspection A, not the next day's B)", day19Nutrition.State)
	}
	if day20Nutrition.State != health.DimensionConcern {
		t.Fatalf("2026-09-20 nutrition state = %q, want CONCERN (B must be visible on its own day, no one-day lag)", day20Nutrition.State)
	}
}

// --- inspection markers ---

// TestGetHiveHealthHistory_InspectionMarkersIncludeRangeBoundaries proves
// the inspections[] marker filter is inclusive on both ends, exactly like
// the points range: an inspection dated precisely `from` or precisely
// `to` must appear, and anything strictly outside must not.
func TestGetHiveHealthHistory_InspectionMarkersIncludeRangeBoundaries(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	onFrom := seedInspection(t, repo, userID, hiveID, from, foodStoresAssessment(inspection.FoodStoresAdequate))
	onTo := seedInspection(t, repo, userID, hiveID, to, foodStoresAssessment(inspection.FoodStoresAdequate))
	beforeRange := seedInspection(t, repo, userID, hiveID, from.AddDate(0, 0, -1), foodStoresAssessment(inspection.FoodStoresAdequate))
	afterRange := seedInspection(t, repo, userID, hiveID, to.AddDate(0, 0, 1), foodStoresAssessment(inspection.FoodStoresAdequate))

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, from, to)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	included := make(map[uuid.UUID]bool, len(result.Inspections))
	for _, i := range result.Inspections {
		included[i.ID] = true
	}

	if !included[onFrom.ID] {
		t.Error("inspection dated exactly `from` (2026-09-01) is missing from markers, want included")
	}
	if !included[onTo.ID] {
		t.Error("inspection dated exactly `to` (2026-09-30) is missing from markers, want included")
	}
	if included[beforeRange.ID] {
		t.Error("inspection dated before `from` (2026-08-31) is present in markers, want excluded")
	}
	if included[afterRange.ID] {
		t.Error("inspection dated after `to` (2026-10-01) is present in markers, want excluded")
	}
	if len(result.Inspections) != 2 {
		t.Fatalf("len(Inspections) = %d, want 2 (exactly the two boundary-dated inspections)", len(result.Inspections))
	}
}

// --- multiple inspections on the same calendar date ---

// TestGetHiveHealthHistory_SameDateConflictingEvidenceIsCoEqualAndOrderIndependent
// documents the existing (not newly introduced) semantics for two
// inspections sharing one calendar date and the same (dimension,
// sourceField): there is no "latest wins" tiebreaker - both readings are
// treated as simultaneously true evidence and blended by the existing
// evaluator rules. A DimensionState of WATCH can only arise here from
// counting *both* the favorable and the concerning reading together
// (health.stateFor: hasConcern && hasFavorable => WATCH); if either
// reading alone controlled the result, the state would be GOOD or
// CONCERN, never WATCH. The test runs both physical orderings of the two
// inspections through a repository double that returns them in an exact,
// caller-chosen order (fixedOrderRepo) - not fakeRepo's own sort - to
// prove the outcome does not depend on row order, without relying on
// created_at, UUID ordering, or any inspection time-of-day (none exists).
func TestGetHiveHealthHistory_SameDateConflictingEvidenceIsCoEqualAndOrderIndependent(t *testing.T) {
	userID := uuid.New()
	hiveID := uuid.New()
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	favorable := buildInspection(userID, hiveID, day, foodStoresAssessment(inspection.FoodStoresAdequate))
	concern := buildInspection(userID, hiveID, day, foodStoresAssessment(inspection.FoodStoresLow))

	cases := []struct {
		name  string
		order []*inspection.Inspection
	}{
		{"favorable scanned before concern", []*inspection.Inspection{favorable, concern}},
		{"concern scanned before favorable", []*inspection.Inspection{concern, favorable}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fixedOrderRepo{fakeRepo: newFakeRepo(), order: tc.order}
			verifier := newFakeHiveVerifier()
			verifier.allow("token", hiveID)
			svc := appinspection.NewService(repo, verifier, newFakeMediaClient(), 14, newFakeEntitlementResolver())

			result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day, day)
			if err != nil {
				t.Fatalf("GetHiveHealthHistory: %v", err)
			}
			nutrition := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionNutrition)
			if nutrition.State != health.DimensionWatch {
				t.Fatalf("%s: nutrition state = %q, want WATCH (both same-day readings must count as co-equal evidence)", tc.name, nutrition.State)
			}
		})
	}
}

// --- matches the live snapshot for "today" ---

// TestGetHiveHealthHistory_LastPointMatchesLiveHealthForSameClock proves
// that when to is the same calendar day GetHiveHealth is evaluated for
// with the identical clock value, the history endpoint's last point and
// the live endpoint's result agree exactly - the two must never silently
// diverge for the "today" point a client would show as the current
// reading on the graph.
func TestGetHiveHealthHistory_LastPointMatchesLiveHealthForSameClock(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	today := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	seedInspection(t, repo, userID, hiveID, today.AddDate(0, 0, -3), queenStatusAssessment(inspection.QueenStatusHealthy))
	seedInspection(t, repo, userID, hiveID, today.AddDate(0, 0, -1), foodStoresAssessment(inspection.FoodStoresAdequate))

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	live, err := svc.GetHiveHealth(context.Background(), userID, "token", hiveID, today)
	if err != nil {
		t.Fatalf("GetHiveHealth: %v", err)
	}
	history, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, today.AddDate(0, 0, -5), today)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	last := history.Points[len(history.Points)-1]
	if !last.Date.Equal(today) {
		t.Fatalf("last point date = %v, want %v", last.Date, today)
	}
	if last.Evaluation.State != live.State || last.Evaluation.Coverage != live.Coverage {
		t.Fatalf("history last point = (%q, %q), want live (%q, %q)", last.Evaluation.State, last.Evaluation.Coverage, live.State, live.Coverage)
	}
	for _, dimension := range live.Dimensions {
		historyDimension := findDimension(last.Evaluation.Dimensions, dimension.Dimension)
		if historyDimension.State != dimension.State || historyDimension.Coverage != dimension.Coverage {
			t.Fatalf("history dimension %s = (%q, %q), want live (%q, %q)", dimension.Dimension, historyDimension.State, historyDimension.Coverage, dimension.State, dimension.Coverage)
		}
	}
}

// --- recency boundaries ---

// TestGetHiveHealthHistory_NutritionTenDayCoverageBoundary proves
// Nutrition's 10-day CURRENT/RECENT boundary changes coverage from HIGH
// to MEDIUM the day after, with no new inspection - two nutrition-
// contributing observations recorded the same day both count as
// meaningful-current through day 10, and both drop to merely
// meaningful-recent from day 11 on.
func TestGetHiveHealthHistory_NutritionTenDayCoverageBoundary(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	recorded := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	feedingNo := inspection.FeedingNeedNo
	seedInspection(t, repo, userID, hiveID, recorded, &inspection.Assessment{Version: 1, FoodStores: ptr(inspection.FoodStoresAdequate), FeedingNeed: &feedingNo})

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	day10 := recorded.AddDate(0, 0, 10)
	day11 := recorded.AddDate(0, 0, 11)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day10, day11)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	atBoundary := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionNutrition)
	afterBoundary := findDimension(result.Points[1].Evaluation.Dimensions, health.DimensionNutrition)

	if atBoundary.Coverage != health.CoverageHigh {
		t.Fatalf("nutrition coverage at day 10 = %q, want HIGH", atBoundary.Coverage)
	}
	if afterBoundary.Coverage != health.CoverageMedium {
		t.Fatalf("nutrition coverage at day 11 = %q, want MEDIUM", afterBoundary.Coverage)
	}
	// State itself is unaffected by CURRENT vs RECENT - only STALE (30
	// days) drops evidence from consideration entirely.
	if atBoundary.State != health.DimensionGood || afterBoundary.State != health.DimensionGood {
		t.Fatalf("nutrition state = (%q, %q), want (GOOD, GOOD) unchanged across the 10-day boundary", atBoundary.State, afterBoundary.State)
	}
}

// TestGetHiveHealthHistory_OtherDimensionFourteenDayCoverageBoundary is
// the same shape of test as the Nutrition one above, for a dimension on
// the 14-day policy (QUEEN).
func TestGetHiveHealthHistory_OtherDimensionFourteenDayCoverageBoundary(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	recorded := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	normal := inspection.QueenConditionNormal
	seedInspection(t, repo, userID, hiveID, recorded, &inspection.Assessment{Version: 1, QueenStatus: ptr(inspection.QueenStatusHealthy), QueenCondition: &normal})

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	day14 := recorded.AddDate(0, 0, 14)
	day15 := recorded.AddDate(0, 0, 15)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day14, day15)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	atBoundary := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionQueen)
	afterBoundary := findDimension(result.Points[1].Evaluation.Dimensions, health.DimensionQueen)

	if atBoundary.Coverage != health.CoverageHigh {
		t.Fatalf("queen coverage at day 14 = %q, want HIGH", atBoundary.Coverage)
	}
	if afterBoundary.Coverage != health.CoverageMedium {
		t.Fatalf("queen coverage at day 15 = %q, want MEDIUM", afterBoundary.Coverage)
	}
}

// TestGetHiveHealthHistory_ThirtyDayStaleBoundaryChangesState proves the
// dimension (and aggregate) STATE itself changes at the 30-day stale
// boundary: evidence classified STALE is excluded from state
// determination entirely, unlike the CURRENT/RECENT boundaries above
// which only move coverage.
func TestGetHiveHealthHistory_ThirtyDayStaleBoundaryChangesState(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	recorded := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seedInspection(t, repo, userID, hiveID, recorded, queenStatusAssessment(inspection.QueenStatusHealthy))

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	day30 := recorded.AddDate(0, 0, 30)
	day31 := recorded.AddDate(0, 0, 31)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day30, day31)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	atBoundary := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionQueen)
	afterBoundary := findDimension(result.Points[1].Evaluation.Dimensions, health.DimensionQueen)

	if atBoundary.State != health.DimensionGood {
		t.Fatalf("queen state at day 30 = %q, want GOOD (age == RecentFor is still inclusive)", atBoundary.State)
	}
	if afterBoundary.State != health.DimensionUnknown {
		t.Fatalf("queen state at day 31 = %q, want UNKNOWN (evidence is now STALE)", afterBoundary.State)
	}

	// The same transition changes without any inspection occurring on
	// either day 30 or day 31 - only time passing.
	if len(result.Inspections) != 0 {
		t.Fatalf("Inspections in [day30, day31] = %d, want 0 (the only inspection was on day1)", len(result.Inspections))
	}
}

// TestGetHiveHealthHistory_RecencyAcrossYearBoundary proves age-in-days
// (and the resulting Nutrition CURRENT/RECENT coverage boundary) is
// computed correctly across a Dec 31 -> Jan 1 rollover, not merely that
// point dates/labels step correctly across it (see
// TestGetHiveHealthHistory_MonthYearLeapBoundaries, which only checks
// labels). The inspection is recorded 2026-12-21; 2026-12-31 is exactly
// 10 days later (Nutrition's CURRENT boundary), 2027-01-01 is 11.
func TestGetHiveHealthHistory_RecencyAcrossYearBoundary(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	recorded := time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC)
	feedingNo := inspection.FeedingNeedNo
	seedInspection(t, repo, userID, hiveID, recorded, &inspection.Assessment{Version: 1, FoodStores: ptr(inspection.FoodStoresAdequate), FeedingNeed: &feedingNo})

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	lastDayOfYear := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	firstDayOfNextYear := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, lastDayOfYear, firstDayOfNextYear)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	if len(result.Points) != 2 {
		t.Fatalf("len(Points) = %d, want 2", len(result.Points))
	}

	dec31 := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionNutrition)
	jan1 := findDimension(result.Points[1].Evaluation.Dimensions, health.DimensionNutrition)

	if dec31.Coverage != health.CoverageHigh {
		t.Fatalf("nutrition coverage on 2026-12-31 (age=10 days) = %q, want HIGH", dec31.Coverage)
	}
	if jan1.Coverage != health.CoverageMedium {
		t.Fatalf("nutrition coverage on 2027-01-01 (age=11 days) = %q, want MEDIUM - the year rollover must not distort day-counting", jan1.Coverage)
	}
	if dec31.State != health.DimensionGood || jan1.State != health.DimensionGood {
		t.Fatalf("nutrition state = (%q, %q), want (GOOD, GOOD) unchanged across the year boundary", dec31.State, jan1.State)
	}
}

// TestGetHiveHealthHistory_RecencyAcrossLeapDay proves age-in-days is
// computed correctly when the elapsed span includes February 29 of a
// leap year - if the implementation ever assumed a fixed 28-day February
// instead of deriving age from real calendar dates, this would silently
// shift the 14-day boundary by one day. The inspection is recorded
// 2028-02-15 (2028 is a leap year); 2028-02-29 is exactly 14 days later
// (the QUEEN/other-dimension CURRENT boundary, and the leap day itself),
// 2028-03-01 is 15.
func TestGetHiveHealthHistory_RecencyAcrossLeapDay(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	recorded := time.Date(2028, 2, 15, 0, 0, 0, 0, time.UTC)
	normal := inspection.QueenConditionNormal
	seedInspection(t, repo, userID, hiveID, recorded, &inspection.Assessment{Version: 1, QueenStatus: ptr(inspection.QueenStatusHealthy), QueenCondition: &normal})

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	leapDay := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	dayAfter := time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, leapDay, dayAfter)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}
	if len(result.Points) != 2 {
		t.Fatalf("len(Points) = %d, want 2", len(result.Points))
	}

	feb29 := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionQueen)
	mar1 := findDimension(result.Points[1].Evaluation.Dimensions, health.DimensionQueen)

	if feb29.Coverage != health.CoverageHigh {
		t.Fatalf("queen coverage on 2028-02-29 (age=14 days, the leap day) = %q, want HIGH", feb29.Coverage)
	}
	if mar1.Coverage != health.CoverageMedium {
		t.Fatalf("queen coverage on 2028-03-01 (age=15 days) = %q, want MEDIUM - the leap day must still count as exactly one day", mar1.Coverage)
	}
	if feb29.State != health.DimensionGood || mar1.State != health.DimensionGood {
		t.Fatalf("queen state = (%q, %q), want (GOOD, GOOD) unchanged across the leap day", feb29.State, mar1.State)
	}
}

// --- explainability provenance (ContributingEvidence) ---

// seedRoutineInspection mirrors seedInspection but persists a ROUTINE
// inspection instead of HEALTH - required here because
// health.NormalizeInspection only produces ColonyStrength evidence for
// ROUTINE/SEASONAL inspections (see its own switch on input.Type), and this
// file's provenance tests specifically need STRENGTH-dimension evidence.
func seedRoutineInspection(t *testing.T, repo *fakeRepo, userID, hiveID uuid.UUID, at time.Time, assessment *inspection.Assessment) *inspection.Inspection {
	t.Helper()
	created := inspection.New(userID, hiveID, at, "", inspection.TypeRoutine)
	created.Assessment = assessment
	if err := repo.Create(context.Background(), created); err != nil {
		t.Fatalf("seed routine inspection: %v", err)
	}
	return created
}

// colonyStrengthAssessment builds a minimal single-field assessment - used
// below to isolate exactly one (dimension, field) fact per inspection so
// supersession is unambiguous.
func colonyStrengthAssessment(value inspection.ColonyStrength) *inspection.Assessment {
	return &inspection.Assessment{Version: 1, ColonyStrength: &value}
}

// fullPositiveRoutineAssessment is the exact assessment from the Health
// History audit that reported an unexplained CONCERN before an all-positive
// ROUTINE inspection.
func fullPositiveRoutineAssessment() *inspection.Assessment {
	strength := inspection.ColonyStrengthStrong
	queen := inspection.QueenStatusHealthy
	brood := inspection.BroodStatusHealthy
	food := inspection.FoodStoresAbundant
	concerns := inspection.HealthConcernsNone
	return &inspection.Assessment{
		Version:        1,
		ColonyStrength: &strength,
		QueenStatus:    &queen,
		BroodStatus:    &brood,
		FoodStores:     &food,
		HealthConcerns: &concerns,
	}
}

// TestGetHiveHealthHistory_SinglePositiveRoutineInspectionExplainsAsGood is
// the direct regression for the audited scenario: a single, entirely
// positive ROUTINE inspection must explain as GOOD on and after its own
// date (with STRENGTH/QUEEN/BROOD/NUTRITION all attributing to that exact
// inspection), and UNKNOWN with no source at all before it - never CONCERN.
func TestGetHiveHealthHistory_SinglePositiveRoutineInspectionExplainsAsGood(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	inspectedAt := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	seeded := seedRoutineInspection(t, repo, userID, hiveID, inspectedAt, fullPositiveRoutineAssessment())

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	dayBefore := inspectedAt.AddDate(0, 0, -1)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, dayBefore, inspectedAt)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	before := result.Points[0]
	if before.Evaluation.State != health.DimensionUnknown {
		t.Fatalf("day before the only inspection: state = %q, want UNKNOWN", before.Evaluation.State)
	}
	for _, dimension := range before.Evaluation.Dimensions {
		if len(dimension.ContributingEvidence) != 0 {
			t.Errorf("day before the only inspection: dimension %s has %d contributing evidence, want 0 (nothing exists yet)", dimension.Dimension, len(dimension.ContributingEvidence))
		}
	}

	on := result.Points[1]
	if on.Evaluation.State != health.DimensionGood {
		t.Fatalf("inspection date: state = %q, want GOOD", on.Evaluation.State)
	}
	for _, name := range []health.HealthDimension{health.DimensionStrength, health.DimensionQueen, health.DimensionBrood, health.DimensionNutrition} {
		dimension := findDimension(on.Evaluation.Dimensions, name)
		if dimension.State != health.DimensionGood {
			t.Fatalf("inspection date: %s state = %q, want GOOD", name, dimension.State)
		}
		if len(dimension.ContributingEvidence) != 1 {
			t.Fatalf("inspection date: %s contributing evidence = %d, want exactly 1", name, len(dimension.ContributingEvidence))
		}
		source := dimension.ContributingEvidence[0].Source
		if source.InspectionID == nil || *source.InspectionID != seeded.ID {
			t.Errorf("inspection date: %s source inspection = %v, want %s", name, source.InspectionID, seeded.ID)
		}
		if !source.OccurredAt.Equal(inspectedAt) {
			t.Errorf("inspection date: %s source occurredAt = %v, want %v", name, source.OccurredAt, inspectedAt)
		}
	}
	pests := findDimension(on.Evaluation.Dimensions, health.DimensionPestsAndDisease)
	if pests.State != health.DimensionUnknown || len(pests.ContributingEvidence) != 0 {
		t.Fatalf("inspection date: PESTS_AND_DISEASE = (%q, %d sources), want (UNKNOWN, 0) - no evidence was ever recorded for it", pests.State, len(pests.ContributingEvidence))
	}
}

// TestGetHiveHealthHistory_PreWindowSourceExplainsInWindowConcern is the
// direct regression for the audited root cause: an older inspection dated
// BEFORE the requested [from, to] window can still be the explained source
// of an in-window CONCERN point, and that same source must stop being
// reported the moment a newer inspection supersedes it on the identical
// (dimension, field) - even though the newer inspection is the only one
// ever visible as an in-window marker.
func TestGetHiveHealthHistory_PreWindowSourceExplainsInWindowConcern(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	older := seedRoutineInspection(t, repo, userID, hiveID, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), colonyStrengthAssessment(inspection.ColonyStrengthWeak))
	newer := seedRoutineInspection(t, repo, userID, hiveID, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), fullPositiveRoutineAssessment())

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	// The requested window starts AFTER the older inspection - it can never
	// appear as an inspections[] marker, only newer can.
	from := older.InspectedAt.AddDate(0, 0, 5)
	to := newer.InspectedAt.AddDate(0, 0, 1)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, from, to)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	included := make(map[uuid.UUID]bool, len(result.Inspections))
	for _, i := range result.Inspections {
		included[i.ID] = true
	}
	if included[older.ID] {
		t.Fatal("older inspection unexpectedly appears as an in-window marker")
	}
	if !included[newer.ID] {
		t.Fatal("newer inspection must appear as an in-window marker")
	}

	dayBeforeNewer := findPoint(t, result.Points, newer.InspectedAt.AddDate(0, 0, -1))
	strengthBefore := findDimension(dayBeforeNewer.Evaluation.Dimensions, health.DimensionStrength)
	if strengthBefore.State != health.DimensionConcern {
		t.Fatalf("day before the newer inspection: STRENGTH state = %q, want CONCERN (driven by the older, out-of-window inspection)", strengthBefore.State)
	}
	if len(strengthBefore.ContributingEvidence) != 1 {
		t.Fatalf("day before the newer inspection: STRENGTH contributing evidence = %d, want exactly 1", len(strengthBefore.ContributingEvidence))
	}
	beforeSource := strengthBefore.ContributingEvidence[0].Source
	if beforeSource.InspectionID == nil || *beforeSource.InspectionID != older.ID {
		t.Errorf("day before the newer inspection: STRENGTH source = %v, want the OLDER inspection %s (even though it has no visible marker)", beforeSource.InspectionID, older.ID)
	}
	if !beforeSource.OccurredAt.Equal(older.InspectedAt) {
		t.Errorf("day before the newer inspection: STRENGTH source occurredAt = %v, want %v", beforeSource.OccurredAt, older.InspectedAt)
	}

	onNewer := findPoint(t, result.Points, newer.InspectedAt)
	strengthOn := findDimension(onNewer.Evaluation.Dimensions, health.DimensionStrength)
	if strengthOn.State != health.DimensionGood {
		t.Fatalf("newer inspection's own date: STRENGTH state = %q, want GOOD", strengthOn.State)
	}
	if len(strengthOn.ContributingEvidence) != 1 {
		t.Fatalf("newer inspection's own date: STRENGTH contributing evidence = %d, want exactly 1 (the older same-field fact must be fully superseded, not blended)", len(strengthOn.ContributingEvidence))
	}
	onSource := strengthOn.ContributingEvidence[0].Source
	if onSource.InspectionID == nil || *onSource.InspectionID != newer.ID {
		t.Errorf("newer inspection's own date: STRENGTH source = %v, want the NEWER inspection %s, never the superseded older one", onSource.InspectionID, newer.ID)
	}
}

// TestGetHiveHealthHistory_FutureInspectionNeverReportedAsSource extends
// TestGetHiveHealthHistory_FutureInspectionsDoNotAffectEarlierPoints's own
// scenario to explicitly assert on provenance, not just state: an
// inspection dated after the evaluated day must never appear in that day's
// ContributingEvidence, even though it exists in the repository.
func TestGetHiveHealthHistory_FutureInspectionNeverReportedAsSource(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	day10 := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	day20 := day10.AddDate(0, 0, 10)
	early := seedInspection(t, repo, userID, hiveID, day10, foodStoresAssessment(inspection.FoodStoresLow))
	future := seedInspection(t, repo, userID, hiveID, day20, foodStoresAssessment(inspection.FoodStoresAdequate))

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day10, day10)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	nutrition := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionNutrition)
	if nutrition.State != health.DimensionConcern {
		t.Fatalf("day10 nutrition state = %q, want CONCERN", nutrition.State)
	}
	for _, evidence := range nutrition.ContributingEvidence {
		if evidence.Source.InspectionID != nil && *evidence.Source.InspectionID == future.ID {
			t.Fatal("day10 nutrition ContributingEvidence includes the FUTURE inspection - future evidence must never be reported as a source")
		}
	}
	if len(nutrition.ContributingEvidence) != 1 || nutrition.ContributingEvidence[0].Source.InspectionID == nil || *nutrition.ContributingEvidence[0].Source.InspectionID != early.ID {
		t.Fatalf("day10 nutrition ContributingEvidence = %+v, want exactly the day10 (early) inspection", nutrition.ContributingEvidence)
	}
}

// TestGetHiveHealthHistory_StaleEvidenceNotReportedAsActiveSource extends
// TestGetHiveHealthHistory_ThirtyDayStaleBoundaryChangesState: the day
// after the 30-day stale boundary must not report the now-stale inspection
// as if it still actively explains a state - since the state itself is
// UNKNOWN, there is no "active" state for it to (mis)explain.
func TestGetHiveHealthHistory_StaleEvidenceNotReportedAsActiveSource(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	recorded := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seedInspection(t, repo, userID, hiveID, recorded, queenStatusAssessment(inspection.QueenStatusHealthy))

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	day31 := recorded.AddDate(0, 0, 31)
	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day31, day31)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	queen := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionQueen)
	if queen.State != health.DimensionUnknown {
		t.Fatalf("day31 queen state = %q, want UNKNOWN (evidence is stale)", queen.State)
	}
	// The evaluator may still surface the stale evidence for explanatory
	// purposes (see evaluateDimension's UNKNOWN fallback) - the contract
	// here is only that it can never be mistaken for an ACTIVE source of a
	// non-UNKNOWN state, which is already impossible since the state is
	// UNKNOWN. A client must render this as "no recent evidence", never
	// "based on inspection from <date>".
	t.Logf("day31 queen ContributingEvidence (informational, may be non-empty stale evidence): %+v", queen.ContributingEvidence)
}

// TestGetHiveHealthHistory_MultipleGenuineSourcesArePreserved proves that
// when two DIFFERENT (dimension, field) facts genuinely both contribute to
// one dimension's state, both are preserved in ContributingEvidence rather
// than one being arbitrarily dropped.
func TestGetHiveHealthHistory_MultipleGenuineSourcesArePreserved(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	normal := inspection.QueenConditionNormal
	seeded := seedInspection(t, repo, userID, hiveID, day, &inspection.Assessment{
		Version:        1,
		QueenStatus:    ptr(inspection.QueenStatusHealthy),
		QueenCondition: &normal,
	})

	svc := newTestHiveHealthHistoryService(repo, verifier, newFakeEntitlementResolver())

	result, err := svc.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, day, day)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	queen := findDimension(result.Points[0].Evaluation.Dimensions, health.DimensionQueen)
	if queen.State != health.DimensionGood || len(queen.ContributingEvidence) != 2 {
		t.Fatalf("queen = (%q, %d contributing), want (GOOD, 2) - both queenStatus and queenCondition genuinely contributed", queen.State, len(queen.ContributingEvidence))
	}
	fields := map[health.SourceField]bool{}
	for _, evidence := range queen.ContributingEvidence {
		fields[evidence.Source.SourceField] = true
		if evidence.Source.InspectionID == nil || *evidence.Source.InspectionID != seeded.ID {
			t.Errorf("contributing evidence source = %v, want %s", evidence.Source.InspectionID, seeded.ID)
		}
	}
	if !fields[health.SourceFieldQueenStatus] || !fields[health.SourceFieldQueenCondition] {
		t.Fatalf("contributing fields = %v, want both queenStatus and queenCondition preserved", fields)
	}
}

// findPoint locates the HealthHistoryPoint for the exact calendar date -
// test helper for the provenance tests above, which need to inspect a
// specific day within a multi-day range rather than relying on slice index.
func findPoint(t *testing.T, points []appinspection.HealthHistoryPoint, date time.Time) appinspection.HealthHistoryPoint {
	t.Helper()
	for _, point := range points {
		if point.Date.Equal(date) {
			return point
		}
	}
	t.Fatalf("no history point for date %s", date.Format("2006-01-02"))
	return appinspection.HealthHistoryPoint{}
}

func ptr[T any](v T) *T { return &v }

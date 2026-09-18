package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sbezhuk/beebase-common/pagination"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// minSearchLength is the minimum number of characters required for the
// search term to be applied. Shorter terms produce noisy results and put
// unnecessary load on the database.
const minSearchLength = 3

// createdAtOrderClause returns the ORDER BY clause for a list query. When
// sortOrder is nil, defaultClause (the query's normal, pre-existing order)
// is used unchanged; otherwise the list is ordered by creation date in the
// requested direction, with id tied to the same direction as a stable
// tiebreaker (matching the convention every other ORDER BY in this
// repository already follows).
func createdAtOrderClause(sortOrder *string, defaultClause string) string {
	if sortOrder == nil {
		return defaultClause
	}
	dir := "ASC"
	if *sortOrder == "desc" {
		dir = "DESC"
	}
	return fmt.Sprintf("created_at %s, id %s", dir, dir)
}

// InspectionRepository implements domain/inspection.Repository against
// PostgreSQL. Every method scopes its query by user_id, so a user can
// never read or write an inspection they don't own: there's no separate
// ownership-check step to forget.
type InspectionRepository struct {
	db Querier
}

// NewInspectionRepository returns an InspectionRepository backed by db.
func NewInspectionRepository(db Querier) *InspectionRepository {
	return &InspectionRepository{db: db}
}

func (r *InspectionRepository) Create(ctx context.Context, i *inspection.Inspection) error {
	const q = `
		INSERT INTO inspections (id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at,
			assessment_version, colony_strength, queen_status, brood_status, food_stores, health_concerns)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	args := []any{i.ID, i.HiveID, i.UserID, i.InspectedAt, i.Notes, i.Type, images(i.Images), i.CreatedAt, i.UpdatedAt}
	args = append(args, assessmentArgs(i.Assessment)...)
	_, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("postgres: create inspection: %w", err)
	}

	return nil
}

func assessmentArgs(a *inspection.Assessment) []any {
	if a == nil {
		return []any{nil, nil, nil, nil, nil, nil}
	}
	return []any{a.Version, nullableAssessmentValue(a.ColonyStrength), nullableAssessmentValue(a.QueenStatus), nullableAssessmentValue(a.BroodStatus), nullableAssessmentValue(a.FoodStores), nullableAssessmentValue(a.HealthConcerns)}
}

func nullableAssessmentValue[T ~string](v *T) any {
	if v == nil {
		return nil
	}
	return string(*v)
}

func scanAssessment(version *int, colony, queen, brood, food, health *string) *inspection.Assessment {
	if version == nil && colony == nil && queen == nil && brood == nil && food == nil && health == nil {
		return nil
	}
	a := &inspection.Assessment{Version: *version}
	if colony != nil {
		v := inspection.ColonyStrength(*colony)
		a.ColonyStrength = &v
	}
	if queen != nil {
		v := inspection.QueenStatus(*queen)
		a.QueenStatus = &v
	}
	if brood != nil {
		v := inspection.BroodStatus(*brood)
		a.BroodStatus = &v
	}
	if food != nil {
		v := inspection.FoodStores(*food)
		a.FoodStores = &v
	}
	if health != nil {
		v := inspection.HealthConcerns(*health)
		a.HealthConcerns = &v
	}
	return a
}

// images coalesces a nil slice to an empty one - the images column is NOT
// NULL, and pgx would otherwise encode a nil Go slice as SQL NULL.
func images(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

func (r *InspectionRepository) GetByID(ctx context.Context, userID, inspectionID uuid.UUID) (*inspection.Inspection, error) {
	const q = `
		SELECT id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at, deleted_at,
			assessment_version, colony_strength, queen_status, brood_status, food_stores, health_concerns
		FROM inspections
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var i inspection.Inspection
	var version *int
	var colony, queen, brood, food, health *string

	err := r.db.QueryRow(ctx, q, inspectionID, userID).Scan(
		&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.Type, &i.Images, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt, &version, &colony, &queen, &brood, &food, &health,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, inspection.ErrNotFound
		}
		return nil, fmt.Errorf("postgres: get inspection: %w", err)
	}

	i.Assessment = scanAssessment(version, colony, queen, brood, food, health)
	return &i, nil
}

func (r *InspectionRepository) ListByHive(ctx context.Context, userID, hiveID uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	return r.list(ctx, userID, &hiveID, p, search, typ, dateFrom, dateTo, sortOrder)
}

func (r *InspectionRepository) ListByUser(ctx context.Context, userID uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	return r.list(ctx, userID, nil, p, search, typ, dateFrom, dateTo, sortOrder)
}

// list is the shared implementation behind ListByHive (hiveID non-nil) and
// ListByUser (hiveID nil). search, typ, and dateFrom/dateTo are optional
// filters applied together with AND semantics; placeholders are numbered
// dynamically since which filters are present varies per call. dateFrom/dateTo
// are optional domain-date filters for either list scope.
func (r *InspectionRepository) list(ctx context.Context, userID uuid.UUID, hiveID *uuid.UUID, p pagination.Params, search *string, typ *inspection.Type, dateFrom, dateTo *time.Time, sortOrder *string) ([]*inspection.Inspection, int, error) {
	countQ := `
		SELECT count(*)
		FROM inspections
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	q := `
		SELECT id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at, deleted_at,
			assessment_version, colony_strength, queen_status, brood_status, food_stores, health_concerns
		FROM inspections
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	countArgs := []any{userID}
	argIdx := 2

	if hiveID != nil {
		cond := fmt.Sprintf(" AND hive_id = $%d", argIdx)
		countQ += cond
		q += cond
		countArgs = append(countArgs, *hiveID)
		argIdx++
	}

	if typ != nil {
		cond := fmt.Sprintf(" AND type = $%d", argIdx)
		countQ += cond
		q += cond
		countArgs = append(countArgs, *typ)
		argIdx++
	}

	if dateFrom != nil {
		cond := fmt.Sprintf(" AND inspected_at >= $%d", argIdx)
		countQ += cond
		q += cond
		countArgs = append(countArgs, dateFrom.Format("2006-01-02"))
		argIdx++
	}

	if dateTo != nil {
		cond := fmt.Sprintf(" AND inspected_at < $%d", argIdx)
		countQ += cond
		q += cond
		countArgs = append(countArgs, dateTo.Format("2006-01-02"))
		argIdx++
	}

	listArgs := make([]any, len(countArgs))
	copy(listArgs, countArgs)

	if search != nil && len(*search) >= minSearchLength {
		pattern := "%" + *search + "%"
		cond := fmt.Sprintf(" AND notes ILIKE $%d", argIdx)
		countQ += cond
		q += cond
		countArgs = append(countArgs, pattern)
		listArgs = append(listArgs, pattern)
		argIdx++
	}

	q += fmt.Sprintf(`
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, createdAtOrderClause(sortOrder, "inspected_at ASC, id ASC"), argIdx, argIdx+1)
	listArgs = append(listArgs, p.Limit, p.Offset())

	var total int
	if err := r.db.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: count inspections: %w", err)
	}

	rows, err := r.db.Query(ctx, q, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list inspections: %w", err)
	}
	defer rows.Close()

	inspections := []*inspection.Inspection{}
	for rows.Next() {
		var i inspection.Inspection
		var version *int
		var colony, queen, brood, food, health *string
		if err := rows.Scan(&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.Type, &i.Images, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt, &version, &colony, &queen, &brood, &food, &health); err != nil {
			return nil, 0, fmt.Errorf("postgres: scan inspection: %w", err)
		}
		i.Assessment = scanAssessment(version, colony, queen, brood, food, health)
		inspections = append(inspections, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list inspections: %w", err)
	}

	return inspections, total, nil
}

func (r *InspectionRepository) Update(ctx context.Context, i *inspection.Inspection) error {
	const q = `
		UPDATE inspections
		SET inspected_at = $1, notes = $2, type = $3, images = $4, updated_at = $5,
			assessment_version = $6, colony_strength = $7, queen_status = $8, brood_status = $9, food_stores = $10, health_concerns = $11
		WHERE id = $12 AND user_id = $13 AND deleted_at IS NULL
	`

	args := []any{i.InspectedAt, i.Notes, i.Type, images(i.Images), i.UpdatedAt}
	args = append(args, assessmentArgs(i.Assessment)...)
	args = append(args, i.ID, i.UserID)
	tag, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("postgres: update inspection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return inspection.ErrNotFound
	}

	return nil
}

func (r *InspectionRepository) Delete(ctx context.Context, userID, inspectionID uuid.UUID) error {
	const q = `
		UPDATE inspections
		SET deleted_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	tag, err := r.db.Exec(ctx, q, inspectionID, userID)
	if err != nil {
		return fmt.Errorf("postgres: delete inspection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return inspection.ErrNotFound
	}

	return nil
}

func (r *InspectionRepository) DeleteAllByUserHard(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM inspections WHERE user_id = $1`, userID)
	return err
}

func (r *InspectionRepository) DeleteByHive(ctx context.Context, userID, hiveID uuid.UUID) ([]uuid.UUID, int64, error) {
	const q = `DELETE FROM inspections WHERE hive_id = $1 AND user_id = $2 RETURNING images`

	rows, err := r.db.Query(ctx, q, hiveID, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: delete inspections by hive: %w", err)
	}
	defer rows.Close()

	var allImages []uuid.UUID
	var count int64
	for rows.Next() {
		var rowImages []uuid.UUID
		if err := rows.Scan(&rowImages); err != nil {
			return nil, 0, fmt.Errorf("postgres: scan deleted inspection images: %w", err)
		}
		allImages = append(allImages, rowImages...)
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: delete inspections by hive: %w", err)
	}

	return allImages, count, nil
}

func (r *InspectionRepository) ListIDsByHive(ctx context.Context, userID, hiveID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM inspections WHERE hive_id=$1 AND user_id=$2`, hiveID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *InspectionRepository) LatestInspectedAtByHive(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]time.Time, error) {
	const q = `
		SELECT hive_id, max(inspected_at)
		FROM inspections
		WHERE user_id = $1 AND deleted_at IS NULL
		GROUP BY hive_id
	`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: latest inspected_at by hive: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]time.Time)
	for rows.Next() {
		var hiveID uuid.UUID
		var latest time.Time
		if err := rows.Scan(&hiveID, &latest); err != nil {
			return nil, fmt.Errorf("postgres: scan latest inspected_at by hive: %w", err)
		}
		out[hiveID] = latest
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: latest inspected_at by hive: %w", err)
	}

	return out, nil
}

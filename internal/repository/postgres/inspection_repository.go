package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sbezhuk/beebase-common/pagination"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// minSearchLength is the minimum number of characters required for the
// search term to be applied. Shorter terms produce noisy results and put
// unnecessary load on the database.
const minSearchLength = 3

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
		INSERT INTO inspections (id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Exec(ctx, q, i.ID, i.HiveID, i.UserID, i.InspectedAt, i.Notes, i.Type, images(i.Images), i.CreatedAt, i.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create inspection: %w", err)
	}

	return nil
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
		SELECT id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at, deleted_at
		FROM inspections
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var i inspection.Inspection

	err := r.db.QueryRow(ctx, q, inspectionID, userID).Scan(
		&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.Type, &i.Images, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, inspection.ErrNotFound
		}
		return nil, fmt.Errorf("postgres: get inspection: %w", err)
	}

	return &i, nil
}

func (r *InspectionRepository) ListByHive(ctx context.Context, userID, hiveID uuid.UUID, p pagination.Params, search *string) ([]*inspection.Inspection, int, error) {
	countQ := `
		SELECT count(*)
		FROM inspections
		WHERE user_id = $1 AND hive_id = $2 AND deleted_at IS NULL
	`
	countArgs := []any{userID, hiveID}

	q := `
		SELECT id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at, deleted_at
		FROM inspections
		WHERE user_id = $1 AND hive_id = $2 AND deleted_at IS NULL
	`
	listArgs := []any{userID, hiveID}

	if search != nil && len(*search) >= minSearchLength {
		pattern := "%" + *search + "%"
		countQ += ` AND notes ILIKE $3`
		countArgs = append(countArgs, pattern)
		q += fmt.Sprintf(` AND notes ILIKE $3`)
		q += fmt.Sprintf(`
		ORDER BY inspected_at ASC, id ASC
		LIMIT $4 OFFSET $5`)
		listArgs = append(listArgs, pattern, p.Limit, p.Offset())
	} else {
		q += `
		ORDER BY inspected_at ASC, id ASC
		LIMIT $3 OFFSET $4`
		listArgs = append(listArgs, p.Limit, p.Offset())
	}

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
		if err := rows.Scan(&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.Type, &i.Images, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt); err != nil {
			return nil, 0, fmt.Errorf("postgres: scan inspection: %w", err)
		}
		inspections = append(inspections, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list inspections: %w", err)
	}

	return inspections, total, nil
}

func (r *InspectionRepository) ListByUser(ctx context.Context, userID uuid.UUID, p pagination.Params, search *string) ([]*inspection.Inspection, int, error) {
	countQ := `
		SELECT count(*)
		FROM inspections
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	countArgs := []any{userID}

	q := `
		SELECT id, hive_id, user_id, inspected_at, notes, type, images, created_at, updated_at, deleted_at
		FROM inspections
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	listArgs := []any{userID}

	if search != nil && len(*search) >= minSearchLength {
		pattern := "%" + *search + "%"
		countQ += ` AND notes ILIKE $2`
		countArgs = append(countArgs, pattern)
		q += fmt.Sprintf(` AND notes ILIKE $2`)
		q += fmt.Sprintf(`
		ORDER BY inspected_at ASC, id ASC
		LIMIT $3 OFFSET $4`)
		listArgs = append(listArgs, pattern, p.Limit, p.Offset())
	} else {
		q += `
		ORDER BY inspected_at ASC, id ASC
		LIMIT $2 OFFSET $3`
		listArgs = append(listArgs, p.Limit, p.Offset())
	}

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
		if err := rows.Scan(&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.Type, &i.Images, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt); err != nil {
			return nil, 0, fmt.Errorf("postgres: scan inspection: %w", err)
		}
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
		SET inspected_at = $1, notes = $2, type = $3, images = $4, updated_at = $5
		WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL
	`

	tag, err := r.db.Exec(ctx, q, i.InspectedAt, i.Notes, i.Type, images(i.Images), i.UpdatedAt, i.ID, i.UserID)
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

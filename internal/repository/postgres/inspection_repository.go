package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

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
		INSERT INTO inspections (id, hive_id, user_id, inspected_at, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Exec(ctx, q, i.ID, i.HiveID, i.UserID, i.InspectedAt, i.Notes, i.CreatedAt, i.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create inspection: %w", err)
	}

	return nil
}

func (r *InspectionRepository) GetByID(ctx context.Context, userID, inspectionID uuid.UUID) (*inspection.Inspection, error) {
	const q = `
		SELECT id, hive_id, user_id, inspected_at, notes, created_at, updated_at, deleted_at
		FROM inspections
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var i inspection.Inspection

	err := r.db.QueryRow(ctx, q, inspectionID, userID).Scan(
		&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, inspection.ErrNotFound
		}
		return nil, fmt.Errorf("postgres: get inspection: %w", err)
	}

	return &i, nil
}

func (r *InspectionRepository) ListByHive(ctx context.Context, userID, hiveID uuid.UUID) ([]*inspection.Inspection, error) {
	const q = `
		SELECT id, hive_id, user_id, inspected_at, notes, created_at, updated_at, deleted_at
		FROM inspections
		WHERE user_id = $1 AND hive_id = $2 AND deleted_at IS NULL
		ORDER BY inspected_at ASC
	`

	rows, err := r.db.Query(ctx, q, userID, hiveID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list inspections: %w", err)
	}
	defer rows.Close()

	inspections := []*inspection.Inspection{}
	for rows.Next() {
		var i inspection.Inspection
		if err := rows.Scan(&i.ID, &i.HiveID, &i.UserID, &i.InspectedAt, &i.Notes, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan inspection: %w", err)
		}
		inspections = append(inspections, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list inspections: %w", err)
	}

	return inspections, nil
}

func (r *InspectionRepository) Update(ctx context.Context, i *inspection.Inspection) error {
	const q = `
		UPDATE inspections
		SET inspected_at = $1, notes = $2, updated_at = $3
		WHERE id = $4 AND user_id = $5 AND deleted_at IS NULL
	`

	tag, err := r.db.Exec(ctx, q, i.InspectedAt, i.Notes, i.UpdatedAt, i.ID, i.UserID)
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

package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"

	"mad/pkg/contracts"
)

type AssignedTraining struct {
	ID              string   `json:"id"`
	OperatorID      string   `json:"operatorId"`
	ModuleID        string   `json:"moduleId"`
	Title           string   `json:"title"`
	DurationMinutes int      `json:"durationMinutes"`
	TriggerTag      string   `json:"triggerTag"`
	Status          string   `json:"status"`
	Score           *float64 `json:"score"`
}

type TrainingAssignedEvent struct {
	OperatorID string `json:"operatorId"`
	ModuleID   string `json:"moduleId"`
	Title      string `json:"title"`
	TriggerTag string `json:"triggerTag"`
}

type Repository interface {
	AssignModulesForAnomaly(ctx context.Context, operatorID string, anomalyType string) ([]TrainingAssignedEvent, error)
	ListOperatorTraining(ctx context.Context, operatorID string) ([]AssignedTraining, error)
	CompleteTraining(ctx context.Context, id string, score float64) (*AssignedTraining, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) AssignModulesForAnomaly(ctx context.Context, operatorID string, anomalyType string) ([]TrainingAssignedEvent, error) {
	const selectModules = `
		SELECT id, title, trigger_tag
		FROM training_modules
		WHERE trigger_tag = $1`

	rows, err := r.db.QueryContext(ctx, selectModules, anomalyType)
	if err != nil {
		return nil, fmt.Errorf("select training modules: %w", err)
	}
	defer rows.Close()

	events := make([]TrainingAssignedEvent, 0)
	for rows.Next() {
		var module contracts.TrainingModule
		if err := rows.Scan(&module.ID, &module.Title, &module.TriggerTag); err != nil {
			return nil, fmt.Errorf("scan training module: %w", err)
		}

		assignmentID, err := NewID()
		if err != nil {
			return nil, fmt.Errorf("generate operator training id: %w", err)
		}

		const insertAssignment = `
			INSERT INTO operator_training (id, operator_id, module_id, status, score)
			VALUES ($1, $2, $3, $4, $5)`
		if _, err := r.db.ExecContext(ctx, insertAssignment, assignmentID, operatorID, module.ID, "ASSIGNED", nil); err != nil {
			return nil, fmt.Errorf("insert operator training: %w", err)
		}

		events = append(events, TrainingAssignedEvent{
			OperatorID: operatorID,
			ModuleID:   module.ID,
			Title:      module.Title,
			TriggerTag: module.TriggerTag,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate training modules: %w", err)
	}

	return events, nil
}

func (r *PostgresRepository) ListOperatorTraining(ctx context.Context, operatorID string) ([]AssignedTraining, error) {
	const query = `
		SELECT ot.id, ot.operator_id, ot.module_id, tm.title, tm.duration_minutes, tm.trigger_tag, ot.status, ot.score
		FROM operator_training ot
		INNER JOIN training_modules tm ON tm.id = ot.module_id
		WHERE ot.operator_id = $1 AND ot.status IN ('ASSIGNED', 'COMPLETED')
		ORDER BY tm.title`

	rows, err := r.db.QueryContext(ctx, query, operatorID)
	if err != nil {
		return nil, fmt.Errorf("list operator training: %w", err)
	}
	defer rows.Close()

	trainings := make([]AssignedTraining, 0)
	for rows.Next() {
		training, err := scanAssignedTraining(rows)
		if err != nil {
			return nil, err
		}
		trainings = append(trainings, training)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operator training: %w", err)
	}

	return trainings, nil
}

func (r *PostgresRepository) CompleteTraining(ctx context.Context, id string, score float64) (*AssignedTraining, error) {
	const query = `
		UPDATE operator_training
		SET status = 'COMPLETED', score = $1
		WHERE id = $2
		RETURNING id, operator_id, module_id, status, score`

	var completed contracts.OperatorTraining
	if err := r.db.QueryRowContext(ctx, query, score, id).Scan(
		&completed.ID,
		&completed.OperatorID,
		&completed.ModuleID,
		&completed.Status,
		&completed.Score,
	); err != nil {
		return nil, fmt.Errorf("complete operator training: %w", err)
	}

	const moduleQuery = `
		SELECT title, duration_minutes, trigger_tag
		FROM training_modules
		WHERE id = $1`
	var module contracts.TrainingModule
	if err := r.db.QueryRowContext(ctx, moduleQuery, completed.ModuleID).Scan(
		&module.Title,
		&module.DurationMinutes,
		&module.TriggerTag,
	); err != nil {
		return nil, fmt.Errorf("get completed training module: %w", err)
	}

	return &AssignedTraining{
		ID:              completed.ID,
		OperatorID:      completed.OperatorID,
		ModuleID:        completed.ModuleID,
		Title:           module.Title,
		DurationMinutes: module.DurationMinutes,
		TriggerTag:      module.TriggerTag,
		Status:          completed.Status,
		Score:           &completed.Score,
	}, nil
}

type trainingScanner interface {
	Scan(dest ...interface{}) error
}

func scanAssignedTraining(scanner trainingScanner) (AssignedTraining, error) {
	var training AssignedTraining
	var score sql.NullFloat64
	if err := scanner.Scan(
		&training.ID,
		&training.OperatorID,
		&training.ModuleID,
		&training.Title,
		&training.DurationMinutes,
		&training.TriggerTag,
		&training.Status,
		&score,
	); err != nil {
		return AssignedTraining{}, fmt.Errorf("scan operator training: %w", err)
	}
	if score.Valid {
		training.Score = &score.Float64
	}
	return training, nil
}

func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

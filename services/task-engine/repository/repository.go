package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"mad/pkg/contracts"
)

type TaskProgress struct {
	Status           string
	TargetVolume     float64
	EstimatedMinutes int
}

type TaskRepository interface {
	CreateTask(ctx context.Context, task *contracts.Task) error
	GetTaskByID(ctx context.Context, id string) (*contracts.Task, error)
	UpdateTaskProgress(ctx context.Context, id string, progress TaskProgress) (*contracts.Task, error)
}

type MachineRepository interface {
	AssignOperator(ctx context.Context, machineID string, operatorID string) error
}

type Store interface {
	TaskRepository
	MachineRepository
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreateTask(ctx context.Context, task *contracts.Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create task transaction: %w", err)
	}
	committed := false
	defer rollbackUnlessCommitted(tx, &committed)

	const insertTask = `
		INSERT INTO tasks (id, category, machine_id, operator_id, status, target_volume, estimated_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = tx.ExecContext(
		ctx,
		insertTask,
		task.ID,
		task.Category,
		task.MachineID,
		task.OperatorID,
		task.Status,
		task.TargetVolume,
		task.EstimatedMinutes,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	const assignMachine = `
		UPDATE machines
		SET current_operator_id = $1, status = 'ASSIGNED'
		WHERE id = $2`
	result, err := tx.ExecContext(ctx, assignMachine, task.OperatorID, task.MachineID)
	if err != nil {
		return fmt.Errorf("assign machine: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read assigned machine count: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("machine %s not found", task.MachineID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create task transaction: %w", err)
	}
	committed = true

	return nil
}

func (s *PostgresStore) GetTaskByID(ctx context.Context, id string) (*contracts.Task, error) {
	const query = `
		SELECT id, category, machine_id, operator_id, status, target_volume, estimated_minutes
		FROM tasks
		WHERE id = $1`

	var task contracts.Task
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.Category,
		&task.MachineID,
		&task.OperatorID,
		&task.Status,
		&task.TargetVolume,
		&task.EstimatedMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("get task by id: %w", err)
	}

	return &task, nil
}

func (s *PostgresStore) UpdateTaskProgress(ctx context.Context, id string, progress TaskProgress) (*contracts.Task, error) {
	const query = `
		UPDATE tasks
		SET status = $1, target_volume = $2, estimated_minutes = $3
		WHERE id = $4
		RETURNING id, category, machine_id, operator_id, status, target_volume, estimated_minutes`

	var task contracts.Task
	err := s.db.QueryRowContext(
		ctx,
		query,
		progress.Status,
		progress.TargetVolume,
		progress.EstimatedMinutes,
		id,
	).Scan(
		&task.ID,
		&task.Category,
		&task.MachineID,
		&task.OperatorID,
		&task.Status,
		&task.TargetVolume,
		&task.EstimatedMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("update task progress: %w", err)
	}

	return &task, nil
}

func (s *PostgresStore) AssignOperator(ctx context.Context, machineID string, operatorID string) error {
	const query = `
		UPDATE machines
		SET current_operator_id = $1, status = 'ASSIGNED'
		WHERE id = $2`

	result, err := s.db.ExecContext(ctx, query, operatorID, machineID)
	if err != nil {
		return fmt.Errorf("assign operator: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read assigned operator count: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("machine %s not found", machineID)
	}

	return nil
}

func rollbackUnlessCommitted(tx *sql.Tx, committed *bool) {
	if tx == nil {
		return
	}
	if committed != nil && *committed {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Printf("rollback create task transaction: %v", err)
	}
}

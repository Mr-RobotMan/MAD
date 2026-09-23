package notification

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type queryPool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type postgresRepository struct{ pool queryPool }

func NewPostgresRepository(pool queryPool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) CreateAlert(ctx context.Context, anomaly Anomaly) (Alert, error) {
	var alert Alert
	err := r.pool.QueryRow(ctx, `INSERT INTO alerts
		(operator_id, machine_id, type, severity, timestamp)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, operator_id::text, machine_id::text, type, severity, timestamp`,
		anomaly.OperatorID, anomaly.MachineID, anomaly.Type, anomaly.Severity, anomaly.Timestamp,
	).Scan(&alert.ID, &alert.OperatorID, &alert.MachineID, &alert.Type, &alert.Severity, &alert.Timestamp)
	if err != nil {
		return Alert{}, fmt.Errorf("insert alert: %w", err)
	}
	return alert, nil
}

func (r *postgresRepository) CreateEscalation(ctx context.Context, alert Alert) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO escalations (alert_id) VALUES ($1)`, alert.ID)
	if err != nil {
		return fmt.Errorf("insert escalation: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListAlerts(ctx context.Context, operatorID, severity string) ([]Alert, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, operator_id::text, machine_id::text,
		type, severity, timestamp FROM alerts
		WHERE ($1 = '' OR operator_id::text = $1) AND ($2 = '' OR severity = $2)
		ORDER BY timestamp DESC`, operatorID, severity)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()
	alerts := make([]Alert, 0)
	for rows.Next() {
		var alert Alert
		if err := rows.Scan(&alert.ID, &alert.OperatorID, &alert.MachineID, &alert.Type, &alert.Severity, &alert.Timestamp); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerts: %w", err)
	}
	return alerts, nil
}

func (r *postgresRepository) ListEscalations(ctx context.Context) ([]Escalation, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.id::text, a.id::text, a.operator_id::text,
		a.machine_id::text, a.type, a.severity, a.timestamp
		FROM escalations e JOIN alerts a ON a.id = e.alert_id ORDER BY a.timestamp DESC`)
	if err != nil {
		return nil, fmt.Errorf("query escalations: %w", err)
	}
	defer rows.Close()
	escalations := make([]Escalation, 0)
	for rows.Next() {
		var escalation Escalation
		if err := rows.Scan(&escalation.ID, &escalation.AlertID, &escalation.OperatorID,
			&escalation.MachineID, &escalation.Type, &escalation.Severity, &escalation.Timestamp); err != nil {
			return nil, fmt.Errorf("scan escalation: %w", err)
		}
		escalations = append(escalations, escalation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate escalations: %w", err)
	}
	return escalations, nil
}

func (r *postgresRepository) UserRole(ctx context.Context, userID string) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx, `SELECT role FROM users WHERE id::text = $1`, userID).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("lookup user role: %w", err)
	}
	return role, nil
}

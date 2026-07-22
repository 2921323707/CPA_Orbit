package controlplane

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

type GatewayTarget struct {
	ID                 int64     `json:"id"`
	Kind               string    `json:"kind"`
	Name               string    `json:"name"`
	BaseURL            string    `json:"baseUrl"`
	AdminKey           string    `json:"-"`
	AdminKeyConfigured bool      `json:"adminKeyConfigured"`
	Enabled            bool      `json:"enabled"`
	Primary            bool      `json:"primary"`
	AllowRemote        bool      `json:"allowRemote"`
	DefaultGroupIDs    []int64   `json:"defaultGroupIds,omitempty"`
	DefaultConcurrency int       `json:"defaultConcurrency"`
	DefaultPriority    int       `json:"defaultPriority"`
	RateMultiplier     float64   `json:"rateMultiplier"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type DeploymentBinding struct {
	ID                    int64      `json:"id"`
	SubscriptionID        string     `json:"subscriptionId"`
	TargetID              int64      `json:"targetId"`
	RemoteAccountID       string     `json:"remoteAccountId,omitempty"`
	Mode                  string     `json:"mode"`
	Ownership             string     `json:"ownership"`
	DesiredState          string     `json:"desiredState"`
	ObservedState         string     `json:"observedState"`
	CredentialFingerprint string     `json:"credentialFingerprint,omitempty"`
	LastError             string     `json:"lastError,omitempty"`
	LastSyncedAt          *time.Time `json:"lastSyncedAt,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type SyncOperation struct {
	ID             int64      `json:"id"`
	SubscriptionID string     `json:"subscriptionId"`
	TargetID       int64      `json:"targetId"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Attempt        int        `json:"attempt"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

type UsageBucket struct {
	ID                  int64     `json:"id"`
	TargetID            int64     `json:"targetId"`
	BucketAt            time.Time `json:"bucketAt"`
	BucketMinutes       int       `json:"bucketMinutes"`
	AccountID           string    `json:"accountId,omitempty"`
	GroupName           string    `json:"groupName,omitempty"`
	Model               string    `json:"model,omitempty"`
	Requests            int64     `json:"requests"`
	Successes           int64     `json:"successes"`
	Failures            int64     `json:"failures"`
	InputTokens         int64     `json:"inputTokens"`
	OutputTokens        int64     `json:"outputTokens"`
	CacheCreationTokens int64     `json:"cacheCreationTokens"`
	CacheReadTokens     int64     `json:"cacheReadTokens"`
	Cost                float64   `json:"cost"`
	ActualCost          float64   `json:"actualCost"`
	AverageDurationMS   float64   `json:"averageDurationMs"`
	FirstTokenMS        float64   `json:"firstTokenMs"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("control-plane database path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve control-plane database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return nil, fmt.Errorf("create control-plane database directory: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(abs) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open control-plane database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect control-plane database: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin control-plane migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS gateway_targets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL DEFAULT '',
			admin_key TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			is_primary INTEGER NOT NULL DEFAULT 0,
			allow_remote INTEGER NOT NULL DEFAULT 0,
			default_group_ids TEXT NOT NULL DEFAULT '[]',
			default_concurrency INTEGER NOT NULL DEFAULT 1,
			default_priority INTEGER NOT NULL DEFAULT 0,
			rate_multiplier REAL NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(kind, name)
		)`,
		`CREATE TABLE IF NOT EXISTS deployment_bindings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			subscription_id TEXT NOT NULL,
			target_id INTEGER NOT NULL REFERENCES gateway_targets(id) ON DELETE CASCADE,
			remote_account_id TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL DEFAULT 'primary',
			ownership TEXT NOT NULL DEFAULT 'managed',
			desired_state TEXT NOT NULL DEFAULT 'active',
			observed_state TEXT NOT NULL DEFAULT 'unknown',
			credential_fingerprint TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			last_synced_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(subscription_id, target_id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_operations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			subscription_id TEXT NOT NULL,
			target_id INTEGER NOT NULL REFERENCES gateway_targets(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			attempt INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS sync_operations_status_idx ON sync_operations(status, created_at)`,
		`CREATE TABLE IF NOT EXISTS usage_buckets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			target_id INTEGER NOT NULL REFERENCES gateway_targets(id) ON DELETE CASCADE,
			bucket_at TEXT NOT NULL,
			bucket_minutes INTEGER NOT NULL,
			account_id TEXT NOT NULL DEFAULT '',
			group_name TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			requests INTEGER NOT NULL DEFAULT 0,
			successes INTEGER NOT NULL DEFAULT 0,
			failures INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cost REAL NOT NULL DEFAULT 0,
			actual_cost REAL NOT NULL DEFAULT 0,
			average_duration_ms REAL NOT NULL DEFAULT 0,
			first_token_ms REAL NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			UNIQUE(target_id, bucket_at, bucket_minutes, account_id, group_name, model)
		)`,
		`CREATE INDEX IF NOT EXISTS usage_buckets_time_idx ON usage_buckets(target_id, bucket_at)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate control-plane database: %w", err)
		}
	}
	columns := []struct{ name, ddl string }{
		{"default_group_ids", `ALTER TABLE gateway_targets ADD COLUMN default_group_ids TEXT NOT NULL DEFAULT '[]'`},
		{"default_concurrency", `ALTER TABLE gateway_targets ADD COLUMN default_concurrency INTEGER NOT NULL DEFAULT 1`},
		{"default_priority", `ALTER TABLE gateway_targets ADD COLUMN default_priority INTEGER NOT NULL DEFAULT 0`},
		{"rate_multiplier", `ALTER TABLE gateway_targets ADD COLUMN rate_multiplier REAL NOT NULL DEFAULT 1`},
	}
	for _, column := range columns {
		if err := ensureGatewayTargetColumn(tx, column.name, column.ddl); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)); err != nil {
		return fmt.Errorf("set control-plane schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit control-plane migration: %w", err)
	}
	return nil
}

func ensureGatewayTargetColumn(tx *sql.Tx, name, ddl string) error {
	rows, err := tx.Query(`PRAGMA table_info(gateway_targets)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var columnName, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if columnName == name {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err := tx.Exec(ddl); err != nil {
		return fmt.Errorf("add gateway target column %s: %w", name, err)
	}
	return nil
}

func (s *Store) UpsertGatewayTarget(ctx context.Context, target GatewayTarget) (GatewayTarget, error) {
	now := time.Now().UTC()
	if strings.TrimSpace(target.Kind) == "" || strings.TrimSpace(target.Name) == "" {
		return GatewayTarget{}, errors.New("gateway target kind and name are required")
	}
	if target.DefaultConcurrency < 1 {
		target.DefaultConcurrency = 1
	}
	if target.RateMultiplier <= 0 {
		target.RateMultiplier = 1
	}
	groupIDs, err := json.Marshal(target.DefaultGroupIDs)
	if err != nil {
		return GatewayTarget{}, fmt.Errorf("encode default group IDs: %w", err)
	}
	if target.ID == 0 {
		result, err := s.db.ExecContext(ctx, `INSERT INTO gateway_targets
			(kind, name, base_url, admin_key, enabled, is_primary, allow_remote, default_group_ids, default_concurrency, default_priority, rate_multiplier, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(kind, name) DO UPDATE SET
			base_url=excluded.base_url,
			admin_key=CASE WHEN excluded.admin_key='' THEN gateway_targets.admin_key ELSE excluded.admin_key END,
			enabled=excluded.enabled,
			is_primary=excluded.is_primary,
			allow_remote=excluded.allow_remote,
			default_group_ids=excluded.default_group_ids,
			default_concurrency=excluded.default_concurrency,
			default_priority=excluded.default_priority,
			rate_multiplier=excluded.rate_multiplier,
			updated_at=excluded.updated_at`,
			strings.TrimSpace(target.Kind), strings.TrimSpace(target.Name), strings.TrimRight(strings.TrimSpace(target.BaseURL), "/"), target.AdminKey,
			boolInt(target.Enabled), boolInt(target.Primary), boolInt(target.AllowRemote), string(groupIDs), target.DefaultConcurrency, target.DefaultPriority, target.RateMultiplier, formatTime(now), formatTime(now))
		if err != nil {
			return GatewayTarget{}, fmt.Errorf("upsert gateway target: %w", err)
		}
		id, _ := result.LastInsertId()
		if id == 0 {
			err = s.db.QueryRowContext(ctx, `SELECT id FROM gateway_targets WHERE kind=? AND name=?`, strings.TrimSpace(target.Kind), strings.TrimSpace(target.Name)).Scan(&id)
			if err != nil {
				return GatewayTarget{}, fmt.Errorf("resolve gateway target: %w", err)
			}
		}
		target.ID = id
	} else {
		result, err := s.db.ExecContext(ctx, `UPDATE gateway_targets SET
			kind=?, name=?, base_url=?, admin_key=CASE WHEN ?='' THEN admin_key ELSE ? END,
			enabled=?, is_primary=?, allow_remote=?, default_group_ids=?, default_concurrency=?, default_priority=?, rate_multiplier=?, updated_at=? WHERE id=?`,
			strings.TrimSpace(target.Kind), strings.TrimSpace(target.Name), strings.TrimRight(strings.TrimSpace(target.BaseURL), "/"), target.AdminKey, target.AdminKey,
			boolInt(target.Enabled), boolInt(target.Primary), boolInt(target.AllowRemote), string(groupIDs), target.DefaultConcurrency, target.DefaultPriority, target.RateMultiplier, formatTime(now), target.ID)
		if err != nil {
			return GatewayTarget{}, fmt.Errorf("update gateway target: %w", err)
		}
		if count, _ := result.RowsAffected(); count == 0 {
			return GatewayTarget{}, sql.ErrNoRows
		}
	}
	if target.Primary {
		if _, err := s.db.ExecContext(ctx, `UPDATE gateway_targets SET is_primary=0, updated_at=? WHERE id<>? AND is_primary=1`, formatTime(now), target.ID); err != nil {
			return GatewayTarget{}, fmt.Errorf("enforce primary gateway target: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE deployment_bindings SET mode=CASE WHEN target_id=? THEN 'primary' ELSE 'fallback' END, updated_at=? WHERE mode IN ('primary','fallback')`, target.ID, formatTime(now)); err != nil {
			return GatewayTarget{}, fmt.Errorf("reclassify gateway bindings: %w", err)
		}
	} else if _, err := s.db.ExecContext(ctx, `UPDATE deployment_bindings SET mode='fallback', updated_at=? WHERE target_id=? AND mode='primary'`, formatTime(now), target.ID); err != nil {
		return GatewayTarget{}, fmt.Errorf("reclassify disabled primary bindings: %w", err)
	}
	return s.gatewayTarget(ctx, target.ID, false)
}

func (s *Store) ListGatewayTargets(ctx context.Context) ([]GatewayTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, name, base_url, admin_key<>'', enabled, is_primary, allow_remote, default_group_ids, default_concurrency, default_priority, rate_multiplier, created_at, updated_at FROM gateway_targets ORDER BY is_primary DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("list gateway targets: %w", err)
	}
	defer rows.Close()
	var targets []GatewayTarget
	for rows.Next() {
		var target GatewayTarget
		var enabled, primary, allowRemote int
		var groupIDs string
		var createdAt, updatedAt string
		if err := rows.Scan(&target.ID, &target.Kind, &target.Name, &target.BaseURL, &target.AdminKeyConfigured, &enabled, &primary, &allowRemote, &groupIDs, &target.DefaultConcurrency, &target.DefaultPriority, &target.RateMultiplier, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(groupIDs), &target.DefaultGroupIDs)
		target.Enabled, target.Primary, target.AllowRemote = enabled != 0, primary != 0, allowRemote != 0
		target.CreatedAt, target.UpdatedAt = mustTime(createdAt), mustTime(updatedAt)
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *Store) GatewayTarget(ctx context.Context, id int64) (GatewayTarget, error) {
	return s.gatewayTarget(ctx, id, false)
}

func (s *Store) GatewayTargetSecret(ctx context.Context, id int64) (string, error) {
	var secret string
	if err := s.db.QueryRowContext(ctx, `SELECT admin_key FROM gateway_targets WHERE id=?`, id).Scan(&secret); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *Store) gatewayTarget(ctx context.Context, id int64, includeSecret bool) (GatewayTarget, error) {
	var target GatewayTarget
	var secret string
	var enabled, primary, allowRemote int
	var groupIDs string
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, kind, name, base_url, admin_key, enabled, is_primary, allow_remote, default_group_ids, default_concurrency, default_priority, rate_multiplier, created_at, updated_at FROM gateway_targets WHERE id=?`, id).
		Scan(&target.ID, &target.Kind, &target.Name, &target.BaseURL, &secret, &enabled, &primary, &allowRemote, &groupIDs, &target.DefaultConcurrency, &target.DefaultPriority, &target.RateMultiplier, &createdAt, &updatedAt)
	if err != nil {
		return GatewayTarget{}, err
	}
	target.AdminKeyConfigured = secret != ""
	_ = json.Unmarshal([]byte(groupIDs), &target.DefaultGroupIDs)
	if includeSecret {
		target.AdminKey = secret
	}
	target.Enabled, target.Primary, target.AllowRemote = enabled != 0, primary != 0, allowRemote != 0
	target.CreatedAt, target.UpdatedAt = mustTime(createdAt), mustTime(updatedAt)
	return target, nil
}

func (s *Store) UpsertDeploymentBinding(ctx context.Context, binding DeploymentBinding) (DeploymentBinding, error) {
	now := time.Now().UTC()
	if binding.SubscriptionID == "" || binding.TargetID == 0 {
		return DeploymentBinding{}, errors.New("subscription ID and target ID are required")
	}
	lastSyncedAt := nullableTime(binding.LastSyncedAt)
	_, err := s.db.ExecContext(ctx, `INSERT INTO deployment_bindings
		(subscription_id, target_id, remote_account_id, mode, ownership, desired_state, observed_state, credential_fingerprint, last_error, last_synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(subscription_id, target_id) DO UPDATE SET
		remote_account_id=excluded.remote_account_id, mode=excluded.mode, ownership=excluded.ownership,
		desired_state=excluded.desired_state, observed_state=excluded.observed_state,
		credential_fingerprint=excluded.credential_fingerprint, last_error=excluded.last_error,
		last_synced_at=excluded.last_synced_at, updated_at=excluded.updated_at`,
		binding.SubscriptionID, binding.TargetID, binding.RemoteAccountID, defaultString(binding.Mode, "primary"), defaultString(binding.Ownership, "managed"),
		defaultString(binding.DesiredState, "active"), defaultString(binding.ObservedState, "unknown"), binding.CredentialFingerprint,
		binding.LastError, lastSyncedAt, formatTime(now), formatTime(now))
	if err != nil {
		return DeploymentBinding{}, fmt.Errorf("upsert deployment binding: %w", err)
	}
	return s.deploymentBinding(ctx, binding.SubscriptionID, binding.TargetID)
}

func (s *Store) ListDeploymentBindings(ctx context.Context, subscriptionID string) ([]DeploymentBinding, error) {
	query := `SELECT id, subscription_id, target_id, remote_account_id, mode, ownership, desired_state, observed_state, credential_fingerprint, last_error, last_synced_at, created_at, updated_at FROM deployment_bindings`
	args := []any{}
	if subscriptionID != "" {
		query += ` WHERE subscription_id=?`
		args = append(args, subscriptionID)
	}
	query += ` ORDER BY id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list deployment bindings: %w", err)
	}
	defer rows.Close()
	var bindings []DeploymentBinding
	for rows.Next() {
		binding, err := scanDeploymentBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (s *Store) DeploymentBinding(ctx context.Context, subscriptionID string, targetID int64) (DeploymentBinding, error) {
	return s.deploymentBinding(ctx, subscriptionID, targetID)
}

func (s *Store) deploymentBinding(ctx context.Context, subscriptionID string, targetID int64) (DeploymentBinding, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, subscription_id, target_id, remote_account_id, mode, ownership, desired_state, observed_state, credential_fingerprint, last_error, last_synced_at, created_at, updated_at FROM deployment_bindings WHERE subscription_id=? AND target_id=?`, subscriptionID, targetID)
	return scanDeploymentBinding(row)
}

type rowScanner interface{ Scan(...any) error }

func scanDeploymentBinding(row rowScanner) (DeploymentBinding, error) {
	var binding DeploymentBinding
	var lastSyncedAt sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(&binding.ID, &binding.SubscriptionID, &binding.TargetID, &binding.RemoteAccountID, &binding.Mode, &binding.Ownership, &binding.DesiredState, &binding.ObservedState, &binding.CredentialFingerprint, &binding.LastError, &lastSyncedAt, &createdAt, &updatedAt); err != nil {
		return DeploymentBinding{}, err
	}
	if lastSyncedAt.Valid {
		v := mustTime(lastSyncedAt.String)
		binding.LastSyncedAt = &v
	}
	binding.CreatedAt, binding.UpdatedAt = mustTime(createdAt), mustTime(updatedAt)
	return binding, nil
}

func (s *Store) CreateSyncOperation(ctx context.Context, operation SyncOperation) (SyncOperation, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `INSERT INTO sync_operations (subscription_id, target_id, kind, status, attempt, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		operation.SubscriptionID, operation.TargetID, operation.Kind, defaultString(operation.Status, "pending"), operation.Attempt, operation.LastError, formatTime(now), formatTime(now))
	if err != nil {
		return SyncOperation{}, fmt.Errorf("create sync operation: %w", err)
	}
	operation.ID, _ = result.LastInsertId()
	operation.CreatedAt, operation.UpdatedAt = now, now
	operation.Status = defaultString(operation.Status, "pending")
	return operation, nil
}

func (s *Store) CompleteSyncOperation(ctx context.Context, id int64, status, lastError string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE sync_operations SET status=?, last_error=?, updated_at=?, completed_at=? WHERE id=?`, status, lastError, formatTime(now), formatTime(now), id)
	if err != nil {
		return fmt.Errorf("complete sync operation: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListSyncOperations(ctx context.Context, limit int) ([]SyncOperation, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, subscription_id, target_id, kind, status, attempt, last_error, created_at, updated_at, completed_at FROM sync_operations ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var operations []SyncOperation
	for rows.Next() {
		var operation SyncOperation
		var createdAt, updatedAt string
		var completedAt sql.NullString
		if err := rows.Scan(&operation.ID, &operation.SubscriptionID, &operation.TargetID, &operation.Kind, &operation.Status, &operation.Attempt, &operation.LastError, &createdAt, &updatedAt, &completedAt); err != nil {
			return nil, err
		}
		operation.CreatedAt, operation.UpdatedAt = mustTime(createdAt), mustTime(updatedAt)
		if completedAt.Valid {
			v := mustTime(completedAt.String)
			operation.CompletedAt = &v
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}

func (s *Store) UpsertUsageBucket(ctx context.Context, bucket UsageBucket) error {
	if bucket.TargetID == 0 || bucket.BucketAt.IsZero() || bucket.BucketMinutes < 1 {
		return errors.New("target, bucket time, and bucket size are required")
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO usage_buckets
		(target_id, bucket_at, bucket_minutes, account_id, group_name, model, requests, successes, failures, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost, actual_cost, average_duration_ms, first_token_ms, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_id, bucket_at, bucket_minutes, account_id, group_name, model) DO UPDATE SET
		requests=excluded.requests, successes=excluded.successes, failures=excluded.failures,
		input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
		cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
		cost=excluded.cost, actual_cost=excluded.actual_cost, average_duration_ms=excluded.average_duration_ms,
		first_token_ms=excluded.first_token_ms, updated_at=excluded.updated_at`,
		bucket.TargetID, formatTime(bucket.BucketAt.UTC()), bucket.BucketMinutes, bucket.AccountID, bucket.GroupName, bucket.Model,
		bucket.Requests, bucket.Successes, bucket.Failures, bucket.InputTokens, bucket.OutputTokens, bucket.CacheCreationTokens,
		bucket.CacheReadTokens, bucket.Cost, bucket.ActualCost, bucket.AverageDurationMS, bucket.FirstTokenMS, formatTime(now))
	if err != nil {
		return fmt.Errorf("upsert usage bucket: %w", err)
	}
	return nil
}

func (s *Store) ListUsageBuckets(ctx context.Context, targetID int64, from, to time.Time) ([]UsageBucket, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, target_id, bucket_at, bucket_minutes, account_id, group_name, model, requests, successes, failures, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost, actual_cost, average_duration_ms, first_token_ms, updated_at FROM usage_buckets WHERE target_id=? AND bucket_at>=? AND bucket_at<=? ORDER BY bucket_at, account_id, model`,
		targetID, formatTime(from.UTC()), formatTime(to.UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var buckets []UsageBucket
	for rows.Next() {
		var bucket UsageBucket
		var bucketAt, updatedAt string
		if err := rows.Scan(&bucket.ID, &bucket.TargetID, &bucketAt, &bucket.BucketMinutes, &bucket.AccountID, &bucket.GroupName, &bucket.Model,
			&bucket.Requests, &bucket.Successes, &bucket.Failures, &bucket.InputTokens, &bucket.OutputTokens, &bucket.CacheCreationTokens,
			&bucket.CacheReadTokens, &bucket.Cost, &bucket.ActualCost, &bucket.AverageDurationMS, &bucket.FirstTokenMS, &updatedAt); err != nil {
			return nil, err
		}
		bucket.BucketAt, bucket.UpdatedAt = mustTime(bucketAt), mustTime(updatedAt)
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (s *Store) DeleteUsageBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM usage_buckets WHERE bucket_at<?`, formatTime(cutoff.UTC()))
	if err != nil {
		return 0, fmt.Errorf("delete expired usage buckets: %w", err)
	}
	count, _ := result.RowsAffected()
	return count, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func mustTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return formatTime(*value)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

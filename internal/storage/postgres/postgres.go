package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/meld/pr-allocation-service/internal/domain"
	"github.com/meld/pr-allocation-service/pkg/logger"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(host, port, user, password, dbname, sslmode string) (*PostgresStorage, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

// User operations
func (s *PostgresStorage) CreateUser(ctx context.Context, user *domain.User) error {
	log := logger.FromContext(ctx)
	query := `INSERT INTO users (user_id, username, team_name, is_active, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6)
              ON CONFLICT (user_id) DO UPDATE 
              SET username = $2, team_name = $3, is_active = $4, updated_at = $6`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, query, user.UserID, user.Username, user.TeamName, user.IsActive, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		log.Error(ctx, "failed to create/update user", zap.Error(err), zap.String("user_id", user.UserID))
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Info(ctx, "user created/updated", zap.String("user_id", user.UserID), zap.String("username", user.Username))
	return nil
}

func (s *PostgresStorage) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	log := logger.FromContext(ctx)
	query := `SELECT user_id, username, team_name, is_active, created_at, updated_at FROM users WHERE user_id = $1`

	user := &domain.User{}
	var teamName sql.NullString
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.UserID, &user.Username, &teamName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)

	if teamName.Valid {
		user.TeamName = teamName.String
	}

	if err == sql.ErrNoRows {
		log.Debug(ctx, "user not found", zap.String("user_id", userID))
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		log.Error(ctx, "failed to get user", zap.Error(err), zap.String("user_id", userID))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *PostgresStorage) UpdateUser(ctx context.Context, user *domain.User) error {
	log := logger.FromContext(ctx)
	query := `UPDATE users SET username = $1, team_name = $2, is_active = $3, updated_at = $4 WHERE user_id = $5`

	user.UpdatedAt = time.Now()

	result, err := s.db.ExecContext(ctx, query, user.Username, user.TeamName, user.IsActive, user.UpdatedAt, user.UserID)
	if err != nil {
		log.Error(ctx, "failed to update user", zap.Error(err), zap.String("user_id", user.UserID))
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	log.Info(ctx, "user updated", zap.String("user_id", user.UserID))
	return nil
}

func (s *PostgresStorage) GetUsersByTeam(ctx context.Context, teamName string) ([]*domain.User, error) {
	log := logger.FromContext(ctx)
	query := `SELECT user_id, username, team_name, is_active, created_at, updated_at FROM users WHERE team_name = $1`

	rows, err := s.db.QueryContext(ctx, query, teamName)
	if err != nil {
		log.Error(ctx, "failed to get users by team", zap.Error(err), zap.String("team_name", teamName))
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		var team sql.NullString
		if err := rows.Scan(&user.UserID, &user.Username, &team, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			log.Error(ctx, "failed to scan user", zap.Error(err))
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if team.Valid {
			user.TeamName = team.String
		}
		users = append(users, user)
	}

	return users, nil
}

// Team operations
func (s *PostgresStorage) CreateTeam(ctx context.Context, team *domain.Team) error {
	log := logger.FromContext(ctx)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert team
	query := `INSERT INTO teams (team_name, created_at, updated_at) VALUES ($1, $2, $3)`
	now := time.Now()
	team.CreatedAt = now
	team.UpdatedAt = now

	_, err = tx.ExecContext(ctx, query, team.TeamName, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		log.Error(ctx, "failed to create team", zap.Error(err), zap.String("team_name", team.TeamName))
		return fmt.Errorf("failed to create team: %w", err)
	}

	// Insert/update members
	for _, member := range team.Members {
		user := &domain.User{
			UserID:   member.UserID,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		}

		userQuery := `INSERT INTO users (user_id, username, team_name, is_active, created_at, updated_at)
                      VALUES ($1, $2, $3, $4, $5, $6)
                      ON CONFLICT (user_id) DO UPDATE 
                      SET username = $2, team_name = $3, is_active = $4, updated_at = $6`

		_, err = tx.ExecContext(ctx, userQuery, user.UserID, user.Username, user.TeamName, user.IsActive, now, now)
		if err != nil {
			log.Error(ctx, "failed to create/update user", zap.Error(err), zap.String("user_id", user.UserID))
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info(ctx, "team created", zap.String("team_name", team.TeamName), zap.Int("members", len(team.Members)))
	return nil
}

func (s *PostgresStorage) GetTeam(ctx context.Context, teamName string) (*domain.Team, error) {
	log := logger.FromContext(ctx)

	// Check if team exists
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)`, teamName).Scan(&exists)
	if err != nil {
		log.Error(ctx, "failed to check team existence", zap.Error(err), zap.String("team_name", teamName))
		return nil, fmt.Errorf("failed to check team: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("team not found")
	}

	// Get team members
	users, err := s.GetUsersByTeam(ctx, teamName)
	if err != nil {
		return nil, err
	}

	team := &domain.Team{
		TeamName: teamName,
		Members:  make([]domain.TeamMember, len(users)),
	}

	for i, user := range users {
		team.Members[i] = domain.TeamMember{
			UserID:   user.UserID,
			Username: user.Username,
			IsActive: user.IsActive,
		}
	}

	return team, nil
}

func (s *PostgresStorage) TeamExists(ctx context.Context, teamName string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)`, teamName).Scan(&exists)
	return exists, err
}

// PR operations
func (s *PostgresStorage) CreatePR(ctx context.Context, pr *domain.PullRequest) error {
	log := logger.FromContext(ctx)
	query := `INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7)`

	now := time.Now()
	pr.CreatedAt = &now
	pr.Status = domain.StatusOpen

	_, err := s.db.ExecContext(ctx, query, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status,
		pq.Array(pr.AssignedReviewers), pr.CreatedAt, now)
	if err != nil {
		log.Error(ctx, "failed to create PR", zap.Error(err), zap.String("pr_id", pr.PullRequestID))
		return fmt.Errorf("failed to create PR: %w", err)
	}

	log.Info(ctx, "PR created", zap.String("pr_id", pr.PullRequestID), zap.String("author_id", pr.AuthorID),
		zap.Strings("assigned_reviewers", pr.AssignedReviewers))
	return nil
}

func (s *PostgresStorage) GetPR(ctx context.Context, prID string) (*domain.PullRequest, error) {
	log := logger.FromContext(ctx)
	query := `SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at, updated_at 
              FROM pull_requests WHERE pull_request_id = $1`

	pr := &domain.PullRequest{}
	var createdAt, updatedAt time.Time
	var mergedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, prID).Scan(
		&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status,
		pq.Array(&pr.AssignedReviewers), &createdAt, &mergedAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("PR not found")
	}
	if err != nil {
		log.Error(ctx, "failed to get PR", zap.Error(err), zap.String("pr_id", prID))
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	pr.CreatedAt = &createdAt
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}

	return pr, nil
}

func (s *PostgresStorage) UpdatePR(ctx context.Context, pr *domain.PullRequest) error {
	log := logger.FromContext(ctx)
	query := `UPDATE pull_requests 
              SET pull_request_name = $1, status = $2, assigned_reviewers = $3, merged_at = $4, updated_at = $5
              WHERE pull_request_id = $6`

	now := time.Now()

	result, err := s.db.ExecContext(ctx, query, pr.PullRequestName, pr.Status, pq.Array(pr.AssignedReviewers),
		pr.MergedAt, now, pr.PullRequestID)
	if err != nil {
		log.Error(ctx, "failed to update PR", zap.Error(err), zap.String("pr_id", pr.PullRequestID))
		return fmt.Errorf("failed to update PR: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("PR not found")
	}

	log.Info(ctx, "PR updated", zap.String("pr_id", pr.PullRequestID), zap.String("status", string(pr.Status)))
	return nil
}

func (s *PostgresStorage) GetPRsByReviewer(ctx context.Context, userID string) ([]*domain.PullRequest, error) {
	log := logger.FromContext(ctx)
	query := `SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at, updated_at 
              FROM pull_requests WHERE $1 = ANY(assigned_reviewers)`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		log.Error(ctx, "failed to get PRs by reviewer", zap.Error(err), zap.String("user_id", userID))
		return nil, fmt.Errorf("failed to get PRs: %w", err)
	}
	defer rows.Close()

	var prs []*domain.PullRequest
	for rows.Next() {
		pr := &domain.PullRequest{}
		var createdAt, updatedAt time.Time
		var mergedAt sql.NullTime

		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status,
			pq.Array(&pr.AssignedReviewers), &createdAt, &mergedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan PR: %w", err)
		}

		pr.CreatedAt = &createdAt
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

func (s *PostgresStorage) PRExists(ctx context.Context, prID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`, prID).Scan(&exists)
	return exists, err
}

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Meldy183/code-storage-service/internal/domain"
	"github.com/Meldy183/shared/pkg/logger"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

type Storage struct {
	db *sql.DB
}

func NewPostgresStorage(ctx context.Context, host, port, user, password, dbname, sslmode string) (*Storage, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
	log := logger.FromContext(ctx)
	log.Debug(ctx, "destination:",
		zap.String("dsn", dsn),
	)
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

	log.Info(ctx, "postgres storage initialized successfully",
		zap.String("host", host),
		zap.String("port", port),
		zap.String("dbname", dbname),
	)
	return &Storage{db: db}, nil
}

func (s *Storage) Close(ctx context.Context) error {
	log := logger.FromContext(ctx)
	if err := s.db.Close(); err != nil {
		log.Error(ctx, "failed to close database connection", zap.Error(err))
		return err
	}
	log.Info(ctx, "database connection closed successfully")
	return nil
}

// TeamExists checks if team exists
func (s *Storage) TeamExists(ctx context.Context, teamID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM teams WHERE id = $1)`
	err := s.db.QueryRowContext(ctx, query, teamID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check team existence: %w", err)
	}
	return exists, nil
}

// InitRepository creates a root commit for a new repository
func (s *Storage) InitRepository(ctx context.Context, teamID uuid.UUID, commitName string, code []byte) (*domain.Commit, error) {
	commitID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO commits (id, team_id, root_commit, parent_commit_ids, code, created_at)
		VALUES ($1, $2, $1, $3, $4, $5)
		RETURNING id, team_id, root_commit, parent_commit_ids, created_at
	`

	var commit domain.Commit
	var parentIDs pq.StringArray

	err := s.db.QueryRowContext(ctx, query, commitID, teamID, pq.StringArray{}, code, now).Scan(
		&commit.ID,
		&commit.TeamID,
		&commit.RootCommit,
		&parentIDs,
		&commit.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root commit: %w", err)
	}

	commit.ParentCommitIDs = stringArrayToUUIDs(parentIDs)
	commit.Code = code

	// Save commit name if provided
	if commitName != "" {
		if err := s.SetCommitName(ctx, teamID, commit.RootCommit, commit.ID, commitName); err != nil {
			return nil, fmt.Errorf("failed to set commit name: %w", err)
		}
		commit.CommitName = &commitName
	}

	return &commit, nil
}

// GetCommit retrieves a commit by its identifiers
func (s *Storage) GetCommit(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) (*domain.Commit, error) {
	query := `
		SELECT c.id, c.team_id, c.root_commit, c.parent_commit_ids, c.created_at, cn.name
		FROM commits c
		LEFT JOIN commit_names cn ON c.id = cn.commit_id
		WHERE c.id = $1 AND c.team_id = $2 AND c.root_commit = $3
	`

	var commit domain.Commit
	var parentIDs pq.StringArray
	var name sql.NullString

	err := s.db.QueryRowContext(ctx, query, commitID, teamID, rootCommit).Scan(
		&commit.ID,
		&commit.TeamID,
		&commit.RootCommit,
		&parentIDs,
		&commit.CreatedAt,
		&name,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCommitNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	commit.ParentCommitIDs = stringArrayToUUIDs(parentIDs)
	if name.Valid {
		commit.CommitName = &name.String
	}

	return &commit, nil
}

// GetCommitCode retrieves the code of a commit
func (s *Storage) GetCommitCode(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) ([]byte, error) {
	query := `
		SELECT code FROM commits 
		WHERE id = $1 AND team_id = $2 AND root_commit = $3
	`

	var code []byte
	err := s.db.QueryRowContext(ctx, query, commitID, teamID, rootCommit).Scan(&code)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCommitNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get commit code: %w", err)
	}

	return code, nil
}

// CreateCommit creates a new commit with a parent
func (s *Storage) CreateCommit(ctx context.Context, teamID, rootCommit, parentID uuid.UUID, commitName string, code []byte) (*domain.Commit, error) {
	commitID := uuid.New()
	now := time.Now()
	parentIDs := pq.StringArray{parentID.String()}

	query := `
		INSERT INTO commits (id, team_id, root_commit, parent_commit_ids, code, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, team_id, root_commit, parent_commit_ids, created_at
	`

	var commit domain.Commit
	var returnedParentIDs pq.StringArray

	err := s.db.QueryRowContext(ctx, query, commitID, teamID, rootCommit, parentIDs, code, now).Scan(
		&commit.ID,
		&commit.TeamID,
		&commit.RootCommit,
		&returnedParentIDs,
		&commit.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit: %w", err)
	}

	commit.ParentCommitIDs = stringArrayToUUIDs(returnedParentIDs)
	commit.Code = code

	// Save commit name if provided
	if commitName != "" {
		if err := s.SetCommitName(ctx, teamID, rootCommit, commit.ID, commitName); err != nil {
			return nil, fmt.Errorf("failed to set commit name: %w", err)
		}
		commit.CommitName = &commitName
	}

	return &commit, nil
}

// MergeCommits creates a merge commit from two parent commits
func (s *Storage) MergeCommits(ctx context.Context, teamID, rootCommit, commitID1, commitID2 uuid.UUID) (*domain.Commit, error) {
	// Get code from the first commit (simplified merge - just use first commit's code)
	code, err := s.GetCommitCode(ctx, teamID, rootCommit, commitID1)
	if err != nil {
		return nil, err
	}

	commitID := uuid.New()
	now := time.Now()
	parentIDs := pq.StringArray{commitID1.String(), commitID2.String()}

	query := `
		INSERT INTO commits (id, team_id, root_commit, parent_commit_ids, code, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, team_id, root_commit, parent_commit_ids, created_at
	`

	var commit domain.Commit
	var returnedParentIDs pq.StringArray

	err = s.db.QueryRowContext(ctx, query, commitID, teamID, rootCommit, parentIDs, code, now).Scan(
		&commit.ID,
		&commit.TeamID,
		&commit.RootCommit,
		&returnedParentIDs,
		&commit.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge commit: %w", err)
	}

	commit.ParentCommitIDs = stringArrayToUUIDs(returnedParentIDs)
	commit.Code = code

	return &commit, nil
}

// IsLeafCommit checks if a commit has no children
func (s *Storage) IsLeafCommit(ctx context.Context, teamID, rootCommit, commitID uuid.UUID) (bool, error) {
	query := `
		SELECT NOT EXISTS(
			SELECT 1 FROM commits 
			WHERE team_id = $1 AND root_commit = $2 AND $3 = ANY(parent_commit_ids)
		)
	`

	var isLeaf bool
	err := s.db.QueryRowContext(ctx, query, teamID, rootCommit, commitID.String()).Scan(&isLeaf)
	if err != nil {
		return false, fmt.Errorf("failed to check if commit is leaf: %w", err)
	}

	return isLeaf, nil
}

// RootCommitExists checks if root commit exists for a team
func (s *Storage) RootCommitExists(ctx context.Context, teamID, rootCommit uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM commits 
			WHERE team_id = $1 AND id = $2 AND root_commit = $2
		)
	`

	var exists bool
	err := s.db.QueryRowContext(ctx, query, teamID, rootCommit).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check root commit existence: %w", err)
	}

	return exists, nil
}

// GetCommitName retrieves the name of a commit
func (s *Storage) GetCommitName(ctx context.Context, commitID uuid.UUID) (string, error) {
	query := `SELECT name FROM commit_names WHERE commit_id = $1`

	var name string
	err := s.db.QueryRowContext(ctx, query, commitID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", domain.ErrCommitNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get commit name: %w", err)
	}

	return name, nil
}

// GetCommitIDByName retrieves commit ID by its name within a repository
func (s *Storage) GetCommitIDByName(ctx context.Context, teamID, rootCommit uuid.UUID, name string) (uuid.UUID, error) {
	query := `SELECT commit_id FROM commit_names WHERE team_id = $1 AND root_commit = $2 AND name = $3`

	var commitID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, teamID, rootCommit, name).Scan(&commitID)
	if err == sql.ErrNoRows {
		return uuid.Nil, domain.ErrCommitNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get commit by name: %w", err)
	}

	return commitID, nil
}

// SetCommitName sets a name for a commit
func (s *Storage) SetCommitName(ctx context.Context, teamID, rootCommit, commitID uuid.UUID, name string) error {
	query := `
		INSERT INTO commit_names (team_id, root_commit, commit_id, name)
		VALUES ($1, $2, $3, $4)
	`

	_, err := s.db.ExecContext(ctx, query, teamID, rootCommit, commitID, name)
	if err != nil {
		return fmt.Errorf("failed to set commit name: %w", err)
	}

	return nil
}

// Helper function to convert string array to UUID slice
func stringArrayToUUIDs(arr pq.StringArray) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(arr))
	for _, s := range arr {
		if id, err := uuid.Parse(s); err == nil {
			result = append(result, id)
		}
	}
	return result
}

package storage

import "context"

type Storage interface {
	getCodeByID(ctx context.Context, id string) ([]byte, error)
	createInitialCommit(ctx context.Context, name string, code []byte) (string, error)
	createCommit(ctx context.Context, name string, parentID string, code []byte) (string, error)
	mergeCommits(ctx context.Context, name string, parentIDs []string, code []byte) (string, error)
}

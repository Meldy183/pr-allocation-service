-- Teams table (teams can be managed by external service, but we need reference)
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Commits table - stores all commits
CREATE TABLE IF NOT EXISTS commits (
    id UUID PRIMARY KEY,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    root_commit UUID NOT NULL,
    parent_commit_ids TEXT[] NOT NULL DEFAULT '{}',
    code BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Index for faster lookups
    CONSTRAINT fk_root_commit CHECK (
        (id = root_commit AND parent_commit_ids = '{}') OR
        (id != root_commit AND array_length(parent_commit_ids, 1) >= 1)
    )
);

-- Index for team + root commit queries
CREATE INDEX IF NOT EXISTS idx_commits_team_root ON commits(team_id, root_commit);

-- Index for finding children of a commit
CREATE INDEX IF NOT EXISTS idx_commits_parent_ids ON commits USING GIN(parent_commit_ids);

-- Commit names table - maps human-readable names to commits
CREATE TABLE IF NOT EXISTS commit_names (
    team_id UUID NOT NULL,
    root_commit UUID NOT NULL,
    commit_id UUID NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (team_id, root_commit, name),
    UNIQUE (commit_id)
);

-- Index for lookup by commit_id
CREATE INDEX IF NOT EXISTS idx_commit_names_commit_id ON commit_names(commit_id);


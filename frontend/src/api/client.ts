const API_BASE = '/api';

// Types
export interface TeamMember {
  user_id?: string;
  username: string;
  is_active: boolean;
}

export interface Team {
  team_id?: string;
  team_name: string;
  members: TeamMember[];
}

export interface UserProfile {
  user_id?: string;
  username: string;
  team_id?: string;
  team_name: string;
  is_active: boolean;
}

export interface Commit {
  commit_id?: string;
  root_commit?: string;
  parent_commit_ids?: string[];
  commit_name?: string;
  repo_name?: string;
  created_at: string;
}

export interface PullRequest {
  pr_id?: string;
  pr_name: string;
  title: string;
  author_id?: string;
  author_name: string;
  status: 'OPEN' | 'MERGED' | 'REJECTED';
  reviewer_ids?: string[];
  reviewer_names?: string[];
  source_commit_id?: string;
  source_commit_name?: string;
  target_commit_id?: string;
  target_commit_name?: string;
  root_commit_id?: string;
  repo_name?: string;
  team_name?: string;
  created_at: string;
  merged_at?: string;
}

export interface CreateTeamRequest {
  team_name: string;
  members: { username: string; is_active?: boolean }[];
}

export interface CreatePRRequest {
  title: string;
  pr_name: string;
  team_name: string;
  repo_name: string;
  source_commit_name: string;
  target_commit_name: string;
}

// Helper to get headers
function getHeaders(username: string): HeadersInit {
  return {
    'Content-Type': 'application/json',
    'X-Username': username,
  };
}

// API functions
export async function getProfile(username: string): Promise<UserProfile> {
  const res = await fetch(`${API_BASE}/me`, {
    headers: getHeaders(username),
  });
  if (!res.ok) throw new Error('Failed to get profile');
  const data = await res.json();
  return data.user;
}

export async function createTeam(username: string, req: CreateTeamRequest): Promise<Team> {
  const res = await fetch(`${API_BASE}/team/create`, {
    method: 'POST',
    headers: getHeaders(username),
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Failed to create team');
  }
  const data = await res.json();
  return data.team;
}

export async function getTeam(username: string, teamName: string): Promise<Team> {
  const res = await fetch(`${API_BASE}/team/get?team_name=${encodeURIComponent(teamName)}`, {
    headers: getHeaders(username),
  });
  if (!res.ok) throw new Error('Team not found');
  const data = await res.json();
  return data.team;
}

export async function initRepository(
  username: string,
  teamName: string,
  repoName: string,
  commitName: string,
  code: File
): Promise<Commit> {
  const formData = new FormData();
  formData.append('team_name', teamName);
  formData.append('repo_name', repoName);
  formData.append('commit_name', commitName);
  formData.append('code', code);

  const res = await fetch(`${API_BASE}/repo/init`, {
    method: 'POST',
    headers: { 'X-Username': username },
    body: formData,
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Failed to init repository');
  }
  const data = await res.json();
  return data.commit;
}

export async function pushCommit(
  username: string,
  teamName: string,
  repoName: string,
  parentCommitName: string,
  commitName: string,
  code: File
): Promise<Commit> {
  const formData = new FormData();
  formData.append('team_name', teamName);
  formData.append('repo_name', repoName);
  formData.append('parent_commit_name', parentCommitName);
  formData.append('commit_name', commitName);
  formData.append('code', code);

  const res = await fetch(`${API_BASE}/repo/push`, {
    method: 'POST',
    headers: { 'X-Username': username },
    body: formData,
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Failed to push commit');
  }
  const data = await res.json();
  return data.commit;
}

export async function checkout(
  username: string,
  teamName: string,
  repoName: string,
  commitName: string
): Promise<Blob> {
  const params = new URLSearchParams({
    team_name: teamName,
    repo_name: repoName,
    commit_name: commitName,
  });
  const res = await fetch(`${API_BASE}/repo/checkout?${params}`, {
    headers: { 'X-Username': username },
  });
  if (!res.ok) throw new Error('Failed to checkout');
  return res.blob();
}

export async function listCommits(
  username: string,
  teamName: string,
  repoName: string
): Promise<Commit[]> {
  const params = new URLSearchParams({
    team_name: teamName,
    repo_name: repoName,
  });
  const res = await fetch(`${API_BASE}/repo/commits?${params}`, {
    headers: getHeaders(username),
  });
  if (!res.ok) {
    return []; // Return empty if repo not found
  }
  const data = await res.json();
  return data.commits || [];
}

export async function createPR(username: string, req: CreatePRRequest): Promise<PullRequest> {
  const res = await fetch(`${API_BASE}/pr/create`, {
    method: 'POST',
    headers: getHeaders(username),
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Failed to create PR');
  }
  const data = await res.json();
  return data.pull_request;
}

export async function getMyPRs(username: string, status?: string): Promise<PullRequest[]> {
  const params = status ? `?status=${status}` : '';
  const res = await fetch(`${API_BASE}/pr/my${params}`, {
    headers: getHeaders(username),
  });
  if (!res.ok) throw new Error('Failed to get PRs');
  const data = await res.json();
  return data.pull_requests || [];
}

export async function getReviewPRs(username: string, status?: string): Promise<PullRequest[]> {
  const params = status ? `?status=${status}` : '';
  const res = await fetch(`${API_BASE}/pr/reviews${params}`, {
    headers: getHeaders(username),
  });
  if (!res.ok) throw new Error('Failed to get review PRs');
  const data = await res.json();
  return data.pull_requests || [];
}

export async function approvePR(
  username: string,
  teamName: string,
  prName: string
): Promise<{ pull_request: PullRequest; merge_commit: Commit }> {
  const params = new URLSearchParams({ team_name: teamName, pr_name: prName });
  const res = await fetch(`${API_BASE}/pr/approve?${params}`, {
    method: 'POST',
    headers: getHeaders(username),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Failed to approve PR');
  }
  return res.json();
}

export async function rejectPR(
  username: string,
  teamName: string,
  prName: string,
  reason?: string
): Promise<PullRequest> {
  const params = new URLSearchParams({ team_name: teamName, pr_name: prName });
  const res = await fetch(`${API_BASE}/pr/reject?${params}`, {
    method: 'POST',
    headers: getHeaders(username),
    body: JSON.stringify({ reason }),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Failed to reject PR');
  }
  const data = await res.json();
  return data.pull_request;
}

export async function getPRCode(
  username: string,
  teamName: string,
  prName: string
): Promise<Blob> {
  const params = new URLSearchParams({ team_name: teamName, pr_name: prName });
  const res = await fetch(`${API_BASE}/pr/code?${params}`, {
    headers: { 'X-Username': username },
  });
  if (!res.ok) throw new Error('Failed to get PR code');
  return res.blob();
}


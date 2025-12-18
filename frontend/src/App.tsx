import { useState, ChangeEvent } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { BranchGraph, CommitNode } from '@/components/BranchGraph';
import { PRList } from '@/components/PRList';
import * as api from '@/api/client';
import {
  GitBranch,
  Users,
  FolderGit2,
  GitPullRequest,
  Upload,
  Download,
  Plus,
  LogIn,
  LogOut,
} from 'lucide-react';

type Tab = 'repository' | 'prs' | 'team';

function App() {
  // Auth state
  const [username, setUsername] = useState('');
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [profile, setProfile] = useState<api.UserProfile | null>(null);

  // Team state
  const [team, setTeam] = useState<api.Team | null>(null);
  const [newTeamName, setNewTeamName] = useState('');
  const [newMemberUsername, setNewMemberUsername] = useState('');
  const [newMembers, setNewMembers] = useState<string[]>([]);

  // Repository state
  const [currentRepo, setCurrentRepo] = useState('');
  const [commits, setCommits] = useState<CommitNode[]>([]);
  const [selectedCommit, setSelectedCommit] = useState<string | null>(null);
  const [newRepoName, setNewRepoName] = useState('');
  const [newCommitName, setNewCommitName] = useState('');
  const [parentCommitName, setParentCommitName] = useState('');

  // PR state
  const [myPRs, setMyPRs] = useState<api.PullRequest[]>([]);
  const [reviewPRs, setReviewPRs] = useState<api.PullRequest[]>([]);
  const [prTitle, setPrTitle] = useState('');
  const [prName, setPrName] = useState('');
  const [sourceCommitName, setSourceCommitName] = useState('');
  const [targetCommitName, setTargetCommitName] = useState('');

  // UI state
  const [activeTab, setActiveTab] = useState<Tab>('repository');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Login handler
  const handleLogin = async () => {
    if (!username.trim()) {
      setError('Username is required');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const userProfile = await api.getProfile(username);
      setProfile(userProfile);
      setIsLoggedIn(true);

      // Load team info
      if (userProfile.team_name) {
        const teamData = await api.getTeam(username, userProfile.team_name);
        setTeam(teamData);
      }

      // Load PRs
      const [my, reviews] = await Promise.all([
        api.getMyPRs(username),
        api.getReviewPRs(username),
      ]);
      setMyPRs(my);
      setReviewPRs(reviews);
    } catch (err) {
      // User doesn't exist yet, that's okay
      setIsLoggedIn(true);
      setProfile({ username, team_name: '', is_active: true });
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = () => {
    setIsLoggedIn(false);
    setProfile(null);
    setTeam(null);
    setUsername('');
    setCommits([]);
    setMyPRs([]);
    setReviewPRs([]);
  };

  // Team handlers
  const handleAddMember = () => {
    if (newMemberUsername.trim() && !newMembers.includes(newMemberUsername.trim())) {
      setNewMembers([...newMembers, newMemberUsername.trim()]);
      setNewMemberUsername('');
    }
  };

  const handleCreateTeam = async () => {
    if (!newTeamName.trim()) {
      setError('Team name is required');
      return;
    }
    if (newMembers.length === 0) {
      setError('At least one member is required');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const createdTeam = await api.createTeam(username, {
        team_name: newTeamName,
        members: newMembers.map((m) => ({ username: m, is_active: true })),
      });
      setTeam(createdTeam);
      setNewTeamName('');
      setNewMembers([]);
      // Refresh profile
      const userProfile = await api.getProfile(username);
      setProfile(userProfile);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // Repository handlers
  const handleInitRepo = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !newRepoName.trim() || !team) {
      setError('Repository name and code file are required');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const commit = await api.initRepository(username, team.team_name, newRepoName, file);
      const newCommit: CommitNode = {
        id: commit.commit_id || commit.root_commit || '',
        name: commit.commit_name || newRepoName,
        parentIds: [],
        isRoot: true,
        branch: 'main',
      };
      setCommits([newCommit]);
      setCurrentRepo(newRepoName);
      setNewRepoName('');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handlePush = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !newCommitName.trim() || !parentCommitName.trim() || !team || !currentRepo) {
      setError('All fields are required for push');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const commit = await api.pushCommit(
        username,
        team.team_name,
        currentRepo,
        parentCommitName,
        newCommitName,
        file
      );
      const newCommit: CommitNode = {
        id: commit.commit_id || '',
        name: commit.commit_name || newCommitName,
        parentIds: commit.parent_commit_ids || [],
        branch: newCommitName.includes('feature') ? newCommitName : 'main',
      };
      setCommits([newCommit, ...commits]);
      setNewCommitName('');
      setParentCommitName('');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleCheckout = async () => {
    if (!selectedCommit || !team || !currentRepo) return;
    const commit = commits.find((c) => c.id === selectedCommit);
    if (!commit) return;

    try {
      const blob = await api.checkout(username, team.team_name, currentRepo, commit.name);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${commit.name}.zip`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err: any) {
      setError(err.message);
    }
  };

  // PR handlers
  const handleCreatePR = async () => {
    if (!prTitle.trim() || !prName.trim() || !sourceCommitName.trim() || !targetCommitName.trim() || !team || !currentRepo) {
      setError('All fields are required for PR');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const pr = await api.createPR(username, {
        title: prTitle,
        pr_name: prName,
        team_name: team.team_name,
        repo_name: currentRepo,
        source_commit_name: sourceCommitName,
        target_commit_name: targetCommitName,
      });
      setMyPRs([pr, ...myPRs]);
      setPrTitle('');
      setPrName('');
      setSourceCommitName('');
      setTargetCommitName('');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleApprovePR = async (pr: api.PullRequest) => {
    if (!pr.team_name) return;
    setLoading(true);
    try {
      const result = await api.approvePR(username, pr.team_name, pr.pr_name);
      // Update PR in list
      setReviewPRs(reviewPRs.map((p) => (p.pr_name === pr.pr_name ? result.pull_request : p)));
      // Add merge commit
      if (result.merge_commit) {
        const mergeCommit: CommitNode = {
          id: result.merge_commit.commit_id || '',
          name: `merge-${pr.pr_name}`,
          parentIds: result.merge_commit.parent_commit_ids || [],
          isMerge: true,
          branch: 'main',
        };
        setCommits([mergeCommit, ...commits]);
      }
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleRejectPR = async (pr: api.PullRequest) => {
    if (!pr.team_name) return;
    setLoading(true);
    try {
      const result = await api.rejectPR(username, pr.team_name, pr.pr_name, 'Rejected via UI');
      setReviewPRs(reviewPRs.map((p) => (p.pr_name === pr.pr_name ? result : p)));
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleViewPRCode = async (pr: api.PullRequest) => {
    if (!pr.team_name) return;
    try {
      const blob = await api.getPRCode(username, pr.team_name, pr.pr_name);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${pr.pr_name}-code.zip`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err: any) {
      setError(err.message);
    }
  };

  // Login screen
  if (!isLoggedIn) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <div className="flex justify-center mb-4">
              <GitBranch className="w-12 h-12 text-blue-600" />
            </div>
            <CardTitle>Git Simulation Platform</CardTitle>
            <CardDescription>Enter your username to continue</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Input
              placeholder="Username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleLogin()}
            />
            {error && <p className="text-red-500 text-sm">{error}</p>}
            <Button className="w-full" onClick={handleLogin} disabled={loading}>
              <LogIn className="w-4 h-4 mr-2" />
              {loading ? 'Loading...' : 'Login'}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Header */}
      <header className="bg-white border-b border-slate-200 px-6 py-4">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-4">
            <GitBranch className="w-8 h-8 text-blue-600" />
            <h1 className="text-xl font-bold">Git Simulation</h1>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-slate-600">Logged in as</span>
              <Badge>{profile?.username}</Badge>
              {team && <Badge variant="secondary">{team.team_name}</Badge>}
            </div>
            <Button variant="ghost" onClick={handleLogout}>
              <LogOut className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </header>

      {/* Error banner */}
      {error && (
        <div className="bg-red-50 border-b border-red-200 px-6 py-3">
          <div className="max-w-7xl mx-auto flex items-center justify-between">
            <p className="text-red-600">{error}</p>
            <Button variant="ghost" size="sm" onClick={() => setError(null)}>
              Dismiss
            </Button>
          </div>
        </div>
      )}

      {/* Main content */}
      <main className="max-w-7xl mx-auto p-6">
        {/* Tabs */}
        <div className="flex gap-2 mb-6">
          <Button
            variant={activeTab === 'repository' ? 'default' : 'outline'}
            onClick={() => setActiveTab('repository')}
          >
            <FolderGit2 className="w-4 h-4 mr-2" />
            Repository
          </Button>
          <Button
            variant={activeTab === 'prs' ? 'default' : 'outline'}
            onClick={() => setActiveTab('prs')}
          >
            <GitPullRequest className="w-4 h-4 mr-2" />
            Pull Requests
            {(myPRs.length + reviewPRs.length > 0) && (
              <Badge variant="secondary" className="ml-2">
                {myPRs.length + reviewPRs.length}
              </Badge>
            )}
          </Button>
          <Button
            variant={activeTab === 'team' ? 'default' : 'outline'}
            onClick={() => setActiveTab('team')}
          >
            <Users className="w-4 h-4 mr-2" />
            Team
          </Button>
        </div>

        {/* Repository Tab */}
        {activeTab === 'repository' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Branch visualization */}
            <div className="lg:col-span-2">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center justify-between">
                    <span className="flex items-center gap-2">
                      <GitBranch className="w-5 h-5" />
                      {currentRepo || 'No Repository'}
                    </span>
                    {selectedCommit && (
                      <Button variant="outline" size="sm" onClick={handleCheckout}>
                        <Download className="w-4 h-4 mr-2" />
                        Checkout
                      </Button>
                    )}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <BranchGraph
                    commits={commits}
                    selectedCommit={selectedCommit || undefined}
                    onCommitClick={(commit) => setSelectedCommit(commit.id)}
                  />
                </CardContent>
              </Card>
            </div>

            {/* Actions panel */}
            <div className="space-y-6">
              {/* Init Repository */}
              {!currentRepo && team && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">Initialize Repository</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <Input
                      placeholder="Repository name"
                      value={newRepoName}
                      onChange={(e) => setNewRepoName(e.target.value)}
                    />
                    <div>
                      <label className="block text-sm font-medium mb-2">
                        Initial code (ZIP)
                      </label>
                      <Input
                        type="file"
                        accept=".zip"
                        onChange={handleInitRepo}
                        disabled={!newRepoName || loading}
                      />
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* Push Commit */}
              {currentRepo && team && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Upload className="w-5 h-5" />
                      Push Commit
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <Input
                      placeholder="Parent commit name"
                      value={parentCommitName}
                      onChange={(e) => setParentCommitName(e.target.value)}
                    />
                    <Input
                      placeholder="New commit name"
                      value={newCommitName}
                      onChange={(e) => setNewCommitName(e.target.value)}
                    />
                    <div>
                      <label className="block text-sm font-medium mb-2">
                        Code (ZIP)
                      </label>
                      <Input
                        type="file"
                        accept=".zip"
                        onChange={handlePush}
                        disabled={!parentCommitName || !newCommitName || loading}
                      />
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* Create PR */}
              {currentRepo && team && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <GitPullRequest className="w-5 h-5" />
                      Create Pull Request
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <Input
                      placeholder="PR title"
                      value={prTitle}
                      onChange={(e) => setPrTitle(e.target.value)}
                    />
                    <Input
                      placeholder="PR name (unique identifier)"
                      value={prName}
                      onChange={(e) => setPrName(e.target.value)}
                    />
                    <Input
                      placeholder="Source commit name"
                      value={sourceCommitName}
                      onChange={(e) => setSourceCommitName(e.target.value)}
                    />
                    <Input
                      placeholder="Target commit name"
                      value={targetCommitName}
                      onChange={(e) => setTargetCommitName(e.target.value)}
                    />
                    <Button
                      className="w-full"
                      onClick={handleCreatePR}
                      disabled={loading || !prTitle || !prName || !sourceCommitName || !targetCommitName}
                    >
                      Create PR
                    </Button>
                  </CardContent>
                </Card>
              )}

              {!team && (
                <Card>
                  <CardContent className="py-8 text-center text-slate-500">
                    <Users className="w-12 h-12 mx-auto mb-4 text-slate-400" />
                    <p>Create or join a team first</p>
                    <Button
                      variant="link"
                      onClick={() => setActiveTab('team')}
                    >
                      Go to Team settings
                    </Button>
                  </CardContent>
                </Card>
              )}
            </div>
          </div>
        )}

        {/* Pull Requests Tab */}
        {activeTab === 'prs' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <PRList
              title="My Pull Requests"
              pullRequests={myPRs}
              onViewCode={handleViewPRCode}
            />
            <PRList
              title="Pending Reviews"
              pullRequests={reviewPRs.filter((pr) => pr.status === 'OPEN')}
              isReviewer
              onApprove={handleApprovePR}
              onReject={handleRejectPR}
              onViewCode={handleViewPRCode}
            />
          </div>
        )}

        {/* Team Tab */}
        {activeTab === 'team' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Current team */}
            {team ? (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Users className="w-5 h-5" />
                    {team.team_name}
                    {team.team_id && (
                      <Badge variant="secondary" className="text-xs font-mono">
                        {team.team_id.slice(0, 8)}...
                      </Badge>
                    )}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2">
                    {team.members.map((member) => (
                      <div
                        key={member.username}
                        className="flex items-center justify-between p-3 bg-slate-50 rounded-lg"
                      >
                        <div className="flex items-center gap-2">
                          <div className="w-8 h-8 bg-blue-100 rounded-full flex items-center justify-center">
                            <span className="text-blue-600 font-medium">
                              {member.username[0].toUpperCase()}
                            </span>
                          </div>
                          <span className="font-medium">{member.username}</span>
                          {member.username === username && (
                            <Badge variant="secondary" className="text-xs">You</Badge>
                          )}
                        </div>
                        <Badge variant={member.is_active ? 'success' : 'secondary'}>
                          {member.is_active ? 'Active' : 'Inactive'}
                        </Badge>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            ) : (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Plus className="w-5 h-5" />
                    Create Team
                  </CardTitle>
                  <CardDescription>
                    Create a new team to collaborate on repositories
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <Input
                    placeholder="Team name"
                    value={newTeamName}
                    onChange={(e) => setNewTeamName(e.target.value)}
                  />
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Members</label>
                    <div className="flex gap-2">
                      <Input
                        placeholder="Username"
                        value={newMemberUsername}
                        onChange={(e) => setNewMemberUsername(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && handleAddMember()}
                      />
                      <Button variant="outline" onClick={handleAddMember}>
                        <Plus className="w-4 h-4" />
                      </Button>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {newMembers.map((m) => (
                        <Badge
                          key={m}
                          variant="secondary"
                          className="cursor-pointer"
                          onClick={() => setNewMembers(newMembers.filter((x) => x !== m))}
                        >
                          {m} ×
                        </Badge>
                      ))}
                    </div>
                  </div>
                  <Button
                    className="w-full"
                    onClick={handleCreateTeam}
                    disabled={loading || !newTeamName || newMembers.length === 0}
                  >
                    Create Team
                  </Button>
                </CardContent>
              </Card>
            )}

            {/* Profile info */}
            <Card>
              <CardHeader>
                <CardTitle>Your Profile</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex items-center gap-4">
                    <div className="w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center">
                      <span className="text-2xl text-blue-600 font-bold">
                        {profile?.username[0].toUpperCase()}
                      </span>
                    </div>
                    <div>
                      <h3 className="font-bold text-lg">{profile?.username}</h3>
                      {profile?.team_name && (
                        <p className="text-slate-500">Team: {profile.team_name}</p>
                      )}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4 pt-4 border-t">
                    <div className="text-center">
                      <div className="text-2xl font-bold text-blue-600">{myPRs.length}</div>
                      <div className="text-sm text-slate-500">My PRs</div>
                    </div>
                    <div className="text-center">
                      <div className="text-2xl font-bold text-green-600">
                        {reviewPRs.filter((pr) => pr.status === 'OPEN').length}
                      </div>
                      <div className="text-sm text-slate-500">Pending Reviews</div>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        )}
      </main>
    </div>
  );
}

export default App;


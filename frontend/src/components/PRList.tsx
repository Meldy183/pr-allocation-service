import React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { PullRequest } from '@/api/client';
import { Check, X, FileCode, User, GitBranch } from 'lucide-react';

interface PRListProps {
  pullRequests: PullRequest[];
  title: string;
  isReviewer?: boolean;
  onApprove?: (pr: PullRequest) => void;
  onReject?: (pr: PullRequest) => void;
  onViewCode?: (pr: PullRequest) => void;
}

export function PRList({ pullRequests, title, isReviewer, onApprove, onReject, onViewCode }: PRListProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'OPEN':
        return <Badge variant="warning">Open</Badge>;
      case 'MERGED':
        return <Badge variant="success">Merged</Badge>;
      case 'REJECTED':
        return <Badge variant="destructive">Rejected</Badge>;
      default:
        return <Badge variant="secondary">{status}</Badge>;
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <GitBranch className="w-5 h-5" />
          {title}
          <Badge variant="secondary" className="ml-2">{pullRequests.length}</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {pullRequests.length === 0 ? (
          <p className="text-slate-500 text-center py-4">No pull requests</p>
        ) : (
          <div className="space-y-4">
            {pullRequests.map((pr) => (
              <div
                key={pr.pr_id || pr.pr_name}
                className="border rounded-lg p-4 hover:bg-slate-50 transition-colors"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-medium text-lg">{pr.title}</span>
                      {getStatusBadge(pr.status)}
                    </div>
                    <div className="text-sm text-slate-500 space-y-1">
                      <div className="flex items-center gap-1">
                        <span className="font-mono bg-slate-100 px-1 rounded">{pr.pr_name}</span>
                      </div>
                      <div className="flex items-center gap-4">
                        <span className="flex items-center gap-1">
                          <User className="w-3 h-3" />
                          {pr.author_name}
                        </span>
                        {pr.team_name && (
                          <span>Team: {pr.team_name}</span>
                        )}
                        {pr.repo_name && (
                          <span>Repo: {pr.repo_name}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-green-600">
                          {pr.source_commit_name}
                        </span>
                        <span>→</span>
                        <span className="text-blue-600">
                          {pr.target_commit_name}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className="flex gap-2">
                    {onViewCode && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => onViewCode(pr)}
                      >
                        <FileCode className="w-4 h-4 mr-1" />
                        Code
                      </Button>
                    )}
                    {isReviewer && pr.status === 'OPEN' && (
                      <>
                        <Button
                          variant="success"
                          size="sm"
                          onClick={() => onApprove?.(pr)}
                        >
                          <Check className="w-4 h-4 mr-1" />
                          Approve
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => onReject?.(pr)}
                        >
                          <X className="w-4 h-4 mr-1" />
                          Reject
                        </Button>
                      </>
                    )}
                  </div>
                </div>

                {pr.merged_at && (
                  <div className="mt-2 text-xs text-slate-400">
                    Merged: {new Date(pr.merged_at).toLocaleString()}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}


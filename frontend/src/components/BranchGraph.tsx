import React from 'react';
import { cn } from '@/lib/utils';

export interface CommitNode {
  id: string;
  name: string;
  parentIds: string[];
  isRoot?: boolean;
  isMerge?: boolean;
  branch?: string;
}

interface BranchGraphProps {
  commits: CommitNode[];
  selectedCommit?: string;
  onCommitClick?: (commit: CommitNode) => void;
}

const BRANCH_COLORS = [
  'bg-blue-500',
  'bg-green-500',
  'bg-purple-500',
  'bg-orange-500',
  'bg-pink-500',
  'bg-cyan-500',
];

export function BranchGraph({ commits, selectedCommit, onCommitClick }: BranchGraphProps) {
  // Simple layout: vertical timeline
  const getBranchColor = (branch?: string) => {
    if (!branch) return BRANCH_COLORS[0];
    const index = Math.abs(branch.split('').reduce((a, b) => a + b.charCodeAt(0), 0)) % BRANCH_COLORS.length;
    return BRANCH_COLORS[index];
  };

  return (
    <div className="relative p-4">
      <svg className="absolute top-0 left-0 w-full h-full pointer-events-none" style={{ zIndex: 0 }}>
        {/* Draw lines between commits */}
        {commits.map((commit, index) => {
          const y = index * 80 + 40;
          return commit.parentIds.map((parentId) => {
            const parentIndex = commits.findIndex((c) => c.id === parentId);
            if (parentIndex === -1) return null;
            const parentY = parentIndex * 80 + 40;

            return (
              <line
                key={`${commit.id}-${parentId}`}
                x1="50"
                y1={y}
                x2="50"
                y2={parentY}
                stroke="#94a3b8"
                strokeWidth="2"
              />
            );
          });
        })}
      </svg>

      <div className="relative" style={{ zIndex: 1 }}>
        {commits.map((commit, index) => (
          <div
            key={commit.id}
            className="flex items-center gap-4 mb-8 cursor-pointer hover:bg-slate-50 rounded-lg p-2 transition-colors"
            onClick={() => onCommitClick?.(commit)}
          >
            {/* Commit dot */}
            <div className="relative flex-shrink-0 w-12 flex justify-center">
              <div
                className={cn(
                  'w-6 h-6 rounded-full border-4 border-white shadow-md transition-transform',
                  getBranchColor(commit.branch),
                  selectedCommit === commit.id && 'ring-2 ring-blue-400 ring-offset-2 scale-110',
                  commit.isMerge && 'ring-2 ring-purple-400'
                )}
              />
              {commit.isRoot && (
                <div className="absolute -bottom-1 left-1/2 -translate-x-1/2 text-[10px] text-slate-400">
                  root
                </div>
              )}
            </div>

            {/* Commit info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-mono text-sm font-medium text-slate-900 truncate">
                  {commit.name}
                </span>
                {commit.isMerge && (
                  <span className="text-xs bg-purple-100 text-purple-700 px-2 py-0.5 rounded">
                    merge
                  </span>
                )}
                {commit.branch && (
                  <span className="text-xs bg-slate-100 text-slate-600 px-2 py-0.5 rounded">
                    {commit.branch}
                  </span>
                )}
              </div>
              <div className="text-xs text-slate-500 font-mono truncate">
                {commit.id.slice(0, 8)}...
              </div>
            </div>
          </div>
        ))}

        {commits.length === 0 && (
          <div className="text-center text-slate-500 py-8">
            No commits yet. Initialize a repository to get started.
          </div>
        )}
      </div>
    </div>
  );
}


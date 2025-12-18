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
  '#3b82f6', // blue
  '#22c55e', // green
  '#a855f7', // purple
  '#f97316', // orange
  '#ec4899', // pink
  '#06b6d4', // cyan
  '#eab308', // yellow
];

interface LayoutNode {
  commit: CommitNode;
  x: number; // column (branch lane)
  y: number; // row
}

export function BranchGraph({ commits, selectedCommit, onCommitClick }: BranchGraphProps) {
  // Build layout with proper branching
  const layout = buildLayout(commits);

  const ROW_HEIGHT = 70;
  const COL_WIDTH = 60;
  const PADDING = 40;
  const NODE_RADIUS = 12;

  const maxCol = Math.max(...layout.map(n => n.x), 0);
  const svgWidth = Math.max((maxCol + 1) * COL_WIDTH + PADDING * 2, 200);
  const svgHeight = layout.length * ROW_HEIGHT + PADDING * 2;

  const getNodePos = (node: LayoutNode) => ({
    x: PADDING + node.x * COL_WIDTH + COL_WIDTH / 2,
    y: PADDING + node.y * ROW_HEIGHT + ROW_HEIGHT / 2,
  });

  const getColor = (col: number) => BRANCH_COLORS[col % BRANCH_COLORS.length];

  // Build edges
  const edges: { from: LayoutNode; to: LayoutNode; isMerge: boolean }[] = [];
  layout.forEach(node => {
    node.commit.parentIds.forEach(parentId => {
      const parent = layout.find(n => n.commit.id === parentId);
      if (parent) {
        edges.push({
          from: node,
          to: parent,
          isMerge: node.commit.isMerge || node.x !== parent.x
        });
      }
    });
  });

  return (
    <div className="overflow-auto">
      <div className="relative" style={{ minWidth: svgWidth, minHeight: svgHeight }}>
        <svg
          width={svgWidth}
          height={svgHeight}
          className="absolute top-0 left-0"
        >
          {/* Draw edges */}
          {edges.map(({ from, to, isMerge }, idx) => {
            const fromPos = getNodePos(from);
            const toPos = getNodePos(to);

            // Curved path for branches/merges
            if (from.x !== to.x) {
              const midY = (fromPos.y + toPos.y) / 2;
              return (
                <path
                  key={`edge-${idx}`}
                  d={`M ${fromPos.x} ${fromPos.y} 
                      C ${fromPos.x} ${midY}, ${toPos.x} ${midY}, ${toPos.x} ${toPos.y}`}
                  stroke={isMerge ? '#a855f7' : getColor(Math.min(from.x, to.x))}
                  strokeWidth="2"
                  fill="none"
                  strokeDasharray={isMerge ? "4,4" : undefined}
                />
              );
            }

            // Straight line for same branch
            return (
              <line
                key={`edge-${idx}`}
                x1={fromPos.x}
                y1={fromPos.y}
                x2={toPos.x}
                y2={toPos.y}
                stroke={getColor(from.x)}
                strokeWidth="2"
              />
            );
          })}

          {/* Draw nodes */}
          {layout.map((node) => {
            const pos = getNodePos(node);
            const isSelected = selectedCommit === node.commit.id;

            return (
              <g key={node.commit.id}>
                {/* Selection ring */}
                {isSelected && (
                  <circle
                    cx={pos.x}
                    cy={pos.y}
                    r={NODE_RADIUS + 6}
                    fill="none"
                    stroke="#3b82f6"
                    strokeWidth="2"
                  />
                )}
                {/* Merge indicator */}
                {node.commit.isMerge && (
                  <circle
                    cx={pos.x}
                    cy={pos.y}
                    r={NODE_RADIUS + 3}
                    fill="none"
                    stroke="#a855f7"
                    strokeWidth="2"
                  />
                )}
                {/* Node circle */}
                <circle
                  cx={pos.x}
                  cy={pos.y}
                  r={NODE_RADIUS}
                  fill={getColor(node.x)}
                  stroke="white"
                  strokeWidth="3"
                  className="cursor-pointer hover:opacity-80 transition-opacity"
                  onClick={() => onCommitClick?.(node.commit)}
                />
                {/* Root indicator */}
                {node.commit.isRoot && (
                  <circle
                    cx={pos.x}
                    cy={pos.y}
                    r={4}
                    fill="white"
                  />
                )}
              </g>
            );
          })}
        </svg>

        {/* Labels overlay */}
        <div className="absolute top-0 left-0" style={{ width: svgWidth, height: svgHeight }}>
          {layout.map((node) => {
            const pos = getNodePos(node);

            return (
              <div
                key={`label-${node.commit.id}`}
                className={cn(
                  "absolute flex items-center gap-2 cursor-pointer",
                  "hover:bg-slate-100/80 rounded px-2 py-1 transition-colors"
                )}
                style={{
                  left: pos.x + NODE_RADIUS + 8,
                  top: pos.y - 12,
                }}
                onClick={() => onCommitClick?.(node.commit)}
              >
                <span className="font-mono text-sm font-medium text-slate-900 whitespace-nowrap">
                  {node.commit.name}
                </span>
                {node.commit.isMerge && (
                  <span className="text-xs bg-purple-100 text-purple-700 px-1.5 py-0.5 rounded">
                    merge
                  </span>
                )}
                {node.commit.isRoot && (
                  <span className="text-xs bg-blue-100 text-blue-700 px-1.5 py-0.5 rounded">
                    root
                  </span>
                )}
                {node.commit.branch && !node.commit.isMerge && !node.commit.isRoot && (
                  <span className="text-xs bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded">
                    {node.commit.branch}
                  </span>
                )}
              </div>
            );
          })}
        </div>

        {commits.length === 0 && (
          <div className="flex items-center justify-center h-40 text-slate-500">
            No commits yet. Initialize a repository to get started.
          </div>
        )}
      </div>
    </div>
  );
}

// Build layout: assign x (column/lane) and y (row) to each commit
function buildLayout(commits: CommitNode[]): LayoutNode[] {
  if (commits.length === 0) return [];

  const layout: LayoutNode[] = [];
  const commitMap = new Map<string, CommitNode>();
  const childrenMap = new Map<string, string[]>();

  // Build maps
  commits.forEach(c => {
    commitMap.set(c.id, c);
    c.parentIds.forEach(pid => {
      const children = childrenMap.get(pid) || [];
      children.push(c.id);
      childrenMap.set(pid, children);
    });
  });

  const nodeColumns = new Map<string, number>();

  // Assign columns based on branch lanes
  let currentCol = 0;

  // Process in topological order (parents before children)
  // But display children first (newer at top)
  const sorted = topologicalSort(commits);

  sorted.forEach((commit, index) => {
    if (commit.parentIds.length === 0 || commit.isRoot) {
      // Root commit - main lane
      nodeColumns.set(commit.id, 0);
    } else if (commit.isMerge && commit.parentIds.length >= 2) {
      // Merge commit - goes to target branch lane (first parent)
      const firstParentCol = nodeColumns.get(commit.parentIds[0]) ?? 0;
      nodeColumns.set(commit.id, firstParentCol);
    } else {
      // Regular commit
      const parentId = commit.parentIds[0];
      const parentCol = nodeColumns.get(parentId);
      const siblings = childrenMap.get(parentId) || [];

      if (siblings.length > 1) {
        // Multiple children from same parent = branching
        const siblingIndex = siblings.indexOf(commit.id);
        if (siblingIndex === 0) {
          // First child stays on parent's lane
          nodeColumns.set(commit.id, parentCol ?? 0);
        } else {
          // Other children get new lanes
          currentCol++;
          nodeColumns.set(commit.id, currentCol);
        }
      } else {
        // Single child - stays on parent's lane
        nodeColumns.set(commit.id, parentCol ?? 0);
      }
    }
  });

  // Build layout array
  sorted.forEach((commit, index) => {
    layout.push({
      commit,
      x: nodeColumns.get(commit.id) ?? 0,
      y: index,
    });
  });

  return layout;
}

// Topological sort - children before parents (newer commits first)
function topologicalSort(commits: CommitNode[]): CommitNode[] {
  const result: CommitNode[] = [];
  const visited = new Set<string>();
  const visiting = new Set<string>();

  function visit(commit: CommitNode) {
    if (visited.has(commit.id)) return;
    if (visiting.has(commit.id)) return; // cycle detection

    visiting.add(commit.id);

    // Visit children first (they should appear before parents in display)
    commits.forEach(c => {
      if (c.parentIds.includes(commit.id)) {
        visit(c);
      }
    });

    visiting.delete(commit.id);
    visited.add(commit.id);
    result.push(commit);
  }

  // Start from roots
  commits.filter(c => c.parentIds.length === 0 || c.isRoot).forEach(visit);
  // Then any unvisited
  commits.forEach(c => {
    if (!visited.has(c.id)) visit(c);
  });

  return result;
}

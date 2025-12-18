# Git Simulation Platform - Frontend

React-based frontend for the Git Simulation Platform.

## Features

- 🔐 User authentication via X-Username header
- 👥 Team management (create teams, add members)
- 📁 Repository initialization and commit management
- 🌿 Visual branch/commit graph
- 🔀 Pull Request workflow (create, approve, reject)
- 📥 Code checkout (download as ZIP)

## Tech Stack

- React 18 + TypeScript
- Vite (build tool)
- Tailwind CSS 4
- Lucide React (icons)
- shadcn/ui style components

## Development

### Local Development

```bash
# Install dependencies
npm install

# Start dev server
npm run dev
```

The app will be available at http://localhost:3000

### With Docker

```bash
# From project root
docker-compose up frontend
```

## API Integration

The frontend proxies all `/api/*` requests to the User Gateway Service (port 8082).

### Key API Endpoints Used

- `GET /api/me` - Get current user profile
- `POST /api/team/create` - Create a team
- `GET /api/team/get` - Get team info
- `POST /api/repo/init` - Initialize repository
- `POST /api/repo/push` - Push commit
- `GET /api/repo/checkout` - Checkout code
- `POST /api/pr/create` - Create PR
- `GET /api/pr/my` - Get user's PRs
- `GET /api/pr/reviews` - Get PRs for review
- `POST /api/pr/approve` - Approve PR
- `POST /api/pr/reject` - Reject PR

## Project Structure

```
src/
├── api/
│   └── client.ts       # API client functions
├── components/
│   ├── ui/             # shadcn-style UI components
│   │   ├── badge.tsx
│   │   ├── button.tsx
│   │   ├── card.tsx
│   │   └── input.tsx
│   ├── BranchGraph.tsx # Commit/branch visualization
│   └── PRList.tsx      # Pull Request list component
├── lib/
│   └── utils.ts        # Utility functions (cn)
├── App.tsx             # Main application
├── main.tsx            # Entry point
└── index.css           # Global styles + Tailwind
```

## Usage

1. Enter a username to "login"
2. Create a team with members
3. Initialize a repository with a ZIP file
4. Push commits to the repository
5. Create Pull Requests
6. Review and approve/reject PRs


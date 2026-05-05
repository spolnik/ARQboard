import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';
import App, {
  cardAssigneeText,
  cardMatchesFilters,
  cardMatchesSearch,
  columnAccent,
  defaultAssigneeId,
  dueStatus,
  epicStatusClass,
  epicStatusLabel,
  epicWindow,
  addSprintToDashboard,
  assignCardInDashboard,
  completeSprintInDashboard,
  hasActiveBoardFilters,
  normalizeCard,
  normalizePlanningDashboard,
  normalizeRoadmapDashboard,
  resolveDragMoveTarget,
  resolveMoveTarget,
  resolvePlanningMoveTarget,
  resolveRoadmapMoveTarget,
  roadmapRiskClass,
  roadmapRiskLabel,
  sortCards,
  sprintWeekRange,
  sortSprintPlans,
  sprintSortKey,
  sprintStatusDisplay,
  startSprintInDashboard,
  sprintWindow,
  canManageTeam,
  canReadTeam,
  canWriteTeam,
  teamRoleForUser,
  addCardToBoard,
  assignCardInRoadmap,
  findPlanningCard,
  replaceCardInBoard,
  selectedCardIdForBoard,
  upsertDependencyInRoadmap,
  upsertEpicInRoadmap,
  upsertWikiPageInBoard,
} from './App';

const userFixture = {
  id: 'user-1',
  email: 'admin@example.com',
  displayName: 'Admin',
  isAdmin: true,
};

const developerUserFixture = {
  id: 'user-2',
  email: 'dev@example.com',
  displayName: 'Dev',
  isAdmin: false,
};

const viewerUserFixture = {
  id: 'user-3',
  email: 'viewer@example.com',
  displayName: 'Viewer',
  isAdmin: false,
};

const boardFixture = {
  id: 'board-1',
  workspaceId: 'workspace-1',
  teamId: 'team-platform',
  name: 'Platform Board',
  slug: 'platform',
  members: [
    { id: 'member-1', workspaceId: 'workspace-1', userId: 'user-1', email: 'admin@example.com', displayName: 'Admin', role: 'owner', isAdmin: true },
    { id: 'member-2', workspaceId: 'workspace-1', userId: 'user-2', email: 'dev@example.com', displayName: 'Dev', role: 'member', isAdmin: false },
  ],
  labels: [
    { id: 'label-backend', workspaceId: 'workspace-1', name: 'Backend', color: '#0f766e' },
    { id: 'label-risk', workspaceId: 'workspace-1', name: 'Risk', color: '#be123c' },
  ],
  columns: [
    {
      id: 'column-planned',
      title: 'Planned',
      position: 0,
      cards: [
        {
          id: 'card-1',
          columnId: 'column-planned',
          boardId: 'board-1',
          boardName: 'Platform Board',
          title: 'Wire auth session cookie flow',
          owner: 'Admin',
          assigneeId: 'user-1',
          assigneeName: 'Admin',
          assigneeEmail: 'admin@example.com',
          labels: [{ id: 'label-backend', workspaceId: 'workspace-1', name: 'Backend', color: '#0f766e' }],
          priority: 'High',
          due: '2026-04-30',
          description: 'Map the session cookie lifecycle.',
          position: 0,
        },
      ],
    },
    {
      id: 'column-progress',
      title: 'In progress',
      position: 1,
      cards: [
        {
          id: 'card-2',
          columnId: 'column-progress',
          boardId: 'board-1',
          boardName: 'Platform Board',
          title: 'Ready for review API shape',
          owner: 'Dev',
          assigneeId: 'user-2',
          assigneeName: 'Dev',
          assigneeEmail: 'dev@example.com',
          labels: [{ id: 'label-risk', workspaceId: 'workspace-1', name: 'Risk', color: '#be123c' }],
          priority: 'Urgent',
          due: '2026-05-01',
          description: 'Lock the first JSON contracts.',
          position: 0,
        },
      ],
    },
    {
      id: 'column-review',
      title: 'Ready for review',
      position: 2,
      cards: [
        {
          id: 'card-3',
          columnId: 'column-review',
          boardId: 'board-1',
          boardName: 'Platform Board',
          title: 'Deployment checklist',
          owner: '',
          assigneeId: '',
          assigneeName: '',
          assigneeEmail: '',
          labels: [],
          priority: 'Normal',
          due: '2026-05-03',
          description: 'Document deployment checks.',
          position: 0,
        },
      ],
    },
    {
      id: 'column-done',
      title: 'Done',
      position: 3,
      cards: [],
    },
  ],
  wikiPages: [
    { id: 'wiki-1', title: 'Deployment checklist', slug: 'deployment-checklist', bodyMarkdown: '# Deploy\n\n- Build image' },
    { id: 'wiki-2', title: 'Onboarding notes', slug: 'onboarding-notes', bodyMarkdown: 'Welcome aboard.' },
    {
      id: 'wiki-3',
      title: 'Engineering/Runbooks/Deploy',
      slug: 'engineering-runbooks-deploy',
      bodyMarkdown: '# Deploy\n\nUse the release checklist.',
    },
  ],
};

const boardSummaries = [
  {
    id: 'board-1',
    workspaceId: 'workspace-1',
    teamId: 'team-platform',
    name: 'Platform Board',
    slug: 'platform',
    columnCount: 4,
    cardCount: 3,
  },
  {
    id: 'board-release',
    workspaceId: 'workspace-1',
    teamId: 'team-release',
    name: 'Release Train',
    slug: 'release-train',
    columnCount: 4,
    cardCount: 0,
  },
];

const releaseBoardFixture = {
  id: 'board-release',
  workspaceId: 'workspace-1',
  teamId: 'team-platform',
  name: 'Release Train',
  slug: 'release-train',
  columns: [
    { id: 'release-planned', title: 'Planned', position: 0, cards: [] },
    { id: 'release-progress', title: 'In progress', position: 1, cards: [] },
    { id: 'release-review', title: 'Ready for review', position: 2, cards: [] },
    { id: 'release-done', title: 'Done', position: 3, cards: [] },
  ],
  wikiPages: [],
};

const createdBoardFixture = {
  id: 'board-security',
  workspaceId: 'workspace-1',
  teamId: 'team-platform',
  name: 'Security Backlog',
  slug: 'security-backlog',
  columns: [
    { id: 'security-planned', title: 'Planned', position: 0, cards: [] },
    { id: 'security-progress', title: 'In progress', position: 1, cards: [] },
    { id: 'security-review', title: 'Ready for review', position: 2, cards: [] },
    { id: 'security-done', title: 'Done', position: 3, cards: [] },
  ],
  wikiPages: [],
};

const boardWithBlockedColumn = {
  ...boardFixture,
  columns: [...boardFixture.columns, { id: 'column-blocked', title: 'Blocked', position: 4, cards: [] }],
};

const boardWithRenamedColumn = {
  ...boardFixture,
  columns: boardFixture.columns.map((column) => (column.id === 'column-planned' ? { ...column, title: 'Backlog' } : column)),
};

const createdCard = {
  id: 'card-new',
  columnId: 'column-planned',
  boardId: 'board-1',
  boardName: 'Platform Board',
  title: 'Run local smoke test',
  owner: 'Admin',
  assigneeId: 'user-1',
  assigneeName: 'Admin',
  assigneeEmail: 'admin@example.com',
  labels: [],
  priority: 'Normal',
  due: '2026-05-08',
  description: 'New card created locally.',
  position: 1,
};

const movedBoardFixture = {
  ...boardFixture,
  columns: [
    { ...boardFixture.columns[0], cards: [] },
    {
      ...boardFixture.columns[1],
      cards: [
        boardFixture.columns[1].cards[0],
        { ...boardFixture.columns[0].cards[0], columnId: 'column-progress', position: 1 },
      ],
    },
    boardFixture.columns[2],
    boardFixture.columns[3],
  ],
};

const updatedCard = {
  ...boardFixture.columns[0].cards[0],
  title: 'Wire production auth flow',
  description: 'Document cookie boundaries and refresh behavior.',
  owner: 'Admin',
  priority: 'Urgent',
  due: '2026-05-09',
};

const cardDetail = {
  card: boardFixture.columns[0].cards[0],
  comments: [],
  activity: [{ id: 'event-1', cardId: 'card-1', eventType: 'card.moved', summary: 'Card moved', createdAt: '2026-04-29T10:00:00Z' }],
};

const commentedDetail = {
  card: updatedCard,
  comments: [{ id: 'comment-1', cardId: 'card-1', body: 'Add rollback note before release.', createdAt: '2026-04-29T10:05:00Z' }],
  activity: [
    { id: 'event-2', cardId: 'card-1', eventType: 'card.commented', summary: 'Comment added', createdAt: '2026-04-29T10:05:00Z' },
    { id: 'event-1', cardId: 'card-1', eventType: 'card.updated', summary: 'Card updated', createdAt: '2026-04-29T10:00:00Z' },
  ],
};

const updatedWikiPage = {
  id: 'wiki-1',
  title: 'Deployment checklist',
  slug: 'deployment-checklist',
  bodyMarkdown: '# Deploy safely\n\n- Run migrations',
};

const createdWikiPage = {
  id: 'wiki-new',
  title: 'Release runbook',
  slug: 'release-runbook',
  bodyMarkdown: '# Release runbook\n\nShip carefully.',
};

const workspaceMembers = [
  { id: 'member-1', workspaceId: 'workspace-1', userId: 'user-1', email: 'admin@example.com', displayName: 'Admin', role: 'owner', isAdmin: true },
  { id: 'member-2', workspaceId: 'workspace-1', userId: 'user-2', email: 'dev@example.com', displayName: 'Dev', role: 'member', isAdmin: false },
];

const teamsFixture = [
  {
    id: 'team-platform',
    workspaceId: 'workspace-1',
    name: 'Platform Engineering',
    slug: 'platform-engineering',
    members: [
      { id: 'team-member-1', teamId: 'team-platform', userId: 'user-1', email: 'admin@example.com', displayName: 'Admin', role: 'owner', isAdmin: true },
      { id: 'team-member-2', teamId: 'team-platform', userId: 'user-2', email: 'dev@example.com', displayName: 'Dev', role: 'member', isAdmin: false },
    ],
  },
];

const teamAdminFixture = {
  ...teamsFixture[0],
  members: [
    { id: 'team-member-2', teamId: 'team-platform', userId: 'user-2', email: 'dev@example.com', displayName: 'Dev', role: 'admin', isAdmin: false },
  ],
};

const teamMemberFixture = {
  ...teamsFixture[0],
  members: [
    { id: 'team-member-2', teamId: 'team-platform', userId: 'user-2', email: 'dev@example.com', displayName: 'Dev', role: 'member', isAdmin: false },
  ],
};

const teamViewerFixture = {
  ...teamsFixture[0],
  members: [
    { id: 'team-member-3', teamId: 'team-platform', userId: 'user-3', email: 'viewer@example.com', displayName: 'Viewer', role: 'viewer', isAdmin: false },
  ],
};

const createdTeamFixture = {
  id: 'team-design',
  workspaceId: 'workspace-1',
  name: 'Design Systems',
  slug: 'design-systems',
  members: [],
};

const createdWorkspaceMember = {
  id: 'member-3',
  workspaceId: 'workspace-1',
  userId: 'user-3',
  email: 'qa@example.com',
  displayName: 'QA',
  role: 'viewer',
  isAdmin: false,
};

const updatedWorkspaceMember = {
  ...workspaceMembers[1],
  role: 'admin',
};

const sprintFixture = {
  id: 'sprint-1',
  workspaceId: 'workspace-1',
  teamId: 'team-platform',
  boardId: 'board-1',
  name: 'Sprint 2026-W19',
  goal: 'Ship planning foundations',
  status: 'planned',
  startsOn: '2026-05-04',
  endsOn: '2026-05-10',
};

const activeSprintFixture = {
  ...sprintFixture,
  status: 'active',
  startedAt: '2026-05-04T08:00:00Z',
};

const completedSprintFixture = {
  ...activeSprintFixture,
  status: 'completed',
  completedAt: '2026-05-10T16:00:00Z',
};

const planningDashboardFixture = {
  boardId: 'board-1',
  teamId: 'team-platform',
  teamName: 'Platform Engineering',
  boards: boardSummaries,
  backlog: [boardFixture.columns[0].cards[0]],
  plannedSprints: [],
  completedSprints: [],
};

const roadmapEpicFixture = {
  id: 'epic-1',
  workspaceId: 'workspace-1',
  teamId: 'team-platform',
  title: 'Identity foundations',
  slug: 'identity-foundations',
  description: 'Login, session, and access boundaries.',
  status: 'active' as const,
  startsOn: '2026-05-04',
  targetOn: '2026-05-29',
};

const createdEpicFixture = {
  id: 'epic-2',
  workspaceId: 'workspace-1',
  teamId: 'team-platform',
  title: 'Roadmap hardening',
  slug: 'roadmap-hardening',
  description: 'Make roadmap planning usable for the team.',
  status: 'planned' as const,
  startsOn: '2026-06-01',
  targetOn: '2026-06-19',
};

const createdDependencyFixture = {
  id: 'dependency-created',
  blockedCardId: 'card-1',
  blockedTitle: 'Wire auth session cookie flow',
  blockerCardId: 'card-2',
  blockerTitle: 'Ready for review API shape',
  relationType: 'blocks',
};

const roadmapDashboardFixture = {
  teamId: 'team-platform',
  teamName: 'Platform Engineering',
  epics: [
    {
      epic: roadmapEpicFixture,
      cards: [
        {
          card: { ...boardFixture.columns[1].cards[0], epicId: 'epic-1' },
          columnTitle: 'In progress',
          blockedBy: [],
          blocking: [],
        },
      ],
      totalCards: 1,
      completedCards: 0,
      blockedCards: 0,
      progress: 0,
      risk: 'on_track',
    },
  ],
  unassignedCards: [
    {
      card: { ...boardFixture.columns[0].cards[0], epicId: '' },
      columnTitle: 'Planned',
      blockedBy: [],
      blocking: [],
    },
  ],
  dependencies: [],
};

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

async function clickPrimaryNavButton(name: RegExp) {
  const nav = await screen.findByRole('navigation', { name: /primary navigation/i });
  fireEvent.click(within(nav).getByRole('button', { name }));
}

function dateOffset(days: number) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function stubRoleWorkspace(
  user: typeof userFixture,
  team: (typeof teamsFixture)[number],
  planningResponse: unknown = planningDashboardFixture,
  roadmapResponse: unknown = roadmapDashboardFixture,
) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/me') {
        return jsonResponse(user);
      }
      if (url === '/api/boards') {
        return jsonResponse(boardSummaries);
      }
      if (url === '/api/teams') {
        return jsonResponse([team]);
      }
      if (url === '/api/boards/board-1') {
        return jsonResponse(boardFixture);
      }
      if (url === '/api/planning?teamId=team-platform') {
        return jsonResponse(planningResponse);
      }
      if (url === '/api/roadmap?teamId=team-platform') {
        return jsonResponse(roadmapResponse);
      }

      throw new Error(`Unexpected fetch GET ${url}`);
    }),
  );
}

describe('App', () => {
  beforeEach(() => {
    let planningResponse: unknown = planningDashboardFixture;
    let roadmapResponse: unknown = roadmapDashboardFixture;
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/auth/login' && init?.method === 'POST') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/auth/logout' && init?.method === 'POST') {
          return new Response(null, { status: 204 });
        }
        if (url === '/api/boards' && init?.method === 'POST') {
          return jsonResponse(createdBoardFixture, 201);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams' && init?.method === 'POST') {
          return jsonResponse(createdTeamFixture, 201);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/teams/team-platform/members' && init?.method === 'POST') {
          return jsonResponse({
            ...teamsFixture[0],
            members: [...teamsFixture[0].members, { id: 'team-member-qa', teamId: 'team-platform', userId: 'user-3', email: 'qa@example.com', displayName: 'QA', role: 'viewer', isAdmin: false }],
          });
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/boards/board-release') {
          return jsonResponse(releaseBoardFixture);
        }
        if (url === '/api/boards/board-1/columns' && init?.method === 'POST') {
          return jsonResponse(boardWithBlockedColumn, 201);
        }
        if (url === '/api/columns/column-planned' && init?.method === 'PATCH') {
          return jsonResponse(boardWithRenamedColumn);
        }
        if (url === '/api/cards' && init?.method === 'POST') {
          return jsonResponse(createdCard, 201);
        }
        if (url === '/api/cards/card-1/move' && init?.method === 'PATCH') {
          return jsonResponse(movedBoardFixture);
        }
        if (url === '/api/cards/card-1' && !init?.method) {
          return jsonResponse(cardDetail);
        }
        if (url === '/api/cards/card-2' && !init?.method) {
          return jsonResponse({ card: boardFixture.columns[1].cards[0], comments: [], activity: [] });
        }
        if (url === '/api/cards/card-3' && !init?.method) {
          return jsonResponse({ card: boardFixture.columns[2].cards[0], comments: [], activity: [] });
        }
        if (url === '/api/cards/card-1' && init?.method === 'PATCH') {
          return jsonResponse(updatedCard);
        }
        if (url === '/api/cards/card-1/comments' && init?.method === 'POST') {
          return jsonResponse(commentedDetail, 201);
        }
        if (url === '/api/wiki') {
          if (init?.method === 'POST') {
            return jsonResponse(createdWikiPage, 201);
          }
          return jsonResponse(boardFixture.wikiPages);
        }
        if (url === '/api/wiki/wiki-1' && !init?.method) {
          return jsonResponse(boardFixture.wikiPages[0]);
        }
        if (url === '/api/wiki/wiki-3' && !init?.method) {
          return jsonResponse(boardFixture.wikiPages[2]);
        }
        if (url === '/api/wiki/wiki-1' && init?.method === 'PATCH') {
          return jsonResponse(updatedWikiPage);
        }
        if (url === '/api/members') {
          if (init?.method === 'POST') {
            return jsonResponse(createdWorkspaceMember, 201);
          }
          return jsonResponse(workspaceMembers);
        }
        if (url === '/api/members/member-2' && init?.method === 'PATCH') {
          return jsonResponse(updatedWorkspaceMember);
        }
        if (url === '/api/planning?teamId=team-platform') {
          return jsonResponse(planningResponse);
        }
        if (url === '/api/roadmap?teamId=team-platform') {
          return jsonResponse(roadmapResponse);
        }
        if (url === '/api/epics' && init?.method === 'POST') {
          roadmapResponse = {
            ...(roadmapResponse as typeof roadmapDashboardFixture),
            epics: [...(roadmapResponse as typeof roadmapDashboardFixture).epics, { epic: createdEpicFixture, cards: [], totalCards: 0, completedCards: 0, blockedCards: 0, progress: 0, risk: 'on_track' }],
          };
          return jsonResponse(createdEpicFixture, 201);
        }
        if (url === '/api/cards/card-1/epic' && init?.method === 'PATCH') {
          return jsonResponse({ ...boardFixture.columns[0].cards[0], epicId: 'epic-1' });
        }
        if (url === '/api/cards/card-1/dependencies' && init?.method === 'POST') {
          return jsonResponse(createdDependencyFixture, 201);
        }
        if (url === '/api/card-dependencies/dependency-created' && init?.method === 'DELETE') {
          return new Response(null, { status: 204 });
        }
        if (url === '/api/sprints' && init?.method === 'POST') {
          return jsonResponse(activeSprintFixture, 201);
        }
        if (url === '/api/cards/card-1/sprint' && init?.method === 'PATCH') {
          return jsonResponse({ ...boardFixture.columns[0].cards[0], sprintId: 'sprint-1' });
        }
        if (url === '/api/sprints/sprint-1/start' && init?.method === 'POST') {
          return jsonResponse(activeSprintFixture);
        }
        if (url === '/api/sprints/sprint-1/complete' && init?.method === 'POST') {
          planningResponse = {
            boardId: 'board-1',
            teamId: 'team-platform',
            teamName: 'Platform Engineering',
            boards: boardSummaries,
            backlog: [{ ...boardFixture.columns[0].cards[0], sprintId: '' }],
            activeSprint: null,
            plannedSprints: [],
            completedSprints: [{ sprint: completedSprintFixture, cards: [] }],
          };
          return jsonResponse(completedSprintFixture);
        }

        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows the loading screen while the session check is pending', async () => {
    let resolveSession: (response: Response) => void = () => {};
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/me') {
        return new Promise<Response>((resolve) => {
          resolveSession = resolve;
        });
      }
      if (url === '/api/boards') {
        return jsonResponse(boardSummaries);
      }
      if (url === '/api/teams') {
        return jsonResponse(teamsFixture);
      }
      if (url === '/api/boards/board-1') {
        return jsonResponse(boardFixture);
      }
      if (url === '/api/cards/card-2') {
        return jsonResponse({ card: boardFixture.columns[1].cards[0], comments: [], activity: [] });
      }
      throw new Error(`Unexpected fetch ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<App />);

    expect(screen.getByText(/loading workspace/i)).toBeInTheDocument();
    resolveSession(jsonResponse(userFixture));
    expect(await screen.findByRole('heading', { name: /platform board/i }, { timeout: 5000 })).toBeInTheDocument();
  });

  it('loads the board and wiki workspace surface from the API', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    const taskCard = screen.getByRole('button', { name: /view card wire auth session cookie flow/i });
    expect(taskCard).toHaveClass('cursor-grab');
    expect(within(taskCard).getByText('Admin')).toBeInTheDocument();
    expect(within(taskCard).queryByText('MS')).not.toBeInTheDocument();
    expect(screen.getByLabelText(/priority high/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/due 2026-04-30 overdue/i)).toHaveClass('border-rose-200');
    expect(screen.queryByRole('button', { name: /drag wire auth session cookie flow/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /move wire auth session cookie flow to in progress/i })).not.toBeInTheDocument();
    expect(screen.getByText('Ready for review')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: /done column/i })).toBeInTheDocument();
    expect(screen.getAllByText('Deployment checklist').length).toBeGreaterThan(0);
    expect(fetch).toHaveBeenCalledWith('/api/boards');
    expect(fetch).toHaveBeenCalledWith('/api/boards/board-1');
  });

  it('filters the board by assignee, label, priority, and due status', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/filter by assignee/i), { target: { value: 'user-2' } });
    expect(screen.getByText('Ready for review API shape')).toBeInTheDocument();
    expect(screen.queryByText('Wire auth session cookie flow')).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/filter by label/i), { target: { value: 'label-risk' } });
    expect(screen.getByText('Ready for review API shape')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/filter by priority/i), { target: { value: 'high' } });
    expect(screen.queryByText('Ready for review API shape')).not.toBeInTheDocument();
    expect(screen.getAllByText('No matches').length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText(/filter by priority/i), { target: { value: 'all' } });
    fireEvent.change(screen.getByLabelText(/filter by due status/i), { target: { value: 'overdue' } });
    expect(screen.getByText('Ready for review API shape')).toBeInTheDocument();
  });

  it('marks cards with missing due dates without breaking the board', async () => {
    const boardWithMissingDue = {
      ...boardFixture,
      columns: boardFixture.columns.map((column) => ({
        ...column,
        cards: column.cards.map((card) => (card.id === 'card-1' ? { ...card, due: '' } : card)),
      })),
    };

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardWithMissingDue);
        }
        if (url === '/api/cards/card-1' && !init?.method) {
          return jsonResponse({ card: boardWithMissingDue.columns[0].cards[0], comments: [], activity: [] });
        }

        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );

    render(<App />);

    expect(await screen.findByLabelText(/date missing/i)).toBeInTheDocument();
  });

  it('shows a helpful error when the board cannot load', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        return jsonResponse({ error: { code: 'not_ready' } }, 500);
      }),
    );

    render(<App />);

    expect(await screen.findByText(/could not load the board/i)).toBeInTheDocument();
  });

  it('shows a helpful error when sprint planning cannot load', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/planning?teamId=team-platform') {
          return jsonResponse({ error: { code: 'not_ready' } }, 500);
        }
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);
    await clickPrimaryNavButton(/planning/i);

    expect(await screen.findByText(/could not load planning dashboard/i)).toBeInTheDocument();
    expect(screen.getByText(/backlog is clear/i)).toBeInTheDocument();
  });

  it('renders low-priority cards distinctly', async () => {
    const lowPriorityBoard = {
      ...boardFixture,
      columns: boardFixture.columns.map((column) => ({
        ...column,
        cards: column.cards.map((card) => (card.id === 'card-1' ? { ...card, priority: 'Low' } : card)),
      })),
    };

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(lowPriorityBoard);
        }
        if (url === '/api/cards/card-1') {
          return jsonResponse({ card: lowPriorityBoard.columns[0].cards[0], comments: [], activity: [] });
        }
        return jsonResponse({ card: lowPriorityBoard.columns[1].cards[0], comments: [], activity: [] });
      }),
    );

    render(<App />);

    expect(await screen.findByLabelText(/priority low/i)).toBeInTheDocument();
  });

  it('shows login before loading the workspace and signs in with admin credentials', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/me') {
        return jsonResponse({ error: { code: 'unauthenticated' } }, 401);
      }
      if (url === '/api/auth/login' && init?.method === 'POST') {
        return jsonResponse(userFixture);
      }
      if (url === '/api/boards') {
        return jsonResponse(boardSummaries);
      }
      if (url === '/api/teams') {
        return jsonResponse(teamsFixture);
      }
      if (url === '/api/boards/board-1') {
        return jsonResponse(boardFixture);
      }
      if (url === '/api/cards/card-2') {
        return jsonResponse({ card: boardFixture.columns[1].cards[0], comments: [], activity: [] });
      }
      throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<App />);

    expect(await screen.findByRole('button', { name: /sign in/i })).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith('/api/boards');

    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: 'admin@example.com' },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: 'correct horse battery staple' },
    });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/auth/login',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            email: 'admin@example.com',
            password: 'correct horse battery staple',
          }),
        }),
      ),
    );
    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
  });

  it('shows a login error when credentials are rejected', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse({ error: { code: 'unauthenticated' } }, 401);
        }
        if (url === '/api/auth/login' && init?.method === 'POST') {
          return jsonResponse({ error: { code: 'unauthenticated' } }, 401);
        }
        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );

    render(<App />);

    fireEvent.change(await screen.findByLabelText(/email/i), {
      target: { value: 'admin@example.com' },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: 'wrong password' },
    });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    expect(await screen.findByText(/invalid email or password/i)).toBeInTheDocument();
  });

  it('requires email and password before calling the login API', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/me') {
        return jsonResponse({ error: { code: 'unauthenticated' } }, 401);
      }
      throw new Error(`Unexpected fetch ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /sign in/i }));

    expect(await screen.findByText(/email and password are required/i)).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith('/api/auth/login', expect.anything());
  });

  it('shows a session check error when the current user request fails', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('network unavailable');
      }),
    );

    render(<App />);

    expect(await screen.findByText(/could not check your session/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('signs out and returns to the login form', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /sign out/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/auth/logout',
        expect.objectContaining({
          method: 'POST',
        }),
      ),
    );
    expect(await screen.findByRole('button', { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /platform board/i })).not.toBeInTheDocument();
  });

  it('keeps card detail hidden until a card is selected and clears it from empty board space', async () => {
    const fallbackBoard = {
      ...boardFixture,
      columns: boardFixture.columns.map((column) => ({
        ...column,
        cards: column.cards.map((card) => (card.id === 'card-2' ? { ...card, title: 'Different card' } : card)),
      })),
    };

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(fallbackBoard);
        }
        if (url === '/api/cards/card-1') {
          return jsonResponse({ card: fallbackBoard.columns[0].cards[0], comments: [], activity: [] });
        }
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.queryByRole('complementary', { name: /card detail/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /view card wire auth session cookie flow/i }));
    const detail = await screen.findByRole('complementary', { name: /card detail/i });
    expect(within(detail).getByRole('heading', { name: /wire auth session cookie flow/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('region', { name: /kanban board/i }));
    expect(screen.queryByRole('complementary', { name: /card detail/i })).not.toBeInTheDocument();
  });

  it('shows a create-card error when the board has no columns', async () => {
    const emptyBoard = { ...boardFixture, columns: [], wikiPages: [] };
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(emptyBoard);
        }
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /new card/i }));
    fireEvent.change(screen.getByLabelText(/card title/i), {
      target: { value: 'Cannot place this yet' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create card/i }));

    expect(await screen.findByText(/this board has no columns/i)).toBeInTheDocument();
  });

  it('uses the selected team as the board context', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.queryByLabelText('Board')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Team')).toHaveValue('team-platform');
    expect(screen.getByText(/team board/i)).toHaveTextContent('Platform Board');
  });

  it('creates teams and assigns workspace members to a team', async () => {
    render(<App />);

    await clickPrimaryNavButton(/settings/i);
    fireEvent.change(await screen.findByLabelText(/team name/i), {
      target: { value: 'Design Systems' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create team/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/teams',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ name: 'Design Systems' }),
        }),
      ),
    );
    expect(await screen.findByText(/team created/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^platform engineering/i }));
    fireEvent.change(screen.getByLabelText(/team member/i), { target: { value: 'user-2' } });
    fireEvent.change(screen.getByLabelText(/team role/i), { target: { value: 'admin' } });
    fireEvent.click(screen.getByRole('button', { name: /assign/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/teams/team-platform/members',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ userId: 'user-2', role: 'admin' }),
        }),
      ),
    );
    expect(await screen.findByText(/member assigned to team/i)).toBeInTheDocument();
  });

  it('adds board columns but keeps existing column names fixed', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add column/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /rename planned/i })).not.toBeInTheDocument();

    await clickPrimaryNavButton(/settings/i);
    fireEvent.click(await screen.findByRole('button', { name: /add column/i }));
    expect(screen.getByRole('dialog', { name: /add column/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByRole('dialog', { name: /add column/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /add column/i }));
    fireEvent.change(screen.getByLabelText(/column title/i), {
      target: { value: 'Blocked' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^add column$/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/boards/board-1/columns',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ title: 'Blocked' }),
        }),
      ),
    );
    expect(screen.getByText('Blocked')).toBeInTheDocument();

    expect(screen.queryByRole('button', { name: /rename planned/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: /rename column/i })).not.toBeInTheDocument();
  });

  it('keeps the workspace usable when no boards exist yet', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse([]);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);

    expect(await screen.findByRole('heading', { name: /loading board/i })).toBeInTheDocument();
    expect(screen.getByText(/team board: no board/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /new card/i })).toBeDisabled();
    expect(screen.queryByRole('button', { name: /open add column/i })).not.toBeInTheDocument();
    await clickPrimaryNavButton(/settings/i);
    expect(screen.getByRole('button', { name: /add column/i })).toBeDisabled();
  });

  it('returns to login when the board list request is unauthenticated', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse({ error: { code: 'unauthenticated' } }, 401);
        }
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);

    expect(await screen.findByRole('button', { name: /sign in/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /platform board/i })).not.toBeInTheDocument();
  });

  it('shows errors when board and column management requests fail', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/me') {
        return jsonResponse(userFixture);
      }
      if (url === '/api/boards') {
        return jsonResponse(boardSummaries);
      }
      if (url === '/api/teams') {
        return jsonResponse(teamsFixture);
      }
      if (url === '/api/boards/board-1') {
        return jsonResponse(boardFixture);
      }
      if (url === '/api/cards/card-2') {
        return jsonResponse({ card: boardFixture.columns[1].cards[0], comments: [], activity: [] });
      }
      if (url === '/api/boards/board-1/columns' && init?.method === 'POST') {
        return jsonResponse({ error: { code: 'unavailable' } }, 500);
      }
      throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<App />);

    await clickPrimaryNavButton(/settings/i);
    fireEvent.click(screen.getByRole('button', { name: /add column/i }));
    fireEvent.change(screen.getByLabelText(/column title/i), {
      target: { value: 'Blocked' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^add column$/i }));
    expect(await screen.findByText(/could not add the column/i)).toBeInTheDocument();
  });

  it('shows errors when card management requests fail', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/cards' && init?.method === 'POST') {
          return jsonResponse({ error: { code: 'unavailable' } }, 500);
        }
        if (url === '/api/cards/card-2' && !init?.method) {
          return jsonResponse({ card: boardFixture.columns[1].cards[0], comments: [], activity: [] });
        }
        if (url === '/api/cards/card-2/move' && init?.method === 'PATCH') {
          return jsonResponse({ error: { code: 'unavailable' } }, 500);
        }
        if (url === '/api/cards/card-1' && !init?.method) {
          return jsonResponse(cardDetail);
        }
        if (url === '/api/cards/card-1' && init?.method === 'PATCH') {
          return jsonResponse({ error: { code: 'unavailable' } }, 500);
        }
        if (url === '/api/cards/card-1/comments' && init?.method === 'POST') {
          return jsonResponse({ error: { code: 'unavailable' } }, 500);
        }
        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );

    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /new card/i }));
    fireEvent.change(screen.getByLabelText(/card title/i), {
      target: { value: 'Cannot save' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create card/i }));
    expect(await screen.findByText(/could not create the card/i)).toBeInTheDocument();
    fireEvent.click(within(screen.getByRole('dialog', { name: /create card/i })).getByRole('button', { name: /cancel/i }));

    fireEvent.click(screen.getByRole('button', { name: /view card wire auth session cookie flow/i }));
    fireEvent.click(await screen.findByRole('button', { name: /edit card/i }));
    const detail = screen.getByRole('complementary', { name: /card detail/i });
    fireEvent.change(within(detail).getByLabelText(/card title/i), {
      target: { value: 'Cannot update' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save card/i }));
    expect(await screen.findByText(/could not update the card/i)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/new comment/i), {
      target: { value: 'This will fail.' },
    });
    fireEvent.click(screen.getByRole('button', { name: /add comment/i }));
    expect(await screen.findByText(/could not add the comment/i)).toBeInTheDocument();
  });

  it('shows errors when wiki page requests fail', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/cards/card-2') {
          return jsonResponse({ card: boardFixture.columns[1].cards[0], comments: [], activity: [] });
        }
        if (url === '/api/wiki/wiki-1') {
          return jsonResponse({ error: { code: 'unavailable' } }, 500);
        }
        if (url === '/api/wiki' && init?.method === 'POST') {
          return jsonResponse({ error: { code: 'unavailable' } }, 500);
        }
        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );

    render(<App />);

    await clickPrimaryNavButton(/wiki/i);
    fireEvent.click(await screen.findByRole('button', { name: /deployment checklist/i }));
    expect(await screen.findByText(/could not load the wiki page/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /new wiki page/i }));
    fireEvent.change(screen.getByLabelText(/wiki title/i), {
      target: { value: 'Failed runbook' },
    });
    fireEvent.change(screen.getByLabelText(/markdown body/i), {
      target: { value: '# Failed runbook' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create page/i }));
    expect(await screen.findByText(/could not save the wiki page/i)).toBeInTheDocument();
  });

  it('updates the card detail panel when a card is selected', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /view card wire auth session cookie flow/i }));

    const detail = screen.getByRole('complementary', { name: /card detail/i });
    expect(within(detail).getByRole('heading', { name: /wire auth session cookie flow/i })).toBeInTheDocument();
    expect(within(detail).getByText(/Assignee Admin/)).toBeInTheDocument();
    expect(within(detail).getByText('Backend')).toBeInTheDocument();
    expect(within(detail).getByText(/Priority High/)).toBeInTheDocument();
  });

  it('edits selected card details through the API', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /view card wire auth session cookie flow/i }));
    fireEvent.click(await screen.findByRole('button', { name: /edit card/i }));
    const detail = screen.getByRole('complementary', { name: /card detail/i });
    fireEvent.change(within(detail).getByLabelText(/card title/i), {
      target: { value: 'Wire production auth flow' },
    });
    fireEvent.change(within(detail).getByLabelText(/description/i), {
      target: { value: 'Document cookie boundaries and refresh behavior.' },
    });
    fireEvent.change(within(detail).getByLabelText(/priority/i), {
      target: { value: 'urgent' },
    });
    expect(within(detail).queryByLabelText(/owner initials/i)).not.toBeInTheDocument();
    fireEvent.change(within(detail).getByLabelText(/due date/i), {
      target: { value: '2026-05-09' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save card/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({
            title: 'Wire production auth flow',
            description: 'Document cookie boundaries and refresh behavior.',
            priority: 'urgent',
            assigneeId: 'user-1',
            labelNames: ['Backend'],
            due: '2026-05-09',
          }),
        }),
      ),
    );
    expect(screen.getByRole('heading', { name: /wire production auth flow/i })).toBeInTheDocument();
    expect(screen.getByText(/Priority Urgent/)).toBeInTheDocument();
  });

  it('adds comments and shows card activity', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /view card wire auth session cookie flow/i }));
    fireEvent.change(await screen.findByLabelText(/new comment/i), {
      target: { value: 'Add rollback note before release.' },
    });
    fireEvent.click(screen.getByRole('button', { name: /add comment/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1/comments',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ body: 'Add rollback note before release.' }),
        }),
      ),
    );
    expect(screen.getByText('Add rollback note before release.')).toBeInTheDocument();
    expect(screen.getByText(/card.commented/i)).toBeInTheDocument();
  });

  it('creates a new planned card through the API', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /new card/i }));
    const dialog = screen.getByRole('dialog', { name: /create card/i });
    fireEvent.change(within(dialog).getByLabelText(/card title/i), {
      target: { value: 'Run local smoke test' },
    });
    fireEvent.change(within(dialog).getByLabelText(/assignee/i), {
      target: { value: 'user-2' },
    });
    fireEvent.change(within(dialog).getByLabelText(/labels/i), {
      target: { value: 'Backend, Risk' },
    });
    expect(screen.queryByLabelText(/owner initials/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /create card/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            columnId: 'column-planned',
            title: 'Run local smoke test',
            assigneeId: 'user-2',
            labelNames: ['Backend', 'Risk'],
          }),
        }),
      ),
    );

    const board = screen.getByRole('region', { name: /kanban board/i });
    expect(within(board).getByText('Run local smoke test')).toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: /create card/i })).not.toBeInTheDocument();
    expect(screen.getByText(/Assignee Admin/)).toBeInTheDocument();
  });

  it('removes legacy arrow movement controls from cards', async () => {
    render(<App />);

    await screen.findByRole('button', { name: /view card wire auth session cookie flow/i });

    expect(screen.queryByRole('button', { name: /move wire auth session cookie flow to in progress/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /move ready for review api shape to planned/i })).not.toBeInTheDocument();
  });

  it('resolves drag targets for columns, cards, and invalid drops', () => {
    const board = boardFixture as Parameters<typeof resolveMoveTarget>[0];

    expect(resolveMoveTarget(board, 'column-progress')).toEqual({
      columnId: 'column-progress',
      position: 1,
    });
    expect(resolveMoveTarget(board, 'card-3')).toEqual({
      columnId: 'column-review',
      position: 0,
    });
    expect(resolveMoveTarget(board, 'missing-target')).toBeNull();
  });

  it('resolves cross-column drag targets when the active card remains the top collision', () => {
    const board = boardFixture as Parameters<typeof resolveDragMoveTarget>[0];

    expect(resolveDragMoveTarget(board, 'card-1', 'card-1', ['card-1', 'column-planned', 'column-progress'])).toEqual({
      columnId: 'column-progress',
      position: 1,
    });
    expect(resolveDragMoveTarget(board, 'card-1', 'card-2', ['card-1', 'card-2'])).toEqual({
      columnId: 'column-progress',
      position: 0,
    });
    expect(resolveDragMoveTarget(board, 'card-1', 'column-planned', ['card-1', 'column-planned'])).toEqual({
      columnId: 'column-planned',
      position: 1,
    });
    expect(resolveDragMoveTarget(board, 'card-1', 'card-1', ['card-1', 'missing-target'])).toBeNull();
  });

  it('resolves planning drag targets for sprint lanes and backlog', () => {
    expect(resolvePlanningMoveTarget('planning-card:card-1', 'planning-sprint:sprint-1')).toEqual({
      cardId: 'card-1',
      sprintId: 'sprint-1',
    });
    expect(resolvePlanningMoveTarget('planning-card:card-1', 'planning-backlog')).toEqual({
      cardId: 'card-1',
      sprintId: '',
    });
    expect(resolvePlanningMoveTarget('card-1', 'planning-sprint:sprint-1')).toBeNull();
  });

  it('normalizes and updates roadmap dashboard state', () => {
    type DashboardArg = Parameters<typeof normalizeRoadmapDashboard>[0];
    type CardArg = Parameters<typeof normalizeCard>[0];
    const dashboard = normalizeRoadmapDashboard({
      ...roadmapDashboardFixture,
      dependencies: [createdDependencyFixture],
      unassignedCards: [{ ...roadmapDashboardFixture.unassignedCards[0], blockedBy: [createdDependencyFixture] }],
    } as DashboardArg);
    const card = { ...(boardFixture.columns[0].cards[0] as CardArg), epicId: 'epic-1' };

    expect(resolveRoadmapMoveTarget('roadmap-card:card-1', 'roadmap-epic:epic-1')).toEqual({ cardId: 'card-1', epicId: 'epic-1' });
    expect(resolveRoadmapMoveTarget('roadmap-card:card-1', 'roadmap-unassigned')).toEqual({ cardId: 'card-1', epicId: '' });
    expect(resolveRoadmapMoveTarget('card-1', 'roadmap-epic:epic-1')).toBeNull();

    expect(normalizeRoadmapDashboard({} as DashboardArg)).toMatchObject({ teamId: '', teamName: '', epics: [], unassignedCards: [], dependencies: [] });
    expect(upsertEpicInRoadmap(dashboard, createdEpicFixture).epics.map((plan) => plan.epic.id)).toContain('epic-2');
    expect(upsertEpicInRoadmap(dashboard, { ...roadmapEpicFixture, title: 'Identity foundations updated' }).epics[0].epic.title).toBe(
      'Identity foundations updated',
    );
    expect(assignCardInRoadmap(dashboard, card).epics[0].cards.some((candidate) => candidate.card.id === 'card-1')).toBe(true);
    expect(assignCardInRoadmap(dashboard, { ...card, epicId: '' }).unassignedCards.some((candidate) => candidate.card.id === 'card-1')).toBe(true);

    const unknownCardDependency = {
      ...createdDependencyFixture,
      id: 'dependency-unknown',
      blockedCardId: 'card-unknown',
      blockedTitle: 'Unknown card',
    };
    const unknownCardDashboard = normalizeRoadmapDashboard({ ...roadmapDashboardFixture, dependencies: [unknownCardDependency] } as DashboardArg);
    const unknownCard = { ...(boardFixture.columns[0].cards[0] as CardArg), id: 'card-unknown', title: 'Unknown card', epicId: '' };
    expect(assignCardInRoadmap(unknownCardDashboard, unknownCard).unassignedCards.find((candidate) => candidate.card.id === 'card-unknown')?.blockedBy[0].id).toBe(
      'dependency-unknown',
    );

    const doneDashboard = normalizeRoadmapDashboard({
      ...roadmapDashboardFixture,
      epics: [
        {
          ...roadmapDashboardFixture.epics[0],
          cards: [{ ...roadmapDashboardFixture.epics[0].cards[0], columnTitle: 'Done' }],
        },
      ],
    } as DashboardArg);
    expect(assignCardInRoadmap(doneDashboard, { ...(boardFixture.columns[1].cards[0] as CardArg), epicId: 'epic-1' }).epics[0]).toMatchObject({
      progress: 100,
      risk: 'complete',
    });

    const withoutDependency = upsertDependencyInRoadmap(dashboard, createdDependencyFixture, 'remove');
    expect(withoutDependency.dependencies).toHaveLength(0);
    expect(upsertDependencyInRoadmap(withoutDependency, createdDependencyFixture).unassignedCards[0].blockedBy[0].id).toBe('dependency-created');
    expect(
      upsertDependencyInRoadmap(
        {
          ...withoutDependency,
          dependencies: [{ ...createdDependencyFixture, id: 'dependency-z' }],
        },
        { ...createdDependencyFixture, id: 'dependency-a' },
      ).dependencies.map((dependency) => dependency.id),
    ).toEqual(['dependency-a', 'dependency-z']);
  });

  it('formats sprint windows for complete and partial date ranges', () => {
    const baseSprint = {
      id: 'sprint-window',
      workspaceId: 'workspace-1',
      teamId: 'team-platform',
      boardId: 'board-1',
      name: 'Sprint window',
      goal: '',
      status: 'planned' as const,
    };

    expect(sprintWindow({ ...baseSprint, startsOn: '2026-05-01', endsOn: '2026-05-15' })).toBe(
      '2026-05-01 - 2026-05-15',
    );
    expect(sprintWindow({ ...baseSprint, startsOn: '2026-05-01' })).toBe('Starts 2026-05-01');
    expect(sprintWindow({ ...baseSprint, endsOn: '2026-05-15' })).toBe('Ends 2026-05-15');
    expect(sprintWindow(baseSprint)).toBe('Dates not set');
    expect(sprintWeekRange('2026-W19')).toEqual({ startsOn: '2026-05-04', endsOn: '2026-05-10' });
    expect(sprintWeekRange('not-a-week')).toBeNull();
    expect(sprintWeekRange('2026-W54')).toBeNull();
    expect(epicWindow({ ...roadmapEpicFixture, targetOn: '' })).toBe('Starts 2026-05-04');
    expect(epicWindow({ ...roadmapEpicFixture, startsOn: '' })).toBe('Targets 2026-05-29');
    expect(epicWindow({ ...roadmapEpicFixture, startsOn: '', targetOn: '' })).toBe('Dates not set');
  });

  it('normalizes and updates planning dashboard state', () => {
    type CardArg = Parameters<typeof normalizeCard>[0];
    type DashboardArg = Parameters<typeof normalizePlanningDashboard>[0];
    type SprintArg = Parameters<typeof sprintWindow>[0];
    const planningCard = boardFixture.columns[0].cards[0] as CardArg;
    const plannedSprint = sprintFixture as SprintArg;
    const activeSprint = activeSprintFixture as SprintArg;
    const completedSprintBase = completedSprintFixture as SprintArg;
    const rawCard = {
      id: 'raw-card',
      columnId: 'column-planned',
      title: 'Raw card',
      priority: 'Normal',
      due: '',
      description: '',
      position: 3,
    } as CardArg;
    expect(normalizeCard(rawCard)).toMatchObject({ boardId: '', boardName: '', sprintId: '', labels: [] });

    const emptyDashboard = normalizePlanningDashboard({} as DashboardArg);
    expect(emptyDashboard).toMatchObject({ boardId: '', teamId: '', teamName: '', boards: [], backlog: [], plannedSprints: [], completedSprints: [] });

    const dashboard = normalizePlanningDashboard({
      ...planningDashboardFixture,
      backlog: [{ ...planningCard, sprintId: '' }],
      plannedSprints: [{ sprint: plannedSprint, cards: [] }],
      completedSprints: [],
    } as DashboardArg);
    const added = addSprintToDashboard(dashboard, { ...plannedSprint, id: 'sprint-2', name: 'Sprint 2026-06 Platform', startsOn: '2026-06-01' });
    expect(added.plannedSprints.map((plan) => plan.sprint.id)).toContain('sprint-2');
    expect(addSprintToDashboard(added, { ...plannedSprint, goal: 'Updated goal' }).plannedSprints.find((plan) => plan.sprint.id === 'sprint-1')?.sprint.goal).toBe('Updated goal');
    expect(addSprintToDashboard(dashboard, activeSprint).activeSprint?.sprint.id).toBe(activeSprint.id);
    expect(addSprintToDashboard(dashboard, completedSprintBase).completedSprints[0].sprint.status).toBe('completed');

    const assignedToPlanned = assignCardInDashboard(dashboard, { ...planningCard, sprintId: 'sprint-1' });
    expect(assignedToPlanned.backlog).toHaveLength(0);
    expect(assignedToPlanned.plannedSprints[0].cards[0].id).toBe('card-1');

    const returnedToBacklog = assignCardInDashboard(assignedToPlanned, { ...planningCard, sprintId: '' });
    expect(returnedToBacklog.backlog[0].id).toBe('card-1');

    const activeDashboard = {
      ...dashboard,
      activeSprint: { sprint: activeSprint, cards: [] },
      plannedSprints: [],
    } as DashboardArg;
    expect(assignCardInDashboard(activeDashboard, { ...planningCard, sprintId: activeSprint.id }).activeSprint?.cards[0].id).toBe('card-1');

    const completedSprint = { ...completedSprintBase, id: 'sprint-done', name: 'Done sprint' };
    const completedDashboard = { ...dashboard, plannedSprints: [], completedSprints: [{ sprint: completedSprint, cards: [] }] } as DashboardArg;
    expect(assignCardInDashboard(completedDashboard, { ...planningCard, sprintId: completedSprint.id }).completedSprints[0].cards[0].id).toBe('card-1');

    const cleanupDashboard = {
      ...dashboard,
      activeSprint: { sprint: activeSprint, cards: [{ ...planningCard, sprintId: activeSprint.id }] },
      completedSprints: [{ sprint: completedSprint, cards: [{ ...planningCard, sprintId: completedSprint.id }] }],
    } as DashboardArg;
    const cleaned = assignCardInDashboard(cleanupDashboard, { ...planningCard, sprintId: '' });
    expect(cleaned.activeSprint?.cards).toHaveLength(0);
    expect(cleaned.completedSprints[0].cards).toHaveLength(0);

    const started = startSprintInDashboard(dashboard, activeSprint);
    expect(started.activeSprint?.sprint.status).toBe('active');
    expect(started.plannedSprints).toHaveLength(0);
    const finished = completeSprintInDashboard(started, completedSprintBase);
    expect(finished.activeSprint).toBeNull();
    expect(finished.completedSprints[0].sprint.status).toBe('completed');
  });

  it('updates board-local card and wiki state helpers', () => {
    type BoardArg = Parameters<typeof addCardToBoard>[0];
    type CardArg = Parameters<typeof replaceCardInBoard>[1];
    type PageArg = Parameters<typeof upsertWikiPageInBoard>[1];
    const board = boardFixture as BoardArg;
    const card = { ...(boardFixture.columns[0].cards[0] as CardArg), title: 'Updated helper card' };
    const addedCard = { ...(boardFixture.columns[0].cards[0] as CardArg), id: 'card-helper-add', title: 'Added helper card', position: 2 };
    const newPage = { id: 'wiki-0', title: 'Architecture notes', slug: 'architecture-notes', bodyMarkdown: '# Architecture' } as PageArg;
    const editedPage = { ...boardFixture.wikiPages[0], bodyMarkdown: '# Updated deploy' } as PageArg;

    expect(addCardToBoard(null, addedCard)).toBeNull();
    expect(addCardToBoard(board, addedCard)?.columns[0].cards.some((candidate) => candidate.id === addedCard.id)).toBe(true);
    expect(replaceCardInBoard(null, card)).toBeNull();
    expect(replaceCardInBoard(board, card)?.columns[0].cards[0].title).toBe('Updated helper card');
    expect(selectedCardIdForBoard('card-1', boardFixture as Parameters<typeof selectedCardIdForBoard>[1])).toBe('card-1');
    expect(selectedCardIdForBoard('missing-card', boardFixture as Parameters<typeof selectedCardIdForBoard>[1])).toBe('');
    expect(upsertWikiPageInBoard(null, newPage)).toBeNull();
    expect(upsertWikiPageInBoard(board, newPage)?.wikiPages[0].title).toBe('Architecture notes');
    expect(upsertWikiPageInBoard(board, editedPage)?.wikiPages.find((page) => page.id === editedPage.id)?.bodyMarkdown).toBe('# Updated deploy');
  });

  it('finds planning cards across backlog and sprint lanes', () => {
    type DashboardArg = Parameters<typeof findPlanningCard>[0];
    const backlogCard = { ...boardFixture.columns[0].cards[0], id: 'planning-backlog-card', sprintId: '' };
    const activeCard = { ...boardFixture.columns[1].cards[0], id: 'planning-active-card', sprintId: activeSprintFixture.id };
    const plannedCard = { ...boardFixture.columns[2].cards[0], id: 'planning-planned-card', sprintId: sprintFixture.id };
    const completedCard = { ...boardFixture.columns[0].cards[0], id: 'planning-completed-card', sprintId: completedSprintFixture.id };
    const dashboard = {
      ...planningDashboardFixture,
      backlog: [backlogCard],
      activeSprint: { sprint: activeSprintFixture, cards: [activeCard] },
      plannedSprints: [{ sprint: sprintFixture, cards: [plannedCard] }],
      completedSprints: [{ sprint: completedSprintFixture, cards: [completedCard] }],
    } as DashboardArg;

    expect(findPlanningCard(dashboard, 'planning-backlog-card')?.title).toBe(backlogCard.title);
    expect(findPlanningCard(dashboard, 'planning-active-card')?.title).toBe(activeCard.title);
    expect(findPlanningCard(dashboard, 'planning-planned-card')?.title).toBe(plannedCard.title);
    expect(findPlanningCard(dashboard, 'planning-completed-card')?.title).toBe(completedCard.title);
    expect(findPlanningCard(dashboard, 'missing-card')).toBeUndefined();
  });

  it('sorts cards and sprints for planning timelines', () => {
    type SprintArg = Parameters<typeof sprintWindow>[0];
    const card = boardFixture.columns[0].cards[0] as Parameters<typeof normalizeCard>[0];
    const plannedSprint = sprintFixture as SprintArg;
    const first = { ...card, id: 'card-a', title: 'A card', position: 0 };
    const second = { ...card, id: 'card-b', title: 'B card', position: 1 };
    expect(sortCards([second, first]).map((card) => card.id)).toEqual(['card-a', 'card-b']);

    const startedSprint = { ...plannedSprint, id: 'sprint-started', startsOn: '', startedAt: '2026-05-03T08:00:00Z' };
    const completedSprint = { ...plannedSprint, id: 'sprint-completed', startsOn: '', completedAt: '2026-05-02T08:00:00Z' };
    expect(sortSprintPlans([{ sprint: startedSprint, cards: [] }, { sprint: completedSprint, cards: [] }]).map((plan) => plan.sprint.id)).toEqual([
      'sprint-completed',
      'sprint-started',
    ]);
    expect(sprintSortKey({ ...plannedSprint, startsOn: '2026-05-04' })).toBe('2026-05-04');
    expect(sprintSortKey(startedSprint)).toBe('2026-05-03T08:00:00Z');
    expect(sprintSortKey(completedSprint)).toBe('2026-05-02T08:00:00Z');
    expect(sprintSortKey({ ...plannedSprint, startsOn: '', name: 'Named sprint' })).toBe('Named sprint');
  });

  it('labels sprint statuses for timeline lanes', () => {
    expect(sprintStatusDisplay('planned').label).toBe('Planned');
    expect(sprintStatusDisplay('active').label).toBe('Active');
    expect(sprintStatusDisplay('completed').label).toBe('Completed');
  });

  it('labels roadmap epic statuses and risk states', () => {
    expect(epicStatusLabel('planned')).toBe('Planned');
    expect(epicStatusLabel('active')).toBe('Active');
    expect(epicStatusLabel('done')).toBe('Done');
    expect(epicStatusClass('planned')).toContain('slate');
    expect(epicStatusClass('active')).toContain('emerald');
    expect(epicStatusClass('done')).toContain('slate');

    expect(roadmapRiskLabel('on_track')).toBe('On track');
    expect(roadmapRiskLabel('blocked')).toBe('Blocked');
    expect(roadmapRiskLabel('complete')).toBe('Complete');
    expect(roadmapRiskClass('on_track')).toContain('sky');
    expect(roadmapRiskClass('blocked')).toContain('rose');
    expect(roadmapRiskClass('complete')).toContain('emerald');
  });

  it('classifies due dates and card display helpers', () => {
    expect(dueStatus('').label).toBe('date missing');
    expect(dueStatus(dateOffset(-1)).label).toBe('overdue');
    expect(dueStatus(dateOffset(1)).label).toBe('due soon');
    expect(dueStatus(dateOffset(5)).label).toBe('scheduled');
    expect(columnAccent(0)).toBe('bg-sky-500');
    expect(columnAccent(1)).toBe('bg-amber-500');
    expect(columnAccent(9)).toBe('bg-emerald-500');

    const card = boardFixture.columns[0].cards[0] as Parameters<typeof cardAssigneeText>[0];
    expect(cardAssigneeText(card)).toBe('Admin');
    expect(cardAssigneeText({ ...card, assigneeName: '', assigneeEmail: 'admin@example.com' })).toBe('admin@example.com');
    expect(cardAssigneeText({ ...card, assigneeName: '', assigneeEmail: '' })).toBe('Unassigned');
  });

  it('matches card search and filter combinations', () => {
    const card = boardFixture.columns[0].cards[0] as Parameters<typeof cardMatchesFilters>[0];
    const emptyFilters: Parameters<typeof cardMatchesFilters>[1] = { assigneeId: 'all', labelId: 'all', priority: 'all', dueStatus: 'all' };

    expect(cardMatchesSearch(card, '')).toBe(true);
    expect(cardMatchesSearch(card, 'backend')).toBe(true);
    expect(cardMatchesSearch(card, 'not-on-card')).toBe(false);
    expect(hasActiveBoardFilters(emptyFilters)).toBe(false);
    expect(hasActiveBoardFilters({ ...emptyFilters, assigneeId: 'user-1' })).toBe(true);

    expect(cardMatchesFilters(card, emptyFilters)).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, assigneeId: 'unassigned' })).toBe(false);
    expect(cardMatchesFilters({ ...card, assigneeId: '' }, { ...emptyFilters, assigneeId: 'unassigned' })).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, assigneeId: 'user-2' })).toBe(false);
    expect(cardMatchesFilters(card, { ...emptyFilters, assigneeId: 'user-1' })).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, labelId: 'none' })).toBe(false);
    expect(cardMatchesFilters({ ...card, labels: [] }, { ...emptyFilters, labelId: 'none' })).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, labelId: 'label-risk' })).toBe(false);
    expect(cardMatchesFilters(card, { ...emptyFilters, labelId: 'label-backend' })).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, priority: 'urgent' })).toBe(false);
    expect(cardMatchesFilters(card, { ...emptyFilters, priority: 'high' })).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, dueStatus: dueStatus(card.due).label })).toBe(true);
    expect(cardMatchesFilters(card, { ...emptyFilters, dueStatus: 'scheduled' })).toBe(false);
  });

  it('chooses default assignees from board membership', () => {
    expect(defaultAssigneeId(null, userFixture)).toBe('');
    expect(defaultAssigneeId(boardFixture as Parameters<typeof defaultAssigneeId>[0], null)).toBe('');
    expect(defaultAssigneeId(boardFixture as Parameters<typeof defaultAssigneeId>[0], userFixture)).toBe('user-1');
    expect(defaultAssigneeId(boardFixture as Parameters<typeof defaultAssigneeId>[0], { ...userFixture, id: 'missing-user' })).toBe('');
  });

  it('resolves team role capabilities for the current user', () => {
    expect(teamRoleForUser(teamsFixture[0] as Parameters<typeof teamRoleForUser>[0], userFixture)).toBe('owner');
    expect(teamRoleForUser(teamAdminFixture as Parameters<typeof teamRoleForUser>[0], developerUserFixture)).toBe('admin');
    expect(teamRoleForUser(teamMemberFixture as Parameters<typeof teamRoleForUser>[0], developerUserFixture)).toBe('member');
    expect(teamRoleForUser(teamViewerFixture as Parameters<typeof teamRoleForUser>[0], viewerUserFixture)).toBe('viewer');

    expect(canManageTeam(teamAdminFixture as Parameters<typeof canManageTeam>[0], developerUserFixture)).toBe(true);
    expect(canWriteTeam(teamMemberFixture as Parameters<typeof canWriteTeam>[0], developerUserFixture)).toBe(true);
    expect(canManageTeam(teamMemberFixture as Parameters<typeof canManageTeam>[0], developerUserFixture)).toBe(false);
    expect(canReadTeam(teamViewerFixture as Parameters<typeof canReadTeam>[0], viewerUserFixture)).toBe(true);
    expect(canWriteTeam(teamViewerFixture as Parameters<typeof canWriteTeam>[0], viewerUserFixture)).toBe(false);
    expect(canManageTeam(null, { ...userFixture, id: 'global-admin-without-membership' })).toBe(true);
    expect(canReadTeam(null, null)).toBe(false);
  });

  it('keeps viewer users in read-only workspace mode', async () => {
    stubRoleWorkspace(viewerUserFixture, teamViewerFixture, {
      ...planningDashboardFixture,
      plannedSprints: [{ sprint: sprintFixture, cards: [] }],
    });

    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /new card/i })).toBeDisabled();
    expect(screen.queryByRole('button', { name: /settings/i })).not.toBeInTheDocument();

    await clickPrimaryNavButton(/wiki/i);
    expect(screen.queryByRole('button', { name: /new wiki page/i })).not.toBeInTheDocument();

    await clickPrimaryNavButton(/planning/i);
    expect(await screen.findByRole('region', { name: /planned sprint sprint 2026-w19/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /create weekly sprint/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start sprint 2026-w19/i })).not.toBeInTheDocument();
  });

  it('allows member users to write cards and wiki while hiding administration', async () => {
    stubRoleWorkspace(developerUserFixture, teamMemberFixture);

    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /new card/i })).toBeEnabled();
    expect(screen.queryByRole('button', { name: /settings/i })).not.toBeInTheDocument();

    await clickPrimaryNavButton(/wiki/i);
    expect(screen.getByRole('button', { name: /new wiki page/i })).toBeEnabled();

    await clickPrimaryNavButton(/planning/i);
    expect(screen.queryByRole('button', { name: /create weekly sprint/i })).not.toBeInTheDocument();
  });

  it('allows team admins to manage board structure without workspace user administration', async () => {
    stubRoleWorkspace(developerUserFixture, teamAdminFixture);

    render(<App />);

    await clickPrimaryNavButton(/settings/i);

    expect(await screen.findByRole('heading', { name: /workspace settings/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /new board/i })).not.toBeInTheDocument();
    expect(screen.getByText(/team board: platform board/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add column/i })).toBeEnabled();
    expect(screen.queryByLabelText(/member email/i)).not.toBeInTheDocument();
    expect(screen.getByText(/admin access is required to manage workspace members/i)).toBeInTheDocument();
  });

  it('filters cards and wiki pages with workspace search', async () => {
    render(<App />);

    fireEvent.change(await screen.findByLabelText(/search workspace/i), {
      target: { value: 'deployment' },
    });

    const board = screen.getByRole('region', { name: /kanban board/i });
    const wiki = screen.getByRole('complementary', { name: /wiki pages/i });

    expect(within(board).getByText('Deployment checklist')).toBeInTheDocument();
    expect(within(board).queryByText('Wire auth session cookie flow')).not.toBeInTheDocument();
    expect(within(wiki).getByText('Deployment checklist')).toBeInTheDocument();
    expect(within(wiki).queryByText('Onboarding notes')).not.toBeInTheDocument();
  });

  it('switches between board, wiki, and settings views', async () => {
    render(<App />);

    await clickPrimaryNavButton(/roadmap/i);
    expect(screen.getByRole('heading', { name: /roadmap dashboard/i })).toBeInTheDocument();

    await clickPrimaryNavButton(/planning/i);
    expect(screen.getByRole('heading', { name: /planning dashboard/i })).toBeInTheDocument();

    await clickPrimaryNavButton(/wiki/i);
    expect(screen.getByRole('heading', { name: /wiki pages/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /platform board/i })).not.toBeInTheDocument();

    await clickPrimaryNavButton(/settings/i);
    expect(screen.getByRole('heading', { name: /workspace settings/i })).toBeInTheDocument();

    await clickPrimaryNavButton(/boards/i);
    expect(screen.getByRole('heading', { name: /platform board/i })).toBeInTheDocument();
  });

  it('plans epics and dependencies on the roadmap dashboard', async () => {
    render(<App />);

    await clickPrimaryNavButton(/roadmap/i);

    expect(await screen.findByRole('heading', { name: /roadmap dashboard/i })).toBeInTheDocument();
    expect(await screen.findByRole('region', { name: /epic identity foundations/i })).toHaveTextContent('Ready for review API shape');
    expect(screen.getByRole('region', { name: /unassigned roadmap backlog/i })).toHaveTextContent('Wire auth session cookie flow');

    fireEvent.change(screen.getByLabelText(/epic title/i), {
      target: { value: 'Roadmap hardening' },
    });
    fireEvent.change(screen.getByLabelText(/epic description/i), {
      target: { value: 'Make roadmap planning usable for the team.' },
    });
    fireEvent.change(screen.getByLabelText(/epic start/i), {
      target: { value: '2026-06-01' },
    });
    fireEvent.change(screen.getByLabelText(/epic target/i), {
      target: { value: '2026-06-19' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create epic/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/epics',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            teamId: 'team-platform',
            title: 'Roadmap hardening',
            description: 'Make roadmap planning usable for the team.',
            status: 'planned',
            startsOn: '2026-06-01',
            targetOn: '2026-06-19',
          }),
        }),
      ),
    );
    expect(screen.getByRole('region', { name: /epic roadmap hardening/i })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/epic assignment for wire auth session cookie flow/i), {
      target: { value: 'epic-1' },
    });
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1/epic',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ epicId: 'epic-1' }),
        }),
      ),
    );
    expect(screen.getByRole('region', { name: /epic identity foundations/i })).toHaveTextContent('Wire auth session cookie flow');

    fireEvent.change(screen.getByLabelText(/dependency blocker for wire auth session cookie flow/i), {
      target: { value: 'card-2' },
    });
    fireEvent.click(screen.getByRole('button', { name: /add dependency for wire auth session cookie flow/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1/dependencies',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ blockerCardId: 'card-2' }),
        }),
      ),
    );
    expect(screen.getAllByText(/blocked by ready for review api shape/i).length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole('button', { name: /remove dependency ready for review api shape from wire auth session cookie flow/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/card-dependencies/dependency-created',
        expect.objectContaining({
          method: 'DELETE',
        }),
      ),
    );
    expect(screen.queryByText(/blocked by ready for review api shape/i)).not.toBeInTheDocument();
  });

  it('plans weekly sprints from backlog through completion', async () => {
    render(<App />);

    await clickPrimaryNavButton(/planning/i);

    expect(await screen.findByRole('heading', { name: /planning dashboard/i })).toBeInTheDocument();
    const backlog = screen.getByRole('region', { name: /backlog/i });
    expect(within(backlog).getByText('Wire auth session cookie flow')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/sprint goal/i), {
      target: { value: 'Ship planning foundations' },
    });
    fireEvent.change(screen.getByLabelText(/sprint week/i), {
      target: { value: '2026-W20' },
    });
    fireEvent.change(screen.getByLabelText(/sprint week/i), {
      target: { value: '2026-W19' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create weekly sprint/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/sprints',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            teamId: 'team-platform',
            goal: 'Ship planning foundations',
            startsOn: '2026-05-04',
            endsOn: '2026-05-10',
          }),
        }),
      ),
    );

    const timeline = screen.getByRole('region', { name: /sprint timeline/i });
    expect(within(timeline).getByText('Sprint 2026-W19')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start sprint/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /complete sprint 2026-w19/i }));
    fireEvent.click(screen.getByRole('button', { name: /^complete sprint$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/sprints/sprint-1/complete',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ rollover: [] }),
        }),
      ),
    );
    expect(await screen.findByRole('region', { name: /completed sprint sprint 2026-w19/i })).toHaveTextContent('Sprint 2026-W19');
  });

  it('chooses next-sprint rollover targets when completing the active sprint', async () => {
    const currentSprint = {
      ...activeSprintFixture,
      id: 'sprint-current',
      name: 'Sprint 2026-04 Current',
    };
    const nextSprint = {
      ...sprintFixture,
      id: 'sprint-next',
      name: 'Sprint 2026-W20',
      startsOn: '2026-05-11',
      endsOn: '2026-05-17',
    };
    const activeCards = [
      { ...boardFixture.columns[0].cards[0], sprintId: currentSprint.id },
      { ...boardFixture.columns[1].cards[0], sprintId: currentSprint.id },
    ];
    let planningResponse: unknown = {
      boardId: 'board-1',
      teamId: 'team-platform',
      teamName: 'Platform Engineering',
      boards: boardSummaries,
      backlog: [],
      activeSprint: { sprint: currentSprint, cards: activeCards },
      plannedSprints: [{ sprint: nextSprint, cards: [] }],
      completedSprints: [],
    };

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
        }
        if (url === '/api/boards') {
          return jsonResponse(boardSummaries);
        }
        if (url === '/api/teams') {
          return jsonResponse(teamsFixture);
        }
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/planning?teamId=team-platform') {
          return jsonResponse(planningResponse);
        }
        if (url === '/api/sprints/sprint-current/complete' && init?.method === 'POST') {
          planningResponse = {
            boardId: 'board-1',
            teamId: 'team-platform',
            teamName: 'Platform Engineering',
            boards: boardSummaries,
            backlog: [{ ...activeCards[1], sprintId: '' }],
            activeSprint: null,
            plannedSprints: [{ sprint: nextSprint, cards: [{ ...activeCards[0], sprintId: nextSprint.id }] }],
            completedSprints: [{ sprint: { ...currentSprint, status: 'completed', completedAt: '2026-05-01T12:00:00Z' }, cards: [] }],
          };
          return jsonResponse({ ...currentSprint, status: 'completed', completedAt: '2026-05-01T12:00:00Z' });
        }

        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );

    render(<App />);
    await clickPrimaryNavButton(/planning/i);

    expect(await screen.findByRole('region', { name: /active sprint sprint 2026-04 current/i })).toHaveTextContent('Sprint 2026-04 Current');

    fireEvent.click(screen.getByRole('button', { name: /complete sprint 2026-04 current/i }));
    fireEvent.change(screen.getByLabelText(/completion target for wire auth session cookie flow/i), {
      target: { value: 'sprint-next' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^complete sprint$/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/sprints/sprint-current/complete',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            rollover: [
              { cardId: 'card-2', sprintId: '' },
              { cardId: 'card-1', sprintId: 'sprint-next' },
            ],
          }),
        }),
      ),
    );
    expect(await screen.findByRole('region', { name: /planned sprint sprint 2026-w20/i })).toHaveTextContent('Wire auth session cookie flow');
  });

  it('expands the kanban board and collapses the left navigation', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /view card wire auth session cookie flow/i }));
    expect(await screen.findByRole('complementary', { name: /card detail/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /expand kanban board/i }));
    expect(screen.getByRole('button', { name: /exit full screen board/i })).toBeInTheDocument();
    expect(screen.queryByRole('complementary', { name: /card detail/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('complementary', { name: /wiki pages/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /exit full screen board/i }));
    expect(screen.getByRole('button', { name: /expand kanban board/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /collapse navigation/i }));
    expect(screen.getByRole('button', { name: /expand navigation/i })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: /primary navigation/i })).toHaveAttribute('data-collapsed', 'true');
  });

  it('collapses the right wiki rail and reopens it when a card is selected', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    const sidePanel = screen.getByRole('complementary', { name: /board side panel/i });
    expect(within(sidePanel).getByRole('complementary', { name: /wiki pages/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /collapse wiki and detail panel/i })).toBeInTheDocument();

    fireEvent.click(within(sidePanel).getByRole('button', { name: /collapse wiki and detail panel/i }));
    const collapsedPanel = screen.getByRole('complementary', { name: /collapsed board side panel/i });
    expect(within(collapsedPanel).getByRole('button', { name: /expand wiki and detail panel/i })).toBeInTheDocument();
    expect(screen.queryByRole('complementary', { name: /wiki pages/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /view card wire auth session cookie flow/i }));
    expect(await screen.findByRole('complementary', { name: /card detail/i })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: /wiki pages/i })).toBeInTheDocument();
  });

  it('opens a dedicated card detail page from the right panel', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /view card wire auth session cookie flow/i }));
    fireEvent.click(await screen.findByRole('button', { name: /open card page/i }));

    const page = screen.getByRole('region', { name: /card detail page/i });
    expect(within(page).getByRole('heading', { name: /wire auth session cookie flow/i })).toBeInTheDocument();
    expect(within(page).getByText(/map the session cookie lifecycle/i)).toBeInTheDocument();
    expect(within(page).getByText(/Assignee Admin/i)).toBeInTheDocument();
    expect(within(page).getByText('Backend')).toBeInTheDocument();
    expect(within(page).getByText(/Due 2026-04-30/i)).toBeInTheDocument();
    expect(within(page).getByText(/Activity/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /back to board/i }));
    expect(screen.getByRole('heading', { name: /platform board/i })).toBeInTheDocument();
  });

  it('edits card details from the dedicated card page', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /view card wire auth session cookie flow/i }));
    fireEvent.click(await screen.findByRole('button', { name: /open card page/i }));

    const page = screen.getByRole('region', { name: /card detail page/i });
    fireEvent.click(within(page).getByRole('button', { name: /edit card/i }));
    fireEvent.change(within(page).getByLabelText(/card title/i), {
      target: { value: 'Wire production auth flow' },
    });
    fireEvent.change(within(page).getByLabelText(/description/i), {
      target: { value: 'Document cookie boundaries and refresh behavior.' },
    });
    fireEvent.change(within(page).getByLabelText(/priority/i), {
      target: { value: 'urgent' },
    });
    expect(within(page).queryByLabelText(/owner initials/i)).not.toBeInTheDocument();
    fireEvent.change(within(page).getByLabelText(/due date/i), {
      target: { value: '2026-05-09' },
    });
    fireEvent.click(within(page).getByRole('button', { name: /save card/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({
            title: 'Wire production auth flow',
            description: 'Document cookie boundaries and refresh behavior.',
            priority: 'urgent',
            assigneeId: 'user-1',
            labelNames: ['Backend'],
            due: '2026-05-09',
          }),
        }),
      ),
    );
    expect(within(page).getByRole('heading', { name: /wire production auth flow/i })).toBeInTheDocument();
    expect(within(page).getByText(/Due 2026-05-09/i)).toBeInTheDocument();
  });

  it('manages workspace members and roles from settings', async () => {
    render(<App />);

    await clickPrimaryNavButton(/settings/i);

    expect(await screen.findByText('admin@example.com')).toBeInTheDocument();
    expect(screen.getAllByText('dev@example.com').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Owner').length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText(/member email/i), {
      target: { value: 'qa@example.com' },
    });
    fireEvent.change(screen.getByLabelText(/display name/i), {
      target: { value: 'QA' },
    });
    fireEvent.change(screen.getByLabelText(/temporary password/i), {
      target: { value: 'correct horse battery qa' },
    });
    fireEvent.change(screen.getByLabelText(/new member role/i), {
      target: { value: 'viewer' },
    });
    fireEvent.click(screen.getByRole('button', { name: /add member/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/members',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            email: 'qa@example.com',
            displayName: 'QA',
            password: 'correct horse battery qa',
            role: 'viewer',
          }),
        }),
      ),
    );
    expect(screen.getByText('qa@example.com')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/role for dev@example.com/i), {
      target: { value: 'admin' },
    });

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/members/member-2',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ role: 'admin' }),
        }),
      ),
    );
    expect(screen.getByText(/role updated/i)).toBeInTheDocument();
  });

  it('edits markdown wiki pages with preview', async () => {
    render(<App />);

    await clickPrimaryNavButton(/wiki/i);
    fireEvent.click(await screen.findByRole('button', { name: /deployment checklist/i }));
    expect(await screen.findByRole('heading', { name: /deploy/i })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/markdown body/i), {
      target: { value: '# Deploy safely\n\n- Run migrations' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save page/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/wiki/wiki-1',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({
            title: 'Deployment checklist',
            bodyMarkdown: '# Deploy safely\n\n- Run migrations',
          }),
        }),
      ),
    );
    expect(screen.getByRole('heading', { name: /deploy safely/i })).toBeInTheDocument();
  });

  it('creates markdown wiki pages', async () => {
    render(<App />);

    await clickPrimaryNavButton(/wiki/i);
    fireEvent.click(screen.getByRole('button', { name: /new wiki page/i }));
    fireEvent.change(screen.getByLabelText(/wiki title/i), {
      target: { value: 'Release runbook' },
    });
    fireEvent.change(screen.getByLabelText(/markdown body/i), {
      target: { value: '# Release runbook\n\nShip carefully.' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create page/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/wiki',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            title: 'Release runbook',
            bodyMarkdown: '# Release runbook\n\nShip carefully.',
            boardId: 'board-1',
          }),
        }),
      ),
    );
    expect(screen.getByRole('button', { name: /release runbook/i })).toBeInTheDocument();
  });

  it('renders wiki pages as a tree and opens nested pages', async () => {
    render(<App />);

    await clickPrimaryNavButton(/wiki/i);

    const tree = screen.getByRole('navigation', { name: /wiki page tree/i });
    expect(within(tree).getByText('Engineering')).toBeInTheDocument();
    expect(within(tree).getByText('Runbooks')).toBeInTheDocument();

    fireEvent.click(within(tree).getByRole('button', { name: /open wiki page engineering\/runbooks\/deploy/i }));

    expect(await screen.findByRole('heading', { name: /^deploy$/i })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith('/api/wiki/wiki-3');
  });

  it('renders rich markdown safely in the wiki preview', async () => {
    render(<App />);

    await clickPrimaryNavButton(/wiki/i);
    fireEvent.click(screen.getByRole('button', { name: /new wiki page/i }));
    fireEvent.change(screen.getByLabelText(/markdown body/i), {
      target: {
        value:
          '# Deploy runbook\n\nUse **bold checks**, _careful rollout_, [release docs](https://example.com/runbook), and `kubectl`.\n\n> Pause if readiness fails.\n\n1. Build image\n2. Run smoke test\n\n### Risk table\n\n| Risk | Owner |\n| --- | --- |\n| Drift | QA |\n\n```bash\ngo test ./...\n```\n\n---\n\n<script>alert("x")</script>',
      },
    });

    const preview = screen.getByRole('article', { name: /markdown preview/i });
    expect(within(preview).getByRole('heading', { name: /deploy runbook/i })).toBeInTheDocument();
    expect(within(preview).getByText('bold checks').tagName).toBe('STRONG');
    expect(within(preview).getByText('careful rollout').tagName).toBe('EM');
    expect(within(preview).getByRole('link', { name: /release docs/i })).toHaveAttribute('href', 'https://example.com/runbook');
    expect(within(preview).getByText('kubectl').tagName).toBe('CODE');
    expect(within(preview).getByText('Pause if readiness fails.').closest('blockquote')).not.toBeNull();
    expect(within(preview).getByText('Build image').closest('ol')).not.toBeNull();
    expect(within(preview).getByRole('heading', { name: /risk table/i })).toBeInTheDocument();
    expect(within(preview).getByText('Risk').closest('th')).not.toBeNull();
    expect(within(preview).getByText('QA').closest('td')).not.toBeNull();
    expect(within(preview).getByText('go test ./...').closest('pre')).not.toBeNull();
    expect(preview.querySelector('script')).toBeNull();
  });

  it('previews markdown paragraphs, nested headings, lists, and empty pages', async () => {
    render(<App />);

    await clickPrimaryNavButton(/wiki/i);
    fireEvent.click(screen.getByRole('button', { name: /new wiki page/i }));

    expect(screen.getByText(/no content yet/i)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/markdown body/i), {
      target: {
        value: '## Rollout notes\n\nShip in small batches\nwith checks.\n\n- Run migrations\n- Verify health',
      },
    });

    expect(screen.getByRole('heading', { name: /rollout notes/i })).toBeInTheDocument();
    expect(screen.getByText('Ship in small batches with checks.')).toBeInTheDocument();
    expect(screen.getByText('Run migrations')).toBeInTheDocument();
    expect(screen.getByText('Verify health')).toBeInTheDocument();
  });
});

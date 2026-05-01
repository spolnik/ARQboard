import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';
import App, { resolveDragMoveTarget, resolveMoveTarget, sprintWindow } from './App';

const userFixture = {
  id: 'user-1',
  email: 'admin@example.com',
  displayName: 'Admin',
  isAdmin: true,
};

const boardFixture = {
  id: 'board-1',
  name: 'Platform Board',
  slug: 'platform',
  columns: [
    {
      id: 'column-planned',
      title: 'Planned',
      position: 0,
      cards: [
        {
          id: 'card-1',
          columnId: 'column-planned',
          title: 'Wire auth session cookie flow',
          owner: 'MS',
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
          title: 'Ready for review API shape',
          owner: 'AK',
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
          title: 'Deployment checklist',
          owner: 'JL',
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
    name: 'Platform Board',
    slug: 'platform',
    columnCount: 4,
    cardCount: 3,
  },
  {
    id: 'board-release',
    workspaceId: 'workspace-1',
    name: 'Release Train',
    slug: 'release-train',
    columnCount: 4,
    cardCount: 0,
  },
];

const releaseBoardFixture = {
  id: 'board-release',
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
  title: 'Run local smoke test',
  owner: 'QA',
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
  owner: 'QA',
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
  boardId: 'board-1',
  name: 'Sprint 2026-05 Platform',
  goal: 'Ship planning foundations',
  status: 'planned',
  startsOn: '2026-05-04',
  endsOn: '2026-05-15',
};

const activeSprintFixture = {
  ...sprintFixture,
  status: 'active',
  startedAt: '2026-05-04T08:00:00Z',
};

const completedSprintFixture = {
  ...activeSprintFixture,
  status: 'completed',
  completedAt: '2026-05-15T16:00:00Z',
};

const planningDashboardFixture = {
  boardId: 'board-1',
  backlog: [boardFixture.columns[0].cards[0]],
  plannedSprints: [],
  completedSprints: [],
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

describe('App', () => {
  beforeEach(() => {
    let planningResponse: unknown = planningDashboardFixture;
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
        if (url === '/api/planning?boardId=board-1') {
          return jsonResponse(planningResponse);
        }
        if (url === '/api/sprints' && init?.method === 'POST') {
          return jsonResponse(sprintFixture, 201);
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
        return jsonResponse({ error: { code: 'not_ready' } }, 500);
      }),
    );

    render(<App />);

    expect(await screen.findByText(/could not load the board/i)).toBeInTheDocument();
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

  it('switches between boards from the board selector', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Board'), {
      target: { value: 'board-release' },
    });

    expect(await screen.findByRole('heading', { name: /release train/i })).toBeInTheDocument();
    expect(screen.getAllByText('No matches').length).toBeGreaterThan(0);
    expect(fetch).toHaveBeenCalledWith('/api/boards/board-release');
  });

  it('creates a new board and selects it', async () => {
    render(<App />);

    await clickPrimaryNavButton(/settings/i);
    fireEvent.click(await screen.findByRole('button', { name: /new board/i }));
    fireEvent.change(screen.getByLabelText(/board name/i), {
      target: { value: 'Security Backlog' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create board/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/boards',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ name: 'Security Backlog' }),
        }),
      ),
    );
    expect(await screen.findByRole('heading', { name: /security backlog/i })).toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: /create board/i })).not.toBeInTheDocument();
  });

  it('adds and renames board columns', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add column/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /rename planned/i })).not.toBeInTheDocument();

    await clickPrimaryNavButton(/settings/i);
    fireEvent.click(await screen.findByRole('button', { name: /add column/i }));
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

    fireEvent.click(screen.getByRole('button', { name: /rename planned/i }));
    fireEvent.change(screen.getByLabelText(/column title/i), {
      target: { value: 'Backlog' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save column/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/columns/column-planned',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ title: 'Backlog' }),
        }),
      ),
    );
    expect(screen.getByText('Backlog')).toBeInTheDocument();
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
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);

    expect(await screen.findByRole('heading', { name: /loading board/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /no boards/i })).toBeInTheDocument();
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
      if (url === '/api/boards' && init?.method === 'POST') {
        return jsonResponse({ error: { code: 'unavailable' } }, 500);
      }
      if (url === '/api/boards') {
        return jsonResponse(boardSummaries);
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
      if (url === '/api/columns/column-planned' && init?.method === 'PATCH') {
        return jsonResponse({ error: { code: 'unavailable' } }, 500);
      }
      throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<App />);

    await clickPrimaryNavButton(/settings/i);
    fireEvent.click(await screen.findByRole('button', { name: /new board/i }));
    fireEvent.change(screen.getByLabelText(/board name/i), {
      target: { value: 'Security Backlog' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create board/i }));
    expect(await screen.findByText(/could not create the board/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    await clickPrimaryNavButton(/settings/i);
    fireEvent.click(screen.getByRole('button', { name: /add column/i }));
    fireEvent.change(screen.getByLabelText(/column title/i), {
      target: { value: 'Blocked' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^add column$/i }));
    expect(await screen.findByText(/could not add the column/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    fireEvent.click(screen.getByRole('button', { name: /rename planned/i }));
    fireEvent.change(screen.getByLabelText(/column title/i), {
      target: { value: 'Backlog' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save column/i }));
    expect(await screen.findByText(/could not rename the column/i)).toBeInTheDocument();
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
    expect(within(detail).getByText(/Owner MS/)).toBeInTheDocument();
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
    fireEvent.change(within(detail).getByLabelText(/owner initials/i), {
      target: { value: 'QA' },
    });
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
            ownerInitials: 'QA',
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
    fireEvent.change(screen.getByLabelText(/card title/i), {
      target: { value: 'Run local smoke test' },
    });
    fireEvent.change(screen.getByLabelText(/owner initials/i), {
      target: { value: 'QA' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create card/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            columnId: 'column-planned',
            title: 'Run local smoke test',
            ownerInitials: 'QA',
          }),
        }),
      ),
    );

    const board = screen.getByRole('region', { name: /kanban board/i });
    expect(within(board).getByText('Run local smoke test')).toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: /create card/i })).not.toBeInTheDocument();
    expect(screen.getByText(/Owner QA/)).toBeInTheDocument();
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

  it('formats sprint windows for complete and partial date ranges', () => {
    const baseSprint = {
      id: 'sprint-window',
      workspaceId: 'workspace-1',
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

  it('plans named sprints from backlog through completion', async () => {
    render(<App />);

    await clickPrimaryNavButton(/planning/i);

    expect(await screen.findByRole('heading', { name: /planning dashboard/i })).toBeInTheDocument();
    const backlog = screen.getByRole('region', { name: /backlog/i });
    expect(within(backlog).getByText('Wire auth session cookie flow')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/sprint name/i), {
      target: { value: 'Sprint 2026-05 Platform' },
    });
    fireEvent.change(screen.getByLabelText(/sprint goal/i), {
      target: { value: 'Ship planning foundations' },
    });
    fireEvent.change(screen.getByLabelText(/starts on/i), {
      target: { value: '2026-05-04' },
    });
    fireEvent.change(screen.getByLabelText(/ends on/i), {
      target: { value: '2026-05-15' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create sprint/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/sprints',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            boardId: 'board-1',
            name: 'Sprint 2026-05 Platform',
            goal: 'Ship planning foundations',
            startsOn: '2026-05-04',
            endsOn: '2026-05-15',
          }),
        }),
      ),
    );

    fireEvent.change(screen.getByLabelText(/sprint for wire auth session cookie flow/i), {
      target: { value: 'sprint-1' },
    });
    fireEvent.click(screen.getByRole('button', { name: /assign wire auth session cookie flow/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1/sprint',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ sprintId: 'sprint-1' }),
        }),
      ),
    );

    const planned = screen.getByRole('region', { name: /planned sprints/i });
    expect(within(planned).getByText('Wire auth session cookie flow')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /start sprint 2026-05 platform/i }));
    expect(await screen.findByRole('region', { name: /active sprint/i })).toHaveTextContent('Sprint 2026-05 Platform');

    fireEvent.click(screen.getByRole('button', { name: /complete sprint 2026-05 platform/i }));
    fireEvent.click(screen.getByRole('button', { name: /^complete sprint$/i }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/sprints/sprint-1/complete',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ rollover: [{ cardId: 'card-1', sprintId: '' }] }),
        }),
      ),
    );
    expect(await screen.findByRole('region', { name: /completed sprints/i })).toHaveTextContent('Sprint 2026-05 Platform');
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
      name: 'Sprint 2026-05 Follow-up',
    };
    const activeCards = [
      { ...boardFixture.columns[0].cards[0], sprintId: currentSprint.id },
      { ...boardFixture.columns[1].cards[0], sprintId: currentSprint.id },
    ];
    let planningResponse: unknown = {
      boardId: 'board-1',
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
        if (url === '/api/boards/board-1') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/planning?boardId=board-1') {
          return jsonResponse(planningResponse);
        }
        if (url === '/api/sprints/sprint-current/complete' && init?.method === 'POST') {
          planningResponse = {
            boardId: 'board-1',
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

    expect(await screen.findByRole('region', { name: /active sprint/i })).toHaveTextContent('Sprint 2026-04 Current');

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
    expect(await screen.findByRole('region', { name: /planned sprints/i })).toHaveTextContent('Wire auth session cookie flow');
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
    expect(within(page).getByText(/Owner MS/i)).toBeInTheDocument();
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
    fireEvent.change(within(page).getByLabelText(/owner initials/i), {
      target: { value: 'QA' },
    });
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
            ownerInitials: 'QA',
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
    expect(screen.getByText('dev@example.com')).toBeInTheDocument();
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

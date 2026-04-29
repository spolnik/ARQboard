import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';
import App from './App';

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
          due: 'Apr 30',
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
          due: 'Today',
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
          due: 'May 3',
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
  ],
};

const createdCard = {
  id: 'card-new',
  columnId: 'column-planned',
  title: 'Run local smoke test',
  owner: 'QA',
  priority: 'Normal',
  due: 'Later',
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
  due: 'May 9',
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

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('App', () => {
  beforeEach(() => {
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
        if (url === '/api/boards/default') {
          return jsonResponse(boardFixture);
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
        if (url === '/api/wiki/wiki-1' && init?.method === 'PATCH') {
          return jsonResponse(updatedWikiPage);
        }

        throw new Error(`Unexpected fetch ${init?.method ?? 'GET'} ${url}`);
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('loads the board and wiki workspace surface from the API', async () => {
    render(<App />);

    expect(await screen.findByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /drag wire auth session cookie flow/i })).toBeInTheDocument();
    expect(screen.getByText('Ready for review')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: /done column/i })).toBeInTheDocument();
    expect(screen.getAllByText('Deployment checklist').length).toBeGreaterThan(0);
    expect(fetch).toHaveBeenCalledWith('/api/boards/default');
  });

  it('shows a helpful error when the board cannot load', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === '/api/me') {
          return jsonResponse(userFixture);
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
        if (url === '/api/boards/default') {
          return jsonResponse(lowPriorityBoard);
        }
        if (url === '/api/cards/card-1') {
          return jsonResponse({ card: lowPriorityBoard.columns[0].cards[0], comments: [], activity: [] });
        }
        return jsonResponse({ card: lowPriorityBoard.columns[1].cards[0], comments: [], activity: [] });
      }),
    );

    render(<App />);

    expect(await screen.findByText('Low')).toBeInTheDocument();
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
      if (url === '/api/boards/default') {
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
    expect(fetchMock).not.toHaveBeenCalledWith('/api/boards/default');

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

  it('falls back to the first available card when the preferred default card is absent', async () => {
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
        if (url === '/api/boards/default') {
          return jsonResponse(fallbackBoard);
        }
        if (url === '/api/cards/card-1') {
          return jsonResponse({ card: fallbackBoard.columns[0].cards[0], comments: [], activity: [] });
        }
        throw new Error(`Unexpected fetch ${url}`);
      }),
    );

    render(<App />);

    const detail = await screen.findByRole('complementary', { name: /card detail/i });
    expect(within(detail).getByRole('heading', { name: /wire auth session cookie flow/i })).toBeInTheDocument();
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
        if (url === '/api/boards/default') {
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
    fireEvent.change(screen.getByLabelText(/card title/i), {
      target: { value: 'Wire production auth flow' },
    });
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: 'Document cookie boundaries and refresh behavior.' },
    });
    fireEvent.change(screen.getByLabelText(/priority/i), {
      target: { value: 'urgent' },
    });
    fireEvent.change(screen.getByLabelText(/owner initials/i), {
      target: { value: 'QA' },
    });
    fireEvent.change(screen.getByLabelText(/due label/i), {
      target: { value: 'May 9' },
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
            due: 'May 9',
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

  it('persists card movement when a card is moved to another column', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /move wire auth session cookie flow to in progress/i }));

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        '/api/cards/card-1/move',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({
            columnId: 'column-progress',
            position: 1,
          }),
        }),
      ),
    );

    const progressColumn = screen.getByRole('region', { name: /in progress column/i });
    expect(within(progressColumn).getByText('Wire auth session cookie flow')).toBeInTheDocument();
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

    fireEvent.click(await screen.findByRole('button', { name: /wiki/i }));
    expect(screen.getByRole('heading', { name: /wiki pages/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /platform board/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /settings/i }));
    expect(screen.getByRole('heading', { name: /workspace settings/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /boards/i }));
    expect(screen.getByRole('heading', { name: /platform board/i })).toBeInTheDocument();
  });

  it('edits markdown wiki pages with preview', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /wiki/i }));
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

    fireEvent.click(await screen.findByRole('button', { name: /wiki/i }));
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

  it('previews markdown paragraphs, nested headings, lists, and empty pages', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /wiki/i }));
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

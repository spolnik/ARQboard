import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';
import App from './App';

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
  ],
  wikiPages: [
    { id: 'wiki-1', title: 'Deployment checklist', slug: 'deployment-checklist' },
    { id: 'wiki-2', title: 'Onboarding notes', slug: 'onboarding-notes' },
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
  ],
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
        if (url === '/api/boards/default') {
          return jsonResponse(boardFixture);
        }
        if (url === '/api/cards' && init?.method === 'POST') {
          return jsonResponse(createdCard, 201);
        }
        if (url === '/api/cards/card-1/move' && init?.method === 'PATCH') {
          return jsonResponse(movedBoardFixture);
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
    expect(screen.getAllByText('Deployment checklist').length).toBeGreaterThan(0);
    expect(fetch).toHaveBeenCalledWith('/api/boards/default');
  });

  it('updates the card detail panel when a card is selected', async () => {
    render(<App />);

    fireEvent.click(await screen.findByRole('button', { name: /view card wire auth session cookie flow/i }));

    const detail = screen.getByRole('complementary', { name: /card detail/i });
    expect(within(detail).getByRole('heading', { name: /wire auth session cookie flow/i })).toBeInTheDocument();
    expect(within(detail).getByText(/Owner MS/)).toBeInTheDocument();
    expect(within(detail).getByText(/Priority High/)).toBeInTheDocument();
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
});

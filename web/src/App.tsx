import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import type { DragEndEvent } from '@dnd-kit/core';
import { SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import {
  ArrowRight,
  Bell,
  BookOpen,
  CalendarDays,
  CheckCircle2,
  GripVertical,
  LayoutDashboard,
  MoreHorizontal,
  Plus,
  Search,
  Settings,
  UserRound,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { FormEvent, ReactNode } from 'react';

type View = 'boards' | 'wiki' | 'settings';

type Priority = 'Low' | 'Normal' | 'High' | 'Urgent';

type Card = {
  id: string;
  columnId: string;
  title: string;
  owner: string;
  priority: Priority;
  due: string;
  description: string;
  position: number;
};

type Column = {
  id: string;
  title: string;
  position: number;
  cards: Card[];
};

type WikiPage = {
  id: string;
  title: string;
  slug: string;
};

type Board = {
  id: string;
  name: string;
  slug: string;
  columns: Column[];
  wikiPages: WikiPage[];
};

function App() {
  const [activeView, setActiveView] = useState<View>('boards');
  const [board, setBoard] = useState<Board | null>(null);
  const [selectedCardId, setSelectedCardId] = useState('');
  const [search, setSearch] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newCardTitle, setNewCardTitle] = useState('');
  const [newCardOwner, setNewCardOwner] = useState('');
  const [error, setError] = useState('');

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 6 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  useEffect(() => {
    let cancelled = false;

    async function loadBoard() {
      try {
        const response = await fetch('/api/boards/default');
        if (!response.ok) {
          throw new Error(`Failed to load board: ${response.status}`);
        }
        const nextBoard = normalizeBoard(await response.json());
        if (!cancelled) {
          setBoard(nextBoard);
          setSelectedCardId((current) => current || defaultSelectedCardId(nextBoard));
          setError('');
        }
      } catch {
        if (!cancelled) {
          setError('Could not load the board. Check that migrations have run and the API is available.');
        }
      }
    }

    loadBoard();
    return () => {
      cancelled = true;
    };
  }, []);

  const normalizedSearch = search.trim().toLowerCase();
  const allCards = useMemo(() => board?.columns.flatMap((column) => column.cards) ?? [], [board]);
  const selectedCard = allCards.find((card) => card.id === selectedCardId) ?? allCards[0];

  const filteredColumns = useMemo(() => {
    if (!board) {
      return [];
    }
    if (!normalizedSearch) {
      return board.columns;
    }

    return board.columns.map((column) => ({
      ...column,
      cards: column.cards.filter((card) => cardMatchesSearch(card, normalizedSearch)),
    }));
  }, [board, normalizedSearch]);

  const filteredWikiPages = useMemo(() => {
    const wikiPages = board?.wikiPages ?? [];
    if (!normalizedSearch) {
      return wikiPages;
    }

    return wikiPages.filter((page) => page.title.toLowerCase().includes(normalizedSearch));
  }, [board, normalizedSearch]);

  async function createCard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!board) {
      return;
    }

    const title = newCardTitle.trim();
    if (!title) {
      return;
    }

    const firstColumn = board.columns[0];
    if (!firstColumn) {
      setError('Cannot create a card because this board has no columns.');
      return;
    }

    try {
      const card = await requestJSON<Card>('/api/cards', {
        method: 'POST',
        body: JSON.stringify({
          columnId: firstColumn.id,
          title,
          ownerInitials: ownerInitials(newCardOwner),
        }),
      });

      setBoard((current) => addCardToBoard(current, card));
      setSelectedCardId(card.id);
      setNewCardTitle('');
      setNewCardOwner('');
      setIsCreateOpen(false);
      setActiveView('boards');
      setError('');
    } catch {
      setError('Could not create the card.');
    }
  }

  async function moveCard(cardId: string, columnId: string, position: number) {
    try {
      const nextBoard = await requestJSON<Board>(`/api/cards/${cardId}/move`, {
        method: 'PATCH',
        body: JSON.stringify({ columnId, position }),
      });
      setBoard(normalizeBoard(nextBoard));
      setSelectedCardId(cardId);
      setError('');
    } catch {
      setError('Could not move the card.');
    }
  }

  async function handleDragEnd(event: DragEndEvent) {
    if (!board || !event.over) {
      return;
    }

    const activeCard = findCard(board, String(event.active.id));
    if (!activeCard) {
      return;
    }

    const target = resolveMoveTarget(board, String(event.over.id));
    if (!target) {
      return;
    }

    await moveCard(activeCard.id, target.columnId, target.position);
  }

  return (
    <div className="min-h-screen bg-slate-50 text-slate-950">
      <header className="flex h-14 items-center justify-between border-b border-slate-200 bg-white px-4">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-slate-950 text-sm font-semibold text-white">
            A
          </div>
          <div>
            <p className="text-sm font-semibold leading-4">ARQboard</p>
            <p className="text-xs text-slate-500">Self-hosted workspace</p>
          </div>
        </div>
        <div className="hidden min-w-72 items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-3 py-1.5 md:flex">
          <Search className="h-4 w-4 text-slate-400" aria-hidden="true" />
          <input
            id="workspace-search"
            name="workspaceSearch"
            className="w-full bg-transparent text-sm outline-none placeholder:text-slate-400"
            placeholder="Search cards, pages, comments"
            aria-label="Search workspace"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        <div className="flex items-center gap-2">
          <button
            className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-400"
            type="button"
            onClick={() => setIsCreateOpen(true)}
            disabled={!board}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            New Card
          </button>
          <button
            className="hidden h-9 w-9 items-center justify-center rounded-md border border-slate-200 bg-white text-slate-600 hover:bg-slate-100 sm:inline-flex"
            type="button"
            aria-label="Notifications"
          >
            <Bell className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
      </header>

      <div className="grid min-h-[calc(100vh-3.5rem)] grid-cols-1 lg:grid-cols-[14rem_minmax(0,1fr)_20rem]">
        <aside className="border-b border-slate-200 bg-white p-3 lg:border-b-0 lg:border-r">
          <nav className="flex gap-2 lg:flex-col">
            <NavButton
              active={activeView === 'boards'}
              icon={<LayoutDashboard className="h-4 w-4" aria-hidden="true" />}
              label="Boards"
              onClick={() => setActiveView('boards')}
            />
            <NavButton
              active={activeView === 'wiki'}
              icon={<BookOpen className="h-4 w-4" aria-hidden="true" />}
              label="Wiki"
              onClick={() => setActiveView('wiki')}
            />
            <NavButton
              active={activeView === 'settings'}
              icon={<Settings className="h-4 w-4" aria-hidden="true" />}
              label="Settings"
              onClick={() => setActiveView('settings')}
            />
          </nav>
        </aside>

        {activeView === 'boards' ? (
          <>
            <main className="min-w-0 p-4">
              <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Platform Engineering</p>
                  <h1 className="text-2xl font-semibold tracking-normal">{board?.name ?? 'Platform Board'}</h1>
                </div>
                <div className="flex items-center gap-2 text-sm text-slate-600">
                  <CalendarDays className="h-4 w-4 text-slate-400" aria-hidden="true" />
                  Sprint window Apr 28 - May 12
                </div>
              </div>

              {error ? (
                <p className="mb-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>
              ) : null}

              {!board ? (
                <p className="rounded-md border border-dashed border-slate-200 bg-white px-3 py-8 text-center text-sm text-slate-500">
                  Loading board...
                </p>
              ) : (
                <DndContext sensors={sensors} collisionDetection={closestCorners} onDragEnd={handleDragEnd}>
                  <section className="grid gap-3 xl:grid-cols-3" aria-label="Kanban board">
                    {filteredColumns.map((column) => (
                      <KanbanColumn
                        key={column.id}
                        column={column}
                        allColumns={board.columns}
                        selectedCardId={selectedCard?.id ?? ''}
                        onMoveCard={moveCard}
                        onSelectCard={setSelectedCardId}
                      />
                    ))}
                  </section>
                </DndContext>
              )}
            </main>

            <div className="border-t border-slate-200 bg-white p-4 lg:border-l lg:border-t-0">
              {selectedCard ? (
                <aside className="mb-5" aria-label="Card detail">
                  <div className="mb-2 flex items-center justify-between">
                    <h2 className="text-sm font-semibold">Card detail</h2>
                    <CheckCircle2 className="h-4 w-4 text-emerald-600" aria-hidden="true" />
                  </div>
                  <h3 className="text-lg font-semibold leading-6">{selectedCard.title}</h3>
                  <p className="mt-2 text-sm leading-6 text-slate-600">{selectedCard.description}</p>
                  <div className="mt-3 grid grid-cols-2 gap-2 text-sm text-slate-600">
                    <p>Owner {selectedCard.owner}</p>
                    <p>Priority {selectedCard.priority}</p>
                    <p>Due {selectedCard.due}</p>
                  </div>
                </aside>
              ) : null}

              <aside aria-label="Wiki pages">
                <div className="mb-2 flex items-center justify-between">
                  <h2 className="text-sm font-semibold">Wiki pages</h2>
                  <button className="h-7 w-7 rounded-md text-slate-500 hover:bg-slate-100" type="button" aria-label="Add page">
                    <Plus className="mx-auto h-4 w-4" aria-hidden="true" />
                  </button>
                </div>
                <div className="divide-y divide-slate-200 rounded-md border border-slate-200">
                  {filteredWikiPages.length > 0 ? (
                    filteredWikiPages.map((page) => (
                      <button
                        key={page.id}
                        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-slate-50"
                        type="button"
                      >
                        <span>{page.title}</span>
                        <BookOpen className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
                      </button>
                    ))
                  ) : (
                    <p className="px-3 py-2 text-sm text-slate-500">No matching pages</p>
                  )}
                </div>
              </aside>
            </div>
          </>
        ) : (
          <main className="min-w-0 p-4 lg:col-span-2">
            {activeView === 'wiki' ? (
              <section className="max-w-5xl">
                <div className="mb-4">
                  <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Workspace knowledge</p>
                  <h1 className="text-2xl font-semibold tracking-normal">Wiki pages</h1>
                </div>
                <div className="grid gap-3 md:grid-cols-[16rem_minmax(0,1fr)]">
                  <div className="divide-y divide-slate-200 rounded-md border border-slate-200 bg-white">
                    {filteredWikiPages.map((page) => (
                      <button
                        key={page.id}
                        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-slate-50"
                        type="button"
                      >
                        <span>{page.title}</span>
                        <BookOpen className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
                      </button>
                    ))}
                  </div>
                  <article className="rounded-md border border-slate-200 bg-white p-4">
                    <h2 className="text-lg font-semibold">{filteredWikiPages[0]?.title ?? 'Deployment checklist'}</h2>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      Keep deployment notes close to the board so operational work and implementation work stay connected.
                    </p>
                  </article>
                </div>
              </section>
            ) : (
              <section className="max-w-4xl">
                <div className="mb-4">
                  <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Administration</p>
                  <h1 className="text-2xl font-semibold tracking-normal">Workspace Settings</h1>
                </div>
                <div className="rounded-md border border-slate-200 bg-white p-4">
                  <h2 className="text-sm font-semibold">Local workspace</h2>
                  <p className="mt-2 text-sm leading-6 text-slate-600">
                    Board cards now persist in the configured database. Local development uses SQLite unless
                    `DATABASE_URL` points at PostgreSQL.
                  </p>
                </div>
              </section>
            )}
          </main>
        )}
      </div>

      {isCreateOpen ? (
        <div className="fixed inset-0 z-10 flex items-start justify-center bg-slate-950/20 px-4 py-16">
          <form
            className="w-full max-w-md rounded-md border border-slate-200 bg-white p-4 shadow-lg"
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-card-title"
            onSubmit={createCard}
          >
            <div className="mb-4">
              <h2 id="create-card-title" className="text-lg font-semibold">
                Create Card
              </h2>
              <p className="text-sm text-slate-500">Add a persisted card to the planned column.</p>
            </div>
            <div className="space-y-3">
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="card-title">
                  Card title
                </label>
                <input
                  id="card-title"
                  name="title"
                  className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                  value={newCardTitle}
                  onChange={(event) => setNewCardTitle(event.target.value)}
                  autoFocus
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="card-owner">
                  Owner initials
                </label>
                <input
                  id="card-owner"
                  name="owner"
                  className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm uppercase outline-none focus:border-slate-950"
                  maxLength={3}
                  value={newCardOwner}
                  onChange={(event) => setNewCardOwner(event.target.value)}
                />
              </div>
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                className="h-9 rounded-md border border-slate-200 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                type="button"
                onClick={() => setIsCreateOpen(false)}
              >
                Cancel
              </button>
              <button className="h-9 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800" type="submit">
                Create Card
              </button>
            </div>
          </form>
        </div>
      ) : null}
    </div>
  );
}

function KanbanColumn({
  column,
  allColumns,
  selectedCardId,
  onMoveCard,
  onSelectCard,
}: {
  column: Column;
  allColumns: Column[];
  selectedCardId: string;
  onMoveCard: (cardId: string, columnId: string, position: number) => void;
  onSelectCard: (cardId: string) => void;
}) {
  const { isOver, setNodeRef } = useDroppable({
    id: column.id,
    data: { type: 'column', columnId: column.id },
  });

  return (
    <section
      ref={setNodeRef}
      className={`min-w-0 rounded-md border bg-white ${isOver ? 'border-slate-950' : 'border-slate-200'}`}
      aria-label={`${column.title} column`}
    >
      <div className="flex items-center justify-between border-b border-slate-200 px-3 py-2">
        <div className="flex items-center gap-2">
          <span className={`h-2 w-2 rounded-full ${columnAccent(column.position)}`} />
          <h2 className="text-sm font-semibold">{column.title}</h2>
          <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500">{column.cards.length}</span>
        </div>
        <button
          className="h-7 w-7 rounded-md text-slate-500 hover:bg-slate-100"
          type="button"
          aria-label={`Column actions for ${column.title}`}
        >
          <MoreHorizontal className="mx-auto h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <SortableContext items={column.cards.map((card) => card.id)} strategy={verticalListSortingStrategy}>
        <div className="space-y-2 p-2">
          {column.cards.length > 0 ? (
            column.cards.map((card) => (
              <SortableCard
                key={card.id}
                card={card}
                columns={allColumns}
                selected={selectedCardId === card.id}
                onMoveCard={onMoveCard}
                onSelectCard={onSelectCard}
              />
            ))
          ) : (
            <p className="rounded-md border border-dashed border-slate-200 px-3 py-4 text-sm text-slate-500">No matches</p>
          )}
        </div>
      </SortableContext>
    </section>
  );
}

function SortableCard({
  card,
  columns,
  selected,
  onMoveCard,
  onSelectCard,
}: {
  card: Card;
  columns: Column[];
  selected: boolean;
  onMoveCard: (cardId: string, columnId: string, position: number) => void;
  onSelectCard: (cardId: string) => void;
}) {
  const { attributes, isDragging, listeners, setNodeRef, transform, transition } = useSortable({
    id: card.id,
    data: { type: 'card', card },
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <article
      ref={setNodeRef}
      className={`rounded-md border bg-white p-3 shadow-sm ${selected ? 'border-slate-950 ring-1 ring-slate-950' : 'border-slate-200'} ${
        isDragging ? 'opacity-60' : ''
      }`}
      style={style}
    >
      <div className="mb-3 flex items-start gap-2">
        <button
          className="mt-0.5 h-7 w-7 shrink-0 rounded-md text-slate-400 hover:bg-slate-100 hover:text-slate-700"
          type="button"
          aria-label={`Drag ${card.title}`}
          {...attributes}
          {...listeners}
        >
          <GripVertical className="mx-auto h-4 w-4" aria-hidden="true" />
        </button>
        <button className="min-w-0 flex-1 text-left" type="button" aria-label={`View card ${card.title}`} onClick={() => onSelectCard(card.id)}>
          <span className="mb-2 flex items-start justify-between gap-2">
            <span className="text-sm font-medium leading-5">{card.title}</span>
            <span className={priorityClass(card.priority)}>{card.priority}</span>
          </span>
          <span className="flex items-center justify-between text-xs text-slate-500">
            <span className="inline-flex items-center gap-1">
              <UserRound className="h-3.5 w-3.5" aria-hidden="true" />
              {card.owner}
            </span>
            <span>{card.due}</span>
          </span>
        </button>
      </div>
      <div className="flex justify-end gap-1 border-t border-slate-100 pt-2">
        {columns
          .filter((column) => column.id !== card.columnId)
          .map((column) => (
            <button
              key={column.id}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-slate-700"
              type="button"
              aria-label={`Move ${card.title} to ${column.title}`}
              title={`Move to ${column.title}`}
              onClick={() => onMoveCard(card.id, column.id, column.cards.length)}
            >
              <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          ))}
      </div>
    </article>
  );
}

function NavButton({
  active = false,
  icon,
  label,
  onClick,
}: {
  active?: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      className={`inline-flex h-9 items-center gap-2 rounded-md px-3 text-sm font-medium ${
        active ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'
      }`}
      type="button"
      aria-pressed={active}
      onClick={onClick}
    >
      {icon}
      {label}
    </button>
  );
}

async function requestJSON<T>(url: string, init: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init.headers,
    },
  });
  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

function normalizeBoard(board: Board): Board {
  return {
    ...board,
    columns: [...(board.columns ?? [])]
      .sort((left, right) => left.position - right.position)
      .map((column) => ({
        ...column,
        cards: [...(column.cards ?? [])].sort((left, right) => left.position - right.position),
      })),
    wikiPages: board.wikiPages ?? [],
  };
}

function defaultSelectedCardId(board: Board) {
  const cards = board.columns.flatMap((column) => column.cards);
  return cards.find((card) => card.title === 'Ready for review API shape')?.id ?? cards[0]?.id ?? '';
}

function addCardToBoard(board: Board | null, card: Card): Board | null {
  if (!board) {
    return board;
  }

  return normalizeBoard({
    ...board,
    columns: board.columns.map((column) =>
      column.id === card.columnId
        ? {
            ...column,
            cards: [...column.cards, card],
          }
        : column,
    ),
  });
}

function resolveMoveTarget(board: Board, overId: string) {
  const column = board.columns.find((candidate) => candidate.id === overId);
  if (column) {
    return { columnId: column.id, position: column.cards.length };
  }

  for (const candidate of board.columns) {
    const cardIndex = candidate.cards.findIndex((card) => card.id === overId);
    if (cardIndex >= 0) {
      return { columnId: candidate.id, position: cardIndex };
    }
  }

  return null;
}

function findCard(board: Board, cardId: string) {
  return board.columns.flatMap((column) => column.cards).find((card) => card.id === cardId);
}

function priorityClass(priority: Priority) {
  const base = 'rounded px-1.5 py-0.5 text-xs font-medium';
  switch (priority) {
    case 'Urgent':
      return `${base} bg-rose-50 text-rose-700`;
    case 'High':
      return `${base} bg-amber-50 text-amber-700`;
    case 'Low':
      return `${base} bg-emerald-50 text-emerald-700`;
    default:
      return `${base} bg-slate-100 text-slate-600`;
  }
}

function columnAccent(position: number) {
  switch (position) {
    case 0:
      return 'bg-sky-500';
    case 1:
      return 'bg-amber-500';
    default:
      return 'bg-emerald-500';
  }
}

function cardMatchesSearch(card: Card, search: string) {
  return [card.title, card.owner, card.priority, card.due].some((value) => value.toLowerCase().includes(search));
}

function ownerInitials(value: string) {
  const owner = value.trim().toUpperCase();
  return owner || 'ME';
}

export default App;

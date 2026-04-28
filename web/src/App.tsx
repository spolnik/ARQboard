import {
  Bell,
  BookOpen,
  CalendarDays,
  CheckCircle2,
  LayoutDashboard,
  MoreHorizontal,
  Plus,
  Search,
  Settings,
  UserRound,
} from 'lucide-react';
import { useMemo, useState } from 'react';
import type { FormEvent, ReactNode } from 'react';

type View = 'boards' | 'wiki' | 'settings';

type Priority = 'Low' | 'Normal' | 'High' | 'Urgent';

type Card = {
  id: string;
  title: string;
  owner: string;
  priority: Priority;
  due: string;
  description: string;
};

type Column = {
  id: string;
  title: string;
  accent: string;
  cards: Card[];
};

const columns: Column[] = [
  {
    id: 'todo',
    title: 'Planned',
    accent: 'bg-sky-500',
    cards: [
      {
        id: 'card-1',
        title: 'Wire auth session cookie flow',
        owner: 'MS',
        priority: 'High',
        due: 'Apr 30',
        description: 'Map the session cookie lifecycle, expiry behavior, and local fallback for the first auth pass.',
      },
      {
        id: 'card-2',
        title: 'Draft workspace migration fixtures',
        owner: 'JR',
        priority: 'Normal',
        due: 'May 2',
        description: 'Keep migration examples tiny and readable so the test database can be recreated quickly.',
      },
    ],
  },
  {
    id: 'doing',
    title: 'In progress',
    accent: 'bg-amber-500',
    cards: [
      {
        id: 'card-3',
        title: 'Ready for review API shape',
        owner: 'AK',
        priority: 'Urgent',
        due: 'Today',
        description:
          'Lock the first JSON contracts for boards, columns, cards, and move operations before wiring the UI.',
      },
    ],
  },
  {
    id: 'review',
    title: 'Ready for review',
    accent: 'bg-emerald-500',
    cards: [
      {
        id: 'card-4',
        title: 'Deployment checklist',
        owner: 'JL',
        priority: 'Normal',
        due: 'May 3',
        description: 'Document the minimum local and container checks before a branch is pushed for review.',
      },
    ],
  },
];

const wikiPages = ['Deployment checklist', 'Onboarding notes', 'Incident response'];

function App() {
  const [activeView, setActiveView] = useState<View>('boards');
  const [boardColumns, setBoardColumns] = useState<Column[]>(columns);
  const [selectedCardId, setSelectedCardId] = useState('card-3');
  const [search, setSearch] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newCardTitle, setNewCardTitle] = useState('');
  const [newCardOwner, setNewCardOwner] = useState('');

  const normalizedSearch = search.trim().toLowerCase();
  const allCards = useMemo(() => boardColumns.flatMap((column) => column.cards), [boardColumns]);
  const selectedCard = allCards.find((card) => card.id === selectedCardId) ?? allCards[0];

  const filteredColumns = useMemo(() => {
    if (!normalizedSearch) {
      return boardColumns;
    }

    return boardColumns.map((column) => ({
      ...column,
      cards: column.cards.filter((card) => cardMatchesSearch(card, normalizedSearch)),
    }));
  }, [boardColumns, normalizedSearch]);

  const filteredWikiPages = useMemo(() => {
    if (!normalizedSearch) {
      return wikiPages;
    }

    return wikiPages.filter((page) => page.toLowerCase().includes(normalizedSearch));
  }, [normalizedSearch]);

  function createCard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const title = newCardTitle.trim();
    if (!title) {
      return;
    }

    const owner = newCardOwner.trim().toUpperCase() || 'ME';
    const card: Card = {
      id: `card-${Date.now()}`,
      title,
      owner,
      priority: 'Normal',
      due: 'Later',
      description: 'New card created locally. Persistence will arrive with the board API.',
    };

    setBoardColumns((currentColumns) =>
      currentColumns.map((column) =>
        column.id === 'todo'
          ? {
              ...column,
              cards: [...column.cards, card],
            }
          : column,
      ),
    );
    setSelectedCardId(card.id);
    setNewCardTitle('');
    setNewCardOwner('');
    setIsCreateOpen(false);
    setActiveView('boards');
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
            className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800"
            type="button"
            onClick={() => setIsCreateOpen(true)}
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
                  <h1 className="text-2xl font-semibold tracking-normal">Platform Board</h1>
                </div>
                <div className="flex items-center gap-2 text-sm text-slate-600">
                  <CalendarDays className="h-4 w-4 text-slate-400" aria-hidden="true" />
                  Sprint window Apr 28 - May 12
                </div>
              </div>

              <section className="grid gap-3 xl:grid-cols-3" aria-label="Kanban board">
                {filteredColumns.map((column) => (
                  <div key={column.id} className="min-w-0 rounded-md border border-slate-200 bg-white">
                    <div className="flex items-center justify-between border-b border-slate-200 px-3 py-2">
                      <div className="flex items-center gap-2">
                        <span className={`h-2 w-2 rounded-full ${column.accent}`} />
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
                    <div className="space-y-2 p-2">
                      {column.cards.length > 0 ? (
                        column.cards.map((card) => (
                          <button
                            key={card.id}
                            className={`w-full rounded-md border bg-white p-3 text-left shadow-sm hover:border-slate-400 hover:bg-slate-50 ${
                              selectedCardId === card.id ? 'border-slate-950 ring-1 ring-slate-950' : 'border-slate-200'
                            }`}
                            type="button"
                            aria-label={`View card ${card.title}`}
                            onClick={() => setSelectedCardId(card.id)}
                          >
                            <div className="mb-3 flex items-start justify-between gap-2">
                              <h3 className="text-sm font-medium leading-5">{card.title}</h3>
                              <span className={priorityClass(card.priority)}>{card.priority}</span>
                            </div>
                            <div className="flex items-center justify-between text-xs text-slate-500">
                              <span className="inline-flex items-center gap-1">
                                <UserRound className="h-3.5 w-3.5" aria-hidden="true" />
                                {card.owner}
                              </span>
                              <span>{card.due}</span>
                            </div>
                          </button>
                        ))
                      ) : (
                        <p className="rounded-md border border-dashed border-slate-200 px-3 py-4 text-sm text-slate-500">No matches</p>
                      )}
                    </div>
                  </div>
                ))}
              </section>
            </main>

            <div className="border-t border-slate-200 bg-white p-4 lg:border-l lg:border-t-0">
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
                        key={page}
                        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-slate-50"
                        type="button"
                      >
                        <span>{page}</span>
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
                        key={page}
                        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-slate-50"
                        type="button"
                      >
                        <span>{page}</span>
                        <BookOpen className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
                      </button>
                    ))}
                  </div>
                  <article className="rounded-md border border-slate-200 bg-white p-4">
                    <h2 className="text-lg font-semibold">Deployment checklist</h2>
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
                    Settings are read-only in this prototype. The next backend pass can connect these controls to persisted
                    workspace configuration.
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
              <p className="text-sm text-slate-500">Add a local card to the planned column.</p>
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

function cardMatchesSearch(card: Card, search: string) {
  return [card.title, card.owner, card.priority, card.due].some((value) => value.toLowerCase().includes(search));
}

export default App;

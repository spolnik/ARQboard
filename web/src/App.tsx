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
import type { ReactNode } from 'react';

type Card = {
  id: string;
  title: string;
  owner: string;
  priority: 'Low' | 'Normal' | 'High' | 'Urgent';
  due: string;
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
      },
      {
        id: 'card-2',
        title: 'Draft workspace migration fixtures',
        owner: 'JR',
        priority: 'Normal',
        due: 'May 2',
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
      },
    ],
  },
];

const wikiPages = ['Deployment checklist', 'Onboarding notes', 'Incident response'];

function App() {
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
            className="w-full bg-transparent text-sm outline-none placeholder:text-slate-400"
            placeholder="Search cards, pages, comments"
            aria-label="Search workspace"
          />
        </div>
        <div className="flex items-center gap-2">
          <button
            className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800"
            type="button"
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
            <NavButton active icon={<LayoutDashboard className="h-4 w-4" aria-hidden="true" />} label="Boards" />
            <NavButton icon={<BookOpen className="h-4 w-4" aria-hidden="true" />} label="Wiki" />
            <NavButton icon={<Settings className="h-4 w-4" aria-hidden="true" />} label="Settings" />
          </nav>
        </aside>

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
            {columns.map((column) => (
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
                  {column.cards.map((card) => (
                    <article key={card.id} className="rounded-md border border-slate-200 bg-white p-3 shadow-sm">
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
                    </article>
                  ))}
                </div>
              </div>
            ))}
          </section>
        </main>

        <aside className="border-t border-slate-200 bg-white p-4 lg:border-l lg:border-t-0">
          <div className="mb-5">
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-sm font-semibold">Card detail</h2>
              <CheckCircle2 className="h-4 w-4 text-emerald-600" aria-hidden="true" />
            </div>
            <h3 className="text-lg font-semibold leading-6">Ready for review API shape</h3>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              Lock the first JSON contracts for boards, columns, cards, and move operations before wiring the UI.
            </p>
          </div>

          <div>
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-sm font-semibold">Wiki pages</h2>
              <button className="h-7 w-7 rounded-md text-slate-500 hover:bg-slate-100" type="button" aria-label="Add wiki page">
                <Plus className="mx-auto h-4 w-4" aria-hidden="true" />
              </button>
            </div>
            <div className="divide-y divide-slate-200 rounded-md border border-slate-200">
              {wikiPages.map((page) => (
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
          </div>
        </aside>
      </div>
    </div>
  );
}

function NavButton({ active = false, icon, label }: { active?: boolean; icon: ReactNode; label: string }) {
  return (
    <button
      className={`inline-flex h-9 items-center gap-2 rounded-md px-3 text-sm font-medium ${
        active ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'
      }`}
      type="button"
    >
      {icon}
      {label}
    </button>
  );
}

function priorityClass(priority: Card['priority']) {
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

export default App;

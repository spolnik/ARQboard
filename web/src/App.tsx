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
  ArrowLeft,
  Bell,
  BookOpen,
  CalendarDays,
  CheckCircle2,
  CircleAlert,
  CircleDot,
  ExternalLink,
  FilePlus2,
  Folder,
  LayoutDashboard,
  LogIn,
  LogOut,
  Maximize2,
  MessageSquare,
  Minimize2,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  Pencil,
  Plus,
  Save,
  Search,
  Settings,
  SignalHigh,
  SignalLow,
  UserRound,
} from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useCallback, useEffect, useMemo, useState } from 'react';
import type { FormEvent, MouseEvent, ReactNode } from 'react';

type View = 'boards' | 'wiki' | 'settings' | 'card';

type AuthState = 'loading' | 'authenticated' | 'unauthenticated';

type Priority = 'Low' | 'Normal' | 'High' | 'Urgent';

type CurrentUser = {
  id: string;
  email: string;
  displayName: string;
  isAdmin: boolean;
};

type WorkspaceRole = 'owner' | 'admin' | 'member' | 'viewer';

type WorkspaceMember = {
  id: string;
  workspaceId: string;
  userId: string;
  email: string;
  displayName: string;
  role: WorkspaceRole;
  isAdmin: boolean;
};

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
  bodyMarkdown: string;
};

type CardComment = {
  id: string;
  cardId: string;
  body: string;
  createdAt: string;
};

type ActivityEvent = {
  id: string;
  cardId: string;
  eventType: string;
  summary: string;
  createdAt: string;
};

type CardDetail = {
  card: Card;
  comments: CardComment[];
  activity: ActivityEvent[];
};

type Board = {
  id: string;
  name: string;
  slug: string;
  columns: Column[];
  wikiPages: WikiPage[];
};

type BoardSummary = {
  id: string;
  workspaceId: string;
  name: string;
  slug: string;
  columnCount: number;
  cardCount: number;
};

type CardForm = {
  title: string;
  description: string;
  priority: string;
  ownerInitials: string;
  due: string;
};

type WikiForm = {
  title: string;
  bodyMarkdown: string;
};

type MemberForm = {
  email: string;
  displayName: string;
  password: string;
  role: WorkspaceRole;
};

type WikiTreeNode = {
  key: string;
  label: string;
  page: WikiPage | null;
  children: WikiTreeNode[];
};

const workspaceRoleOptions: WorkspaceRole[] = ['owner', 'admin', 'member', 'viewer'];

function App() {
  const [authState, setAuthState] = useState<AuthState>('loading');
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null);
  const [loginEmail, setLoginEmail] = useState('');
  const [loginPassword, setLoginPassword] = useState('');
  const [loginError, setLoginError] = useState('');
  const [activeView, setActiveView] = useState<View>('boards');
  const [isNavCollapsed, setIsNavCollapsed] = useState(false);
  const [isBoardFullScreen, setIsBoardFullScreen] = useState(false);
  const [isRightRailCollapsed, setIsRightRailCollapsed] = useState(false);
  const [boards, setBoards] = useState<BoardSummary[]>([]);
  const [selectedBoardId, setSelectedBoardId] = useState('');
  const [board, setBoard] = useState<Board | null>(null);
  const [selectedCardId, setSelectedCardId] = useState('');
  const [cardDetail, setCardDetail] = useState<CardDetail | null>(null);
  const [isEditingCard, setIsEditingCard] = useState(false);
  const [cardForm, setCardForm] = useState<CardForm>({
    title: '',
    description: '',
    priority: 'normal',
    ownerInitials: '',
    due: '',
  });
  const [search, setSearch] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newCardTitle, setNewCardTitle] = useState('');
  const [newCardOwner, setNewCardOwner] = useState('');
  const [isCreateBoardOpen, setIsCreateBoardOpen] = useState(false);
  const [newBoardName, setNewBoardName] = useState('');
  const [isCreateColumnOpen, setIsCreateColumnOpen] = useState(false);
  const [newColumnTitle, setNewColumnTitle] = useState('');
  const [renamingColumn, setRenamingColumn] = useState<Column | null>(null);
  const [columnTitle, setColumnTitle] = useState('');
  const [selectedWikiPage, setSelectedWikiPage] = useState<WikiPage | null>(null);
  const [isCreatingWikiPage, setIsCreatingWikiPage] = useState(false);
  const [wikiForm, setWikiForm] = useState<WikiForm>({ title: '', bodyMarkdown: '' });
  const [workspaceMembers, setWorkspaceMembers] = useState<WorkspaceMember[]>([]);
  const [memberForm, setMemberForm] = useState<MemberForm>({
    email: '',
    displayName: '',
    password: '',
    role: 'member',
  });
  const [memberMessage, setMemberMessage] = useState('');
  const [isSavingMember, setIsSavingMember] = useState(false);
  const [newComment, setNewComment] = useState('');
  const [error, setError] = useState('');

  const resetWorkspace = useCallback(() => {
    setBoards([]);
    setSelectedBoardId('');
    setBoard(null);
    setSelectedCardId('');
    setCardDetail(null);
    setIsEditingCard(false);
    setIsBoardFullScreen(false);
    setIsRightRailCollapsed(false);
    setIsCreateOpen(false);
    setIsCreateBoardOpen(false);
    setNewBoardName('');
    setIsCreateColumnOpen(false);
    setNewColumnTitle('');
    setRenamingColumn(null);
    setColumnTitle('');
    setSelectedWikiPage(null);
    setIsCreatingWikiPage(false);
    setWorkspaceMembers([]);
    setMemberForm({ email: '', displayName: '', password: '', role: 'member' });
    setMemberMessage('');
    setIsSavingMember(false);
    setNewComment('');
    setError('');
  }, []);

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

    async function loadCurrentUser() {
      try {
        const response = await fetch('/api/me');
        if (response.status === 401) {
          if (!cancelled) {
            setAuthState('unauthenticated');
            setCurrentUser(null);
          }
          return;
        }
        if (!response.ok) {
          throw new Error(`Failed to load current user: ${response.status}`);
        }
        const user = (await response.json()) as CurrentUser;
        if (!cancelled) {
          setCurrentUser(user);
          setAuthState('authenticated');
          setLoginError('');
        }
      } catch {
        if (!cancelled) {
          setAuthState('unauthenticated');
          setCurrentUser(null);
          setLoginError('Could not check your session.');
        }
      }
    }

    loadCurrentUser();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (authState !== 'authenticated') {
      return;
    }

    let cancelled = false;

    async function loadBoards() {
      try {
        const response = await fetch('/api/boards');
        if (response.status === 401) {
          if (!cancelled) {
            resetWorkspace();
            setAuthState('unauthenticated');
            setCurrentUser(null);
          }
          return;
        }
        if (!response.ok) {
          throw new Error(`Failed to load boards: ${response.status}`);
        }
        const nextBoards = ((await response.json()) as BoardSummary[]).sort((left, right) => left.name.localeCompare(right.name));
        if (!cancelled) {
          setBoards(nextBoards);
          setSelectedBoardId((current) => {
            if (nextBoards.some((candidate) => candidate.id === current)) {
              return current;
            }
            return nextBoards[0]?.id ?? '';
          });
          if (nextBoards.length === 0) {
            setBoard(null);
            setSelectedCardId('');
          }
          setError('');
        }
      } catch {
        if (!cancelled) {
          setError('Could not load boards. Check that migrations have run and the API is available.');
        }
      }
    }

    loadBoards();
    return () => {
      cancelled = true;
    };
  }, [authState, resetWorkspace]);

  useEffect(() => {
    if (authState !== 'authenticated' || !selectedBoardId || board?.id === selectedBoardId) {
      return;
    }

    let cancelled = false;

    async function loadBoard() {
      try {
        const response = await fetch(`/api/boards/${selectedBoardId}`);
        if (response.status === 401) {
          if (!cancelled) {
            resetWorkspace();
            setAuthState('unauthenticated');
            setCurrentUser(null);
          }
          return;
        }
        if (!response.ok) {
          throw new Error(`Failed to load board: ${response.status}`);
        }
        const nextBoard = normalizeBoard(await response.json());
        if (!cancelled) {
          setBoard(nextBoard);
          setSelectedCardId('');
          setCardDetail(null);
          setIsEditingCard(false);
          setSelectedWikiPage(null);
          setIsCreatingWikiPage(false);
          setError('');
        }
      } catch {
        if (!cancelled) {
          setBoard(null);
          setSelectedCardId('');
          setError('Could not load the board. Check that migrations have run and the API is available.');
        }
      }
    }

    loadBoard();
    return () => {
      cancelled = true;
    };
  }, [authState, board?.id, resetWorkspace, selectedBoardId]);

  useEffect(() => {
    if (authState !== 'authenticated' || !selectedCardId) {
      setCardDetail(null);
      return;
    }

    let cancelled = false;
    async function loadSelectedCard() {
      try {
        const detail = normalizeCardDetail(await getJSON<CardDetail>(`/api/cards/${selectedCardId}`));
        if (!cancelled) {
          setCardDetail(detail);
          setCardForm(formFromCard(detail.card));
          setIsEditingCard(false);
          setNewComment('');
          setError('');
        }
      } catch {
        if (!cancelled) {
          setCardDetail(null);
        }
      }
    }

    loadSelectedCard();
    return () => {
      cancelled = true;
    };
  }, [authState, selectedCardId]);

  useEffect(() => {
    if (authState !== 'authenticated' || activeView !== 'settings' || !currentUser?.isAdmin) {
      return;
    }

    let cancelled = false;
    async function loadWorkspaceMembers() {
      try {
        const members = await getJSON<WorkspaceMember[]>('/api/members');
        if (!cancelled) {
          setWorkspaceMembers(sortWorkspaceMembers(members));
          setMemberMessage('');
        }
      } catch {
        if (!cancelled) {
          setMemberMessage('Could not load team members.');
        }
      }
    }

    loadWorkspaceMembers();
    return () => {
      cancelled = true;
    };
  }, [activeView, authState, currentUser?.isAdmin]);

  const normalizedSearch = search.trim().toLowerCase();
  const allCards = useMemo(() => board?.columns.flatMap((column) => column.cards) ?? [], [board]);
  const boardSelectedCard = selectedCardId ? allCards.find((card) => card.id === selectedCardId) : undefined;
  const selectedCard = selectedCardId && cardDetail?.card.id === selectedCardId ? cardDetail.card : boardSelectedCard;
  const boardFullScreen = activeView === 'boards' && isBoardFullScreen;
  const rightRailVisible = activeView === 'boards' && !boardFullScreen && !isRightRailCollapsed;
  const layoutGridColumns = rightRailVisible
    ? isNavCollapsed
      ? 'lg:grid-cols-[4.5rem_minmax(0,1fr)_20rem]'
      : 'lg:grid-cols-[14rem_minmax(0,1fr)_20rem]'
    : isNavCollapsed
      ? 'lg:grid-cols-[4.5rem_minmax(0,1fr)]'
      : 'lg:grid-cols-[14rem_minmax(0,1fr)]';

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

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const email = loginEmail.trim();
    const password = loginPassword;
    if (!email || !password.trim()) {
      setLoginError('Email and password are required.');
      return;
    }

    try {
      const user = await requestJSON<CurrentUser>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });
      setCurrentUser(user);
      setAuthState('authenticated');
      setLoginPassword('');
      setLoginError('');
      setError('');
    } catch {
      setLoginError('Invalid email or password.');
    }
  }

  async function signOut() {
    try {
      await fetch('/api/auth/logout', { method: 'POST' });
    } finally {
      resetWorkspace();
      setCurrentUser(null);
      setAuthState('unauthenticated');
      setLoginPassword('');
    }
  }

  function selectBoard(boardId: string) {
    if (boardId === selectedBoardId) {
      return;
    }

    setSelectedBoardId(boardId);
    setBoard(null);
    setSelectedCardId('');
    setCardDetail(null);
    setIsEditingCard(false);
    setSelectedWikiPage(null);
    setIsCreatingWikiPage(false);
    setError('');
  }

  async function createBoard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const name = newBoardName.trim();
    if (!name) {
      return;
    }

    try {
      const nextBoard = normalizeBoard(
        await requestJSON<Board>('/api/boards', {
          method: 'POST',
          body: JSON.stringify({ name }),
        }),
      );
      setBoards((current) => upsertBoardSummary(current, nextBoard));
      setSelectedBoardId(nextBoard.id);
      setBoard(nextBoard);
      setSelectedCardId('');
      setCardDetail(null);
      setSelectedWikiPage(null);
      setIsCreatingWikiPage(false);
      setNewBoardName('');
      setIsCreateBoardOpen(false);
      showView('boards');
      setError('');
    } catch {
      setError('Could not create the board.');
    }
  }

  async function createColumn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!board) {
      return;
    }

    const title = newColumnTitle.trim();
    if (!title) {
      return;
    }

    try {
      const nextBoard = normalizeBoard(
        await requestJSON<Board>(`/api/boards/${board.id}/columns`, {
          method: 'POST',
          body: JSON.stringify({ title }),
        }),
      );
      setBoard(nextBoard);
      setBoards((current) => upsertBoardSummary(current, nextBoard));
      setSelectedCardId((current) => selectedCardIdForBoard(current, nextBoard));
      setNewColumnTitle('');
      setIsCreateColumnOpen(false);
      setError('');
    } catch {
      setError('Could not add the column.');
    }
  }

  function startRenamingColumn(column: Column) {
    setRenamingColumn(column);
    setColumnTitle(column.title);
  }

  async function renameColumn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!renamingColumn) {
      return;
    }

    const title = columnTitle.trim();
    if (!title) {
      return;
    }

    try {
      const nextBoard = normalizeBoard(
        await requestJSON<Board>(`/api/columns/${renamingColumn.id}`, {
          method: 'PATCH',
          body: JSON.stringify({ title }),
        }),
      );
      setBoard(nextBoard);
      setBoards((current) => upsertBoardSummary(current, nextBoard));
      setSelectedCardId((current) => selectedCardIdForBoard(current, nextBoard));
      setRenamingColumn(null);
      setColumnTitle('');
      setError('');
    } catch {
      setError('Could not rename the column.');
    }
  }

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

      const nextBoard = addCardToBoard(board, card);
      setBoard(nextBoard);
      if (nextBoard) {
        setBoards((current) => upsertBoardSummary(current, nextBoard));
      }
      setSelectedCardId(card.id);
      setIsRightRailCollapsed(false);
      setNewCardTitle('');
      setNewCardOwner('');
      setIsCreateOpen(false);
      showView('boards');
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
      selectCard(cardId);
      setError('');
    } catch {
      setError('Could not move the card.');
    }
  }

  function startEditingCard() {
    if (!selectedCard) {
      return;
    }
    setCardForm(formFromCard(selectedCard));
    setIsEditingCard(true);
  }

  async function updateSelectedCard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedCard) {
      return;
    }

    const payload = {
      title: cardForm.title.trim(),
      description: cardForm.description.trim(),
      priority: cardForm.priority,
      ownerInitials: ownerInitials(cardForm.ownerInitials),
      due: cardForm.due.trim(),
    };
    if (!payload.title) {
      return;
    }

    try {
      const card = await requestJSON<Card>(`/api/cards/${selectedCard.id}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      });
      setBoard((current) => replaceCardInBoard(current, card));
      setCardDetail((current) =>
        current && current.card.id === card.id
          ? {
              ...current,
              card,
            }
          : {
              card,
              comments: [],
              activity: [],
            },
      );
      setCardForm(formFromCard(card));
      setIsEditingCard(false);
      setError('');
    } catch {
      setError('Could not update the card.');
    }
  }

  async function createComment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedCard) {
      return;
    }

    const body = newComment.trim();
    if (!body) {
      return;
    }

    try {
      const detail = normalizeCardDetail(
        await requestJSON<CardDetail>(`/api/cards/${selectedCard.id}/comments`, {
          method: 'POST',
          body: JSON.stringify({ body }),
        }),
      );
      setCardDetail(detail);
      setBoard((current) => replaceCardInBoard(current, detail.card));
      setNewComment('');
      setError('');
    } catch {
      setError('Could not add the comment.');
    }
  }

  async function loadWikiPage(pageID: string) {
    try {
      const page = await getJSON<WikiPage>(`/api/wiki/${pageID}`);
      setSelectedWikiPage(page);
      setWikiForm(formFromWikiPage(page));
      setIsCreatingWikiPage(false);
      showView('wiki');
      setError('');
    } catch {
      setError('Could not load the wiki page.');
    }
  }

  function startCreatingWikiPage() {
    setSelectedWikiPage(null);
    setWikiForm({ title: '', bodyMarkdown: '' });
    setIsCreatingWikiPage(true);
    showView('wiki');
  }

  async function submitWikiPage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const payload = {
      title: wikiForm.title.trim(),
      bodyMarkdown: wikiForm.bodyMarkdown.trim(),
    };
    if (!payload.title) {
      return;
    }

    try {
      const url = isCreatingWikiPage || !selectedWikiPage ? '/api/wiki' : `/api/wiki/${selectedWikiPage.id}`;
      const page = await requestJSON<WikiPage>(url, {
        method: isCreatingWikiPage || !selectedWikiPage ? 'POST' : 'PATCH',
        body: JSON.stringify(payload),
      });
      setBoard((current) => upsertWikiPageInBoard(current, page));
      setSelectedWikiPage(page);
      setWikiForm(formFromWikiPage(page));
      setIsCreatingWikiPage(false);
      setError('');
    } catch {
      setError('Could not save the wiki page.');
    }
  }

  async function submitWorkspaceMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const payload = {
      email: memberForm.email.trim(),
      displayName: memberForm.displayName.trim(),
      password: memberForm.password,
      role: memberForm.role,
    };
    if (!payload.email || !payload.password.trim()) {
      setMemberMessage('Email and temporary password are required.');
      return;
    }

    setIsSavingMember(true);
    try {
      const member = await requestJSON<WorkspaceMember>('/api/members', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      setWorkspaceMembers((current) => sortWorkspaceMembers(upsertWorkspaceMember(current, member)));
      setMemberForm({ email: '', displayName: '', password: '', role: 'member' });
      setMemberMessage('Member added.');
    } catch {
      setMemberMessage('Could not add the member.');
    } finally {
      setIsSavingMember(false);
    }
  }

  async function updateWorkspaceMemberRole(memberID: string, role: WorkspaceRole) {
    try {
      const member = await requestJSON<WorkspaceMember>(`/api/members/${memberID}`, {
        method: 'PATCH',
        body: JSON.stringify({ role }),
      });
      setWorkspaceMembers((current) => sortWorkspaceMembers(upsertWorkspaceMember(current, member)));
      setMemberMessage('Role updated.');
    } catch {
      setMemberMessage('Could not update the role.');
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

  function clearCardSelectionFromBoard(event: MouseEvent<HTMLElement>) {
    const target = event.target as HTMLElement;
    if (target.closest('[data-card-interaction="true"], button, input, textarea, select, a')) {
      return;
    }
    setSelectedCardId('');
    setCardDetail(null);
    setIsEditingCard(false);
  }

  function selectCard(cardId: string) {
    setSelectedCardId(cardId);
    setIsRightRailCollapsed(false);
    setIsBoardFullScreen(false);
    setActiveView('boards');
  }

  function openSelectedCardPage() {
    if (!selectedCard) {
      return;
    }
    setIsBoardFullScreen(false);
    setActiveView('card');
  }

  function showView(view: View) {
    setActiveView(view);
    if (view !== 'boards') {
      setIsBoardFullScreen(false);
    }
  }

  if (authState === 'loading') {
    return <LoadingScreen />;
  }

  if (authState === 'unauthenticated') {
    return (
      <LoginScreen
        email={loginEmail}
        password={loginPassword}
        error={loginError}
        onEmailChange={setLoginEmail}
        onPasswordChange={setLoginPassword}
        onSubmit={signIn}
      />
    );
  }

  return (
    <div className="min-h-screen bg-slate-50 text-slate-950">
      {!boardFullScreen ? (
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
          <div className="hidden items-center gap-2 rounded-md border border-slate-200 bg-white px-2.5 py-1.5 text-sm text-slate-600 sm:flex">
            <UserRound className="h-4 w-4 text-slate-400" aria-hidden="true" />
            <span className="max-w-36 truncate">{currentUser?.displayName || currentUser?.email}</span>
          </div>
          <button
            className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
            type="button"
            onClick={() => setIsCreateBoardOpen(true)}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            New Board
          </button>
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
          <button
            className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
            type="button"
            onClick={signOut}
          >
            <LogOut className="h-4 w-4" aria-hidden="true" />
            Sign out
          </button>
        </div>
        </header>
      ) : null}

      <div
        className={
          boardFullScreen
            ? 'min-h-screen bg-slate-50'
            : `grid min-h-[calc(100vh-3.5rem)] grid-cols-1 ${layoutGridColumns}`
        }
      >
        {!boardFullScreen ? (
        <aside className="border-b border-slate-200 bg-white p-3 lg:border-b-0 lg:border-r">
          <nav
            className={`flex gap-2 lg:flex-col ${isNavCollapsed ? 'lg:items-center' : ''}`}
            aria-label="Primary navigation"
            data-collapsed={isNavCollapsed ? 'true' : 'false'}
          >
            <button
              className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-200 text-slate-600 hover:bg-slate-100"
              type="button"
              aria-label={isNavCollapsed ? 'Expand navigation' : 'Collapse navigation'}
              aria-expanded={!isNavCollapsed}
              onClick={() => setIsNavCollapsed((current) => !current)}
              title={isNavCollapsed ? 'Expand navigation' : 'Collapse navigation'}
            >
              {isNavCollapsed ? <PanelLeftOpen className="h-4 w-4" aria-hidden="true" /> : <PanelLeftClose className="h-4 w-4" aria-hidden="true" />}
            </button>
            <NavButton
              active={activeView === 'boards'}
              icon={<LayoutDashboard className="h-4 w-4" aria-hidden="true" />}
              label="Boards"
              collapsed={isNavCollapsed}
              onClick={() => showView('boards')}
            />
            <NavButton
              active={activeView === 'wiki'}
              icon={<BookOpen className="h-4 w-4" aria-hidden="true" />}
              label="Wiki"
              collapsed={isNavCollapsed}
              onClick={() => showView('wiki')}
            />
            <NavButton
              active={activeView === 'settings'}
              icon={<Settings className="h-4 w-4" aria-hidden="true" />}
              label="Settings"
              collapsed={isNavCollapsed}
              onClick={() => showView('settings')}
            />
          </nav>
        </aside>
        ) : null}

        {activeView === 'boards' ? (
          <>
            <main className={boardFullScreen ? 'min-w-0 p-4' : 'min-w-0 p-4'}>
              <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Platform Engineering</p>
                  <h1 className="text-2xl font-semibold tracking-normal">{board?.name ?? 'Loading board'}</h1>
                </div>
                <div className="flex flex-wrap items-end gap-2">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-slate-600" htmlFor="board-selector">
                      Board
                    </label>
                    <select
                      id="board-selector"
                      className="h-9 min-w-48 rounded-md border border-slate-200 bg-white px-2 text-sm outline-none focus:border-slate-950"
                      value={selectedBoardId}
                      onChange={(event) => selectBoard(event.target.value)}
                    >
                      {boards.length > 0 ? (
                        boards.map((summary) => (
                          <option key={summary.id} value={summary.id}>
                            {summary.name}
                          </option>
                        ))
                      ) : (
                        <option value="">No boards</option>
                      )}
                    </select>
                  </div>
                  <button
                    className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                    type="button"
                    aria-label="Open add column"
                    onClick={() => setIsCreateColumnOpen(true)}
                    disabled={!board}
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    Add Column
                  </button>
                  <button
                    className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                    type="button"
                    aria-label={isRightRailCollapsed ? 'Expand wiki and detail panel' : 'Collapse wiki and detail panel'}
                    aria-expanded={!isRightRailCollapsed}
                    onClick={() => setIsRightRailCollapsed((current) => !current)}
                    disabled={boardFullScreen}
                    title={isRightRailCollapsed ? 'Expand wiki and detail panel' : 'Collapse wiki and detail panel'}
                  >
                    {isRightRailCollapsed ? <PanelRightOpen className="h-4 w-4" aria-hidden="true" /> : <PanelRightClose className="h-4 w-4" aria-hidden="true" />}
                    {isRightRailCollapsed ? 'Show panel' : 'Hide panel'}
                  </button>
                  <button
                    className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                    type="button"
                    aria-label={boardFullScreen ? 'Exit full screen board' : 'Expand kanban board'}
                    aria-pressed={boardFullScreen}
                    onClick={() => setIsBoardFullScreen((current) => !current)}
                  >
                    {boardFullScreen ? <Minimize2 className="h-4 w-4" aria-hidden="true" /> : <Maximize2 className="h-4 w-4" aria-hidden="true" />}
                    {boardFullScreen ? 'Exit full screen' : 'Expand'}
                  </button>
                  <div className="hidden h-9 items-center gap-2 text-sm text-slate-600 md:flex">
                    <CalendarDays className="h-4 w-4 text-slate-400" aria-hidden="true" />
                    Sprint window Apr 28 - May 12
                  </div>
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
                  <div className={boardFullScreen ? 'h-[calc(100vh-7.5rem)] overflow-auto pb-2' : 'overflow-x-auto pb-2'}>
                    <section
                      className={`grid gap-3 ${boardFullScreen ? 'min-h-full min-w-[72rem]' : 'min-w-[64rem]'}`}
                      style={{ gridTemplateColumns: `repeat(${Math.max(filteredColumns.length, 1)}, minmax(15rem, 1fr))` }}
                      aria-label="Kanban board"
                      onClick={clearCardSelectionFromBoard}
                    >
                      {filteredColumns.map((column) => (
                        <KanbanColumn
                          key={column.id}
                          column={column}
                          selectedCardId={selectedCard?.id ?? ''}
                          onSelectCard={selectCard}
                          onRenameColumn={startRenamingColumn}
                        />
                      ))}
                    </section>
                  </div>
                </DndContext>
              )}
            </main>

            {rightRailVisible ? (
            <div className="border-t border-slate-200 bg-white p-4 lg:border-l lg:border-t-0">
              {selectedCard ? (
                <aside className="mb-5" aria-label="Card detail">
                  <div className="mb-2 flex items-center justify-between">
                    <h2 className="text-sm font-semibold">Card detail</h2>
                    <div className="flex items-center gap-1">
                      <button
                        className="inline-flex h-8 items-center gap-1 rounded-md border border-slate-200 px-2 text-xs font-medium text-slate-700 hover:bg-slate-50"
                        type="button"
                        aria-label="Open card page"
                        onClick={openSelectedCardPage}
                      >
                        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                        Open
                      </button>
                      <button
                        className="inline-flex h-8 items-center gap-1 rounded-md border border-slate-200 px-2 text-xs font-medium text-slate-700 hover:bg-slate-50"
                        type="button"
                        onClick={startEditingCard}
                      >
                        <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
                        Edit Card
                      </button>
                      <CheckCircle2 className="h-4 w-4 text-emerald-600" aria-hidden="true" />
                    </div>
                  </div>
                  {isEditingCard ? (
                    <form className="space-y-3" onSubmit={updateSelectedCard}>
                      <div>
                        <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="edit-card-title">
                          Card title
                        </label>
                        <input
                          id="edit-card-title"
                          name="title"
                          className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                          value={cardForm.title}
                          onChange={(event) => setCardForm((current) => ({ ...current, title: event.target.value }))}
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="edit-card-description">
                          Description
                        </label>
                        <textarea
                          id="edit-card-description"
                          name="description"
                          className="min-h-24 w-full resize-y rounded-md border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-950"
                          value={cardForm.description}
                          onChange={(event) => setCardForm((current) => ({ ...current, description: event.target.value }))}
                        />
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="edit-card-priority">
                            Priority
                          </label>
                          <select
                            id="edit-card-priority"
                            name="priority"
                            className="h-9 w-full rounded-md border border-slate-200 px-2 text-sm outline-none focus:border-slate-950"
                            value={cardForm.priority}
                            onChange={(event) => setCardForm((current) => ({ ...current, priority: event.target.value }))}
                          >
                            <option value="low">Low</option>
                            <option value="normal">Normal</option>
                            <option value="high">High</option>
                            <option value="urgent">Urgent</option>
                          </select>
                        </div>
                        <div>
                          <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="edit-card-owner">
                            Owner initials
                          </label>
                          <input
                            id="edit-card-owner"
                            name="owner"
                            className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm uppercase outline-none focus:border-slate-950"
                            maxLength={3}
                            value={cardForm.ownerInitials}
                            onChange={(event) => setCardForm((current) => ({ ...current, ownerInitials: event.target.value }))}
                          />
                        </div>
                      </div>
                      <div>
                        <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="edit-card-due">
                          Due label
                        </label>
                        <input
                          id="edit-card-due"
                          name="due"
                          className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                          value={cardForm.due}
                          onChange={(event) => setCardForm((current) => ({ ...current, due: event.target.value }))}
                        />
                      </div>
                      <div className="flex justify-end gap-2">
                        <button
                          className="h-9 rounded-md border border-slate-200 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                          type="button"
                          onClick={() => setIsEditingCard(false)}
                        >
                          Cancel
                        </button>
                        <button
                          className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800"
                          type="submit"
                        >
                          <Save className="h-4 w-4" aria-hidden="true" />
                          Save Card
                        </button>
                      </div>
                    </form>
                  ) : (
                    <>
                      <h3 className="text-lg font-semibold leading-6">{selectedCard.title}</h3>
                      <p className="mt-2 text-sm leading-6 text-slate-600">{selectedCard.description}</p>
                      <div className="mt-3 grid grid-cols-2 gap-2 text-sm text-slate-600">
                        <p>Owner {selectedCard.owner}</p>
                        <p>Priority {selectedCard.priority}</p>
                        <p>Due {selectedCard.due}</p>
                      </div>
                    </>
                  )}

                  <form className="mt-5 border-t border-slate-200 pt-4" onSubmit={createComment}>
                    <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="new-comment">
                      New comment
                    </label>
                    <textarea
                      id="new-comment"
                      name="newComment"
                      className="min-h-20 w-full resize-y rounded-md border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-950"
                      value={newComment}
                      onChange={(event) => setNewComment(event.target.value)}
                    />
                    <div className="mt-2 flex justify-end">
                      <button
                        className="inline-flex h-8 items-center gap-2 rounded-md bg-slate-950 px-3 text-xs font-medium text-white hover:bg-slate-800"
                        type="submit"
                      >
                        <MessageSquare className="h-3.5 w-3.5" aria-hidden="true" />
                        Add comment
                      </button>
                    </div>
                  </form>

                  <div className="mt-4 space-y-3">
                    <div>
                      <h3 className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Comments</h3>
                      <div className="mt-2 space-y-2">
                        {cardDetail?.comments.length ? (
                          cardDetail.comments.map((comment) => (
                            <p key={comment.id} className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700">
                              {comment.body}
                            </p>
                          ))
                        ) : (
                          <p className="text-sm text-slate-500">No comments yet</p>
                        )}
                      </div>
                    </div>
                    <div>
                      <h3 className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Activity</h3>
                      <div className="mt-2 space-y-1">
                        {cardDetail?.activity.length ? (
                          cardDetail.activity.map((event) => (
                            <p key={event.id} className="text-sm text-slate-600">
                              <span className="font-medium text-slate-800">{event.eventType}</span> {event.summary}
                            </p>
                          ))
                        ) : (
                          <p className="text-sm text-slate-500">No activity yet</p>
                        )}
                      </div>
                    </div>
                  </div>
                </aside>
              ) : null}

              <aside aria-label="Wiki pages">
                <div className="mb-2 flex items-center justify-between">
                  <h2 className="text-sm font-semibold">Wiki pages</h2>
                  <button
                    className="h-7 w-7 rounded-md text-slate-500 hover:bg-slate-100"
                    type="button"
                    aria-label="Add page"
                    onClick={startCreatingWikiPage}
                  >
                    <Plus className="mx-auto h-4 w-4" aria-hidden="true" />
                  </button>
                </div>
                <WikiPageTree pages={filteredWikiPages} selectedPageId={selectedWikiPage?.id} onSelect={loadWikiPage} />
              </aside>
            </div>
            ) : null}
          </>
        ) : (
          <main className="min-w-0 p-4">
            {error ? (
              <p className="mb-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>
            ) : null}
            {activeView === 'card' ? (
              <section className="max-w-5xl" aria-label="Card detail page">
                <button
                  className="mb-4 inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                  type="button"
                  onClick={() => showView('boards')}
                >
                  <ArrowLeft className="h-4 w-4" aria-hidden="true" />
                  Back to board
                </button>

                {selectedCard ? (
                  <div className="space-y-4">
                    <div className="rounded-md border border-slate-200 bg-white p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Card detail</p>
                      <h1 className="mt-1 text-2xl font-semibold tracking-normal">{selectedCard.title}</h1>
                      <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">{selectedCard.description}</p>
                      <div className="mt-4 grid gap-2 text-sm text-slate-600 sm:grid-cols-3">
                        <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2">Owner {selectedCard.owner}</p>
                        <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2">Priority {selectedCard.priority}</p>
                        <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2">Due {selectedCard.due}</p>
                      </div>
                    </div>

                    <div className="grid gap-4 lg:grid-cols-2">
                      <section className="rounded-md border border-slate-200 bg-white p-4" aria-labelledby="card-page-comments-heading">
                        <h2 id="card-page-comments-heading" className="text-sm font-semibold">
                          Comments
                        </h2>
                        <div className="mt-3 space-y-2">
                          {cardDetail?.comments.length ? (
                            cardDetail.comments.map((comment) => (
                              <p key={comment.id} className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700">
                                {comment.body}
                              </p>
                            ))
                          ) : (
                            <p className="text-sm text-slate-500">No comments yet</p>
                          )}
                        </div>
                      </section>

                      <section className="rounded-md border border-slate-200 bg-white p-4" aria-labelledby="card-page-activity-heading">
                        <h2 id="card-page-activity-heading" className="text-sm font-semibold">
                          Activity
                        </h2>
                        <div className="mt-3 space-y-1">
                          {cardDetail?.activity.length ? (
                            cardDetail.activity.map((event) => (
                              <p key={event.id} className="text-sm text-slate-600">
                                <span className="font-medium text-slate-800">{event.eventType}</span> {event.summary}
                              </p>
                            ))
                          ) : (
                            <p className="text-sm text-slate-500">No activity yet</p>
                          )}
                        </div>
                      </section>
                    </div>
                  </div>
                ) : (
                  <article className="rounded-md border border-slate-200 bg-white p-4 text-sm text-slate-600">
                    Select a card from the board to open its full detail page.
                  </article>
                )}
              </section>
            ) : activeView === 'wiki' ? (
              <section className="max-w-5xl">
                <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Workspace knowledge</p>
                    <h1 className="text-2xl font-semibold tracking-normal">Wiki pages</h1>
                  </div>
                  <button
                    className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800"
                    type="button"
                    onClick={startCreatingWikiPage}
                  >
                    <FilePlus2 className="h-4 w-4" aria-hidden="true" />
                    New wiki page
                  </button>
                </div>
                <div className="grid gap-3 md:grid-cols-[16rem_minmax(0,1fr)]">
                  <WikiPageTree pages={filteredWikiPages} selectedPageId={selectedWikiPage?.id} onSelect={loadWikiPage} />
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    {selectedWikiPage || isCreatingWikiPage ? (
                      <form className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]" onSubmit={submitWikiPage}>
                        <div className="space-y-3">
                          <div>
                            <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="wiki-title">
                              Wiki title
                            </label>
                            <input
                              id="wiki-title"
                              name="title"
                              className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                              value={wikiForm.title}
                              onChange={(event) => setWikiForm((current) => ({ ...current, title: event.target.value }))}
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="wiki-body">
                              Markdown body
                            </label>
                            <textarea
                              id="wiki-body"
                              name="bodyMarkdown"
                              className="min-h-72 w-full resize-y rounded-md border border-slate-200 px-3 py-2 font-mono text-sm outline-none focus:border-slate-950"
                              value={wikiForm.bodyMarkdown}
                              onChange={(event) => setWikiForm((current) => ({ ...current, bodyMarkdown: event.target.value }))}
                            />
                          </div>
                          <div className="flex justify-end">
                            <button
                              className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800"
                              type="submit"
                            >
                              <Save className="h-4 w-4" aria-hidden="true" />
                              {isCreatingWikiPage ? 'Create page' : 'Save page'}
                            </button>
                          </div>
                        </div>
                        <article className="min-w-0 rounded-md border border-slate-200 bg-slate-50 p-4" aria-label="Markdown preview">
                          <MarkdownPreview markdown={wikiForm.bodyMarkdown} />
                        </article>
                      </form>
                    ) : (
                      <article className="text-sm leading-6 text-slate-600">
                        <h2 className="text-lg font-semibold text-slate-950">Select a page</h2>
                        <p className="mt-2">
                          Select a page to edit markdown, or create a new runbook directly inside the workspace wiki.
                        </p>
                      </article>
                    )}
                  </div>
                </div>
              </section>
            ) : (
              <section className="max-w-6xl">
                <div className="mb-4">
                  <p className="text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">Administration</p>
                  <h1 className="text-2xl font-semibold tracking-normal">Workspace Settings</h1>
                </div>
                <div className="space-y-4">
                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <h2 className="text-sm font-semibold">Local workspace</h2>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      Board cards now persist in the configured database. Local development uses SQLite unless
                      `DATABASE_URL` points at PostgreSQL.
                    </p>
                  </div>

                  <div className="rounded-md border border-slate-200 bg-white p-4">
                    <div className="mb-4 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <h2 className="text-sm font-semibold">Team members</h2>
                        <p className="text-sm text-slate-500">Manage workspace access and role assignments.</p>
                      </div>
                      <p className="text-xs font-medium uppercase tracking-[0.08em] text-slate-500">{workspaceMembers.length} members</p>
                    </div>

                    {currentUser?.isAdmin ? (
                      <>
                        {memberMessage ? (
                          <p className="mb-3 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700">{memberMessage}</p>
                        ) : null}

                        <div className="overflow-x-auto">
                          <table className="w-full min-w-[42rem] border-collapse text-left text-sm">
                            <thead>
                              <tr className="border-b border-slate-200 text-xs font-semibold uppercase tracking-[0.08em] text-slate-500">
                                <th className="py-2 pr-3">Member</th>
                                <th className="py-2 pr-3">Current role</th>
                                <th className="py-2">Change role</th>
                              </tr>
                            </thead>
                            <tbody>
                              {workspaceMembers.length ? (
                                workspaceMembers.map((member) => (
                                  <tr key={member.id} className="border-b border-slate-100 last:border-b-0">
                                    <td className="py-3 pr-3">
                                      <p className="font-medium text-slate-900">{member.displayName || member.email}</p>
                                      <p className="text-slate-500">{member.email}</p>
                                    </td>
                                    <td className="py-3 pr-3">
                                      <span className="inline-flex rounded-md border border-slate-200 bg-slate-50 px-2 py-1 text-xs font-medium text-slate-700">
                                        {displayWorkspaceRole(member.role)}
                                      </span>
                                      {member.isAdmin ? <span className="ml-2 text-xs text-slate-500">Global admin</span> : null}
                                    </td>
                                    <td className="py-3">
                                      <select
                                        className="h-9 rounded-md border border-slate-200 bg-white px-2 text-sm outline-none focus:border-slate-950"
                                        aria-label={`Role for ${member.email}`}
                                        value={member.role}
                                        onChange={(event) => updateWorkspaceMemberRole(member.id, event.target.value as WorkspaceRole)}
                                      >
                                        {workspaceRoleOptions.map((role) => (
                                          <option key={role} value={role}>
                                            {displayWorkspaceRole(role)}
                                          </option>
                                        ))}
                                      </select>
                                    </td>
                                  </tr>
                                ))
                              ) : (
                                <tr>
                                  <td className="py-6 text-center text-sm text-slate-500" colSpan={3}>
                                    No members loaded yet.
                                  </td>
                                </tr>
                              )}
                            </tbody>
                          </table>
                        </div>

                        <form className="mt-5 grid gap-3 border-t border-slate-200 pt-4 md:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1fr)_10rem_auto]" onSubmit={submitWorkspaceMember}>
                          <div>
                            <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="member-email">
                              Member email
                            </label>
                            <input
                              id="member-email"
                              className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                              value={memberForm.email}
                              onChange={(event) => setMemberForm((current) => ({ ...current, email: event.target.value }))}
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="member-display-name">
                              Display name
                            </label>
                            <input
                              id="member-display-name"
                              className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                              value={memberForm.displayName}
                              onChange={(event) => setMemberForm((current) => ({ ...current, displayName: event.target.value }))}
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="member-password">
                              Temporary password
                            </label>
                            <input
                              id="member-password"
                              type="password"
                              className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                              value={memberForm.password}
                              onChange={(event) => setMemberForm((current) => ({ ...current, password: event.target.value }))}
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium text-slate-700" htmlFor="member-role">
                              New member role
                            </label>
                            <select
                              id="member-role"
                              className="h-9 w-full rounded-md border border-slate-200 bg-white px-2 text-sm outline-none focus:border-slate-950"
                              value={memberForm.role}
                              onChange={(event) => setMemberForm((current) => ({ ...current, role: event.target.value as WorkspaceRole }))}
                            >
                              {workspaceRoleOptions.map((role) => (
                                <option key={role} value={role}>
                                  {displayWorkspaceRole(role)}
                                </option>
                              ))}
                            </select>
                          </div>
                          <div className="flex items-end">
                            <button
                              className="inline-flex h-9 items-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-400"
                              type="submit"
                              disabled={isSavingMember}
                            >
                              <Plus className="h-4 w-4" aria-hidden="true" />
                              Add member
                            </button>
                          </div>
                        </form>
                      </>
                    ) : (
                      <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
                        Admin access is required to manage workspace members.
                      </p>
                    )}
                  </div>
                </div>
              </section>
            )}
          </main>
        )}
      </div>

      {isCreateBoardOpen ? (
        <div className="fixed inset-0 z-10 flex items-start justify-center bg-slate-950/20 px-4 py-16">
          <form
            className="w-full max-w-md rounded-md border border-slate-200 bg-white p-4 shadow-lg"
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-board-title"
            onSubmit={createBoard}
          >
            <div className="mb-4">
              <h2 id="create-board-title" className="text-lg font-semibold">
                Create Board
              </h2>
              <p className="text-sm text-slate-500">Start a new persisted board in this workspace.</p>
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="board-name">
                Board name
              </label>
              <input
                id="board-name"
                name="name"
                className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                value={newBoardName}
                onChange={(event) => setNewBoardName(event.target.value)}
                autoFocus
              />
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                className="h-9 rounded-md border border-slate-200 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                type="button"
                onClick={() => setIsCreateBoardOpen(false)}
              >
                Cancel
              </button>
              <button className="h-9 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800" type="submit">
                Create Board
              </button>
            </div>
          </form>
        </div>
      ) : null}

      {isCreateColumnOpen ? (
        <div className="fixed inset-0 z-10 flex items-start justify-center bg-slate-950/20 px-4 py-16">
          <form
            className="w-full max-w-md rounded-md border border-slate-200 bg-white p-4 shadow-lg"
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-column-title"
            onSubmit={createColumn}
          >
            <div className="mb-4">
              <h2 id="create-column-title" className="text-lg font-semibold">
                Add Column
              </h2>
              <p className="text-sm text-slate-500">Add another workflow state to this board.</p>
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="new-column-title">
                Column title
              </label>
              <input
                id="new-column-title"
                name="title"
                className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                value={newColumnTitle}
                onChange={(event) => setNewColumnTitle(event.target.value)}
                autoFocus
              />
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                className="h-9 rounded-md border border-slate-200 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                type="button"
                onClick={() => setIsCreateColumnOpen(false)}
              >
                Cancel
              </button>
              <button className="h-9 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800" type="submit">
                Add column
              </button>
            </div>
          </form>
        </div>
      ) : null}

      {renamingColumn ? (
        <div className="fixed inset-0 z-10 flex items-start justify-center bg-slate-950/20 px-4 py-16">
          <form
            className="w-full max-w-md rounded-md border border-slate-200 bg-white p-4 shadow-lg"
            role="dialog"
            aria-modal="true"
            aria-labelledby="rename-column-title"
            onSubmit={renameColumn}
          >
            <div className="mb-4">
              <h2 id="rename-column-title" className="text-lg font-semibold">
                Rename Column
              </h2>
              <p className="text-sm text-slate-500">Update this board column title.</p>
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="rename-column-input">
                Column title
              </label>
              <input
                id="rename-column-input"
                name="title"
                className="h-9 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                value={columnTitle}
                onChange={(event) => setColumnTitle(event.target.value)}
                autoFocus
              />
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                className="h-9 rounded-md border border-slate-200 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50"
                type="button"
                onClick={() => {
                  setRenamingColumn(null);
                  setColumnTitle('');
                }}
              >
                Cancel
              </button>
              <button className="h-9 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800" type="submit">
                Save column
              </button>
            </div>
          </form>
        </div>
      ) : null}

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

function LoadingScreen() {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-950">
      <header className="flex h-14 items-center border-b border-slate-200 bg-white px-4">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-slate-950 text-sm font-semibold text-white">A</div>
          <div>
            <p className="text-sm font-semibold leading-4">ARQboard</p>
            <p className="text-xs text-slate-500">Self-hosted workspace</p>
          </div>
        </div>
      </header>
      <main className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4">
        <p className="rounded-md border border-dashed border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">Loading workspace...</p>
      </main>
    </div>
  );
}

function LoginScreen({
  email,
  password,
  error,
  onEmailChange,
  onPasswordChange,
  onSubmit,
}: {
  email: string;
  password: string;
  error: string;
  onEmailChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-950">
      <header className="flex h-14 items-center border-b border-slate-200 bg-white px-4">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-slate-950 text-sm font-semibold text-white">A</div>
          <div>
            <p className="text-sm font-semibold leading-4">ARQboard</p>
            <p className="text-xs text-slate-500">Self-hosted workspace</p>
          </div>
        </div>
      </header>
      <main className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center px-4 py-10">
        <form className="w-full max-w-sm rounded-md border border-slate-200 bg-white p-5 shadow-sm" onSubmit={onSubmit}>
          <div className="mb-5">
            <h1 className="text-xl font-semibold tracking-normal">Sign in to ARQboard</h1>
            <p className="mt-1 text-sm text-slate-500">Use your workspace admin account.</p>
          </div>
          {error ? <p className="mb-4 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p> : null}
          <div className="space-y-3">
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="login-email">
                Email
              </label>
              <input
                id="login-email"
                name="email"
                autoComplete="username"
                className="h-10 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                value={email}
                onChange={(event) => onEmailChange(event.target.value)}
                autoFocus
              />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-slate-700" htmlFor="login-password">
                Password
              </label>
              <input
                id="login-password"
                name="password"
                type="password"
                autoComplete="current-password"
                className="h-10 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-950"
                value={password}
                onChange={(event) => onPasswordChange(event.target.value)}
              />
            </div>
          </div>
          <button className="mt-5 inline-flex h-10 w-full items-center justify-center gap-2 rounded-md bg-slate-950 px-3 text-sm font-medium text-white hover:bg-slate-800" type="submit">
            <LogIn className="h-4 w-4" aria-hidden="true" />
            Sign in
          </button>
        </form>
      </main>
    </div>
  );
}

function KanbanColumn({
  column,
  selectedCardId,
  onSelectCard,
  onRenameColumn,
}: {
  column: Column;
  selectedCardId: string;
  onSelectCard: (cardId: string) => void;
  onRenameColumn: (column: Column) => void;
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
          aria-label={`Rename ${column.title}`}
          title={`Rename ${column.title}`}
          onClick={() => onRenameColumn(column)}
        >
          <Pencil className="mx-auto h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <SortableContext items={column.cards.map((card) => card.id)} strategy={verticalListSortingStrategy}>
        <div className="space-y-2 p-2">
          {column.cards.length > 0 ? (
            column.cards.map((card) => (
              <SortableCard
                key={card.id}
                card={card}
                selected={selectedCardId === card.id}
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
  selected,
  onSelectCard,
}: {
  card: Card;
  selected: boolean;
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
      data-card-interaction="true"
      className={`rounded-md border bg-white shadow-sm ${selected ? 'border-slate-950 ring-1 ring-slate-950' : 'border-slate-200'} ${
        isDragging ? 'opacity-60' : ''
      }`}
      style={style}
    >
      <button
        className="block w-full cursor-grab rounded-md p-3 text-left outline-none focus-visible:ring-2 focus-visible:ring-slate-950 active:cursor-grabbing"
        type="button"
        aria-label={`View card ${card.title}`}
        onClick={() => onSelectCard(card.id)}
        {...attributes}
        {...listeners}
      >
        <span className="mb-3 block">
          <span className="text-xs font-medium uppercase text-slate-400">{card.id.slice(0, 8)}</span>
          <span className="mt-1 block text-sm font-medium leading-5 text-slate-900">{card.title}</span>
        </span>
        <span className="flex flex-wrap items-center gap-1.5 text-xs text-slate-500">
          <span className="inline-flex h-6 items-center gap-1 rounded-md border border-slate-200 bg-white px-1.5" aria-label={`Owner ${card.owner}`}>
            <UserRound className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
            {card.owner}
          </span>
          <span className={priorityChipClass(card.priority)} aria-label={`Priority ${card.priority}`} title={`Priority ${card.priority}`}>
            <PriorityIcon priority={card.priority} />
          </span>
          <span className="inline-flex h-6 items-center gap-1 rounded-md border border-slate-200 bg-white px-1.5" aria-label={`Due ${card.due}`}>
            <CalendarDays className="h-3.5 w-3.5 text-slate-400" aria-hidden="true" />
            <span>{card.due}</span>
          </span>
        </span>
      </button>
    </article>
  );
}

function NavButton({
  active = false,
  collapsed = false,
  icon,
  label,
  onClick,
}: {
  active?: boolean;
  collapsed?: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      className={`inline-flex h-9 items-center gap-2 rounded-md text-sm font-medium ${collapsed ? 'w-9 justify-center px-0' : 'px-3'} ${
        active ? 'bg-slate-950 text-white' : 'text-slate-600 hover:bg-slate-100'
      }`}
      type="button"
      aria-pressed={active}
      title={collapsed ? label : undefined}
      onClick={onClick}
    >
      {icon}
      <span className={collapsed ? 'sr-only' : ''}>{label}</span>
    </button>
  );
}

async function getJSON<T>(url: string): Promise<T> {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
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
    wikiPages: (board.wikiPages ?? []).map((page) => ({
      ...page,
      bodyMarkdown: page.bodyMarkdown ?? '',
    })),
  };
}

function normalizeCardDetail(detail: CardDetail): CardDetail {
  return {
    ...detail,
    comments: detail.comments ?? [],
    activity: detail.activity ?? [],
  };
}

function selectedCardIdForBoard(currentCardId: string, board: Board) {
  if (currentCardId && findCard(board, currentCardId)) {
    return currentCardId;
  }
  return '';
}

function upsertBoardSummary(summaries: BoardSummary[], board: Board): BoardSummary[] {
  const existing = summaries.find((summary) => summary.id === board.id);
  const nextSummary: BoardSummary = {
    id: board.id,
    workspaceId: existing?.workspaceId ?? '',
    name: board.name,
    slug: board.slug,
    columnCount: board.columns.length,
    cardCount: board.columns.reduce((total, column) => total + column.cards.length, 0),
  };

  return [...summaries.filter((summary) => summary.id !== board.id), nextSummary].sort((left, right) => left.name.localeCompare(right.name));
}

function upsertWorkspaceMember(members: WorkspaceMember[], member: WorkspaceMember): WorkspaceMember[] {
  const nextMembers = members.some((candidate) => candidate.id === member.id)
    ? members.map((candidate) => (candidate.id === member.id ? member : candidate))
    : [...members, member];
  return nextMembers;
}

function sortWorkspaceMembers(members: WorkspaceMember[]): WorkspaceMember[] {
  const roleOrder: Record<WorkspaceRole, number> = {
    owner: 0,
    admin: 1,
    member: 2,
    viewer: 3,
  };

  return [...members].sort((left, right) => roleOrder[left.role] - roleOrder[right.role] || left.email.localeCompare(right.email));
}

function displayWorkspaceRole(role: WorkspaceRole) {
  switch (role) {
    case 'owner':
      return 'Owner';
    case 'admin':
      return 'Admin';
    case 'viewer':
      return 'Viewer';
    default:
      return 'Member';
  }
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

function replaceCardInBoard(board: Board | null, card: Card): Board | null {
  if (!board) {
    return board;
  }

  return normalizeBoard({
    ...board,
    columns: board.columns.map((column) => ({
      ...column,
      cards: column.cards.map((candidate) => (candidate.id === card.id ? card : candidate)),
    })),
  });
}

function upsertWikiPageInBoard(board: Board | null, page: WikiPage): Board | null {
  if (!board) {
    return board;
  }

  const exists = board.wikiPages.some((candidate) => candidate.id === page.id);
  const wikiPages = exists
    ? board.wikiPages.map((candidate) => (candidate.id === page.id ? page : candidate))
    : [...board.wikiPages, page];

  return {
    ...board,
    wikiPages: [...wikiPages].sort((left, right) => left.title.localeCompare(right.title)),
  };
}

function formFromCard(card: Card): CardForm {
  return {
    title: card.title,
    description: card.description,
    priority: card.priority.toLowerCase(),
    ownerInitials: card.owner,
    due: card.due,
  };
}

function formFromWikiPage(page: WikiPage): WikiForm {
  return {
    title: page.title,
    bodyMarkdown: page.bodyMarkdown ?? '',
  };
}

export function resolveMoveTarget(board: Board, overId: string) {
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

function priorityChipClass(priority: Priority) {
  const base = 'inline-flex h-6 w-6 items-center justify-center rounded-md border';
  switch (priority) {
    case 'Urgent':
      return `${base} border-rose-200 bg-rose-50 text-rose-600`;
    case 'High':
      return `${base} border-amber-200 bg-amber-50 text-amber-600`;
    case 'Low':
      return `${base} border-emerald-200 bg-emerald-50 text-emerald-600`;
    default:
      return `${base} border-slate-200 bg-white text-slate-500`;
  }
}

function PriorityIcon({ priority }: { priority: Priority }) {
  switch (priority) {
    case 'Urgent':
      return <CircleAlert className="h-3.5 w-3.5" aria-hidden="true" />;
    case 'High':
      return <SignalHigh className="h-3.5 w-3.5" aria-hidden="true" />;
    case 'Low':
      return <SignalLow className="h-3.5 w-3.5" aria-hidden="true" />;
    default:
      return <CircleDot className="h-3.5 w-3.5" aria-hidden="true" />;
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

function WikiPageTree({
  pages,
  selectedPageId,
  onSelect,
}: {
  pages: WikiPage[];
  selectedPageId?: string;
  onSelect: (pageId: string) => void;
}) {
  const tree = buildWikiPageTree(pages);

  return (
    <nav aria-label="Wiki page tree" className="rounded-md border border-slate-200 bg-white p-1">
      {tree.length ? (
        <ul className="space-y-1">
          {tree.map((node) => (
            <WikiPageTreeNodeItem key={node.key} node={node} depth={0} selectedPageId={selectedPageId} onSelect={onSelect} />
          ))}
        </ul>
      ) : (
        <p className="px-2 py-2 text-sm text-slate-500">No matching pages</p>
      )}
    </nav>
  );
}

function WikiPageTreeNodeItem({
  node,
  depth,
  selectedPageId,
  onSelect,
}: {
  node: WikiTreeNode;
  depth: number;
  selectedPageId?: string;
  onSelect: (pageId: string) => void;
}) {
  const paddingLeft = `${0.5 + depth * 0.85}rem`;

  return (
    <li>
      {node.page ? (
        <button
          aria-current={selectedPageId === node.page.id ? 'page' : undefined}
          aria-label={`Open wiki page ${node.page.title}`}
          className={`flex min-h-9 w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-slate-50 ${
            selectedPageId === node.page.id ? 'bg-slate-100 font-medium text-slate-950' : 'text-slate-700'
          }`}
          style={{ paddingLeft }}
          type="button"
          onClick={() => onSelect(node.page!.id)}
        >
          <BookOpen className="h-3.5 w-3.5 shrink-0 text-slate-400" aria-hidden="true" />
          <span className="min-w-0 truncate">{node.label}</span>
        </button>
      ) : (
        <div className="flex min-h-8 items-center gap-2 px-2 py-1 text-xs font-semibold text-slate-500" style={{ paddingLeft }}>
          <Folder className="h-3.5 w-3.5 shrink-0 text-slate-400" aria-hidden="true" />
          <span className="min-w-0 truncate">{node.label}</span>
        </div>
      )}
      {node.children.length ? (
        <ul className="space-y-1">
          {node.children.map((child) => (
            <WikiPageTreeNodeItem key={child.key} node={child} depth={depth + 1} selectedPageId={selectedPageId} onSelect={onSelect} />
          ))}
        </ul>
      ) : null}
    </li>
  );
}

function buildWikiPageTree(pages: WikiPage[]): WikiTreeNode[] {
  const root: WikiTreeNode[] = [];

  for (const page of [...pages].sort((left, right) => left.title.localeCompare(right.title))) {
    const segments = page.title
      .split('/')
      .map((segment) => segment.trim())
      .filter(Boolean);
    const path = segments.length ? segments : [page.title];
    let branch = root;
    let keyPath = '';

    path.forEach((segment, index) => {
      keyPath = keyPath ? `${keyPath}/${segment}` : segment;
      const isLeaf = index === path.length - 1;
      let node = branch.find((candidate) => candidate.label === segment);
      if (!node) {
        node = {
          key: isLeaf ? page.id : `folder:${keyPath}`,
          label: segment,
          page: null,
          children: [],
        };
        branch.push(node);
      }
      if (isLeaf) {
        node.key = page.id;
        node.page = page;
      }
      branch = node.children;
    });
  }

  return root;
}

function MarkdownPreview({ markdown }: { markdown: string }) {
  if (!markdown.trim()) {
    return <p className="text-sm text-slate-500">No content yet</p>;
  }

  return (
    <div className="space-y-3 text-sm leading-6 text-slate-700">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          h1: ({ children }) => <h2 className="text-xl font-semibold text-slate-950">{children}</h2>,
          h2: ({ children }) => <h3 className="text-lg font-semibold text-slate-950">{children}</h3>,
          h3: ({ children }) => <h4 className="text-base font-semibold text-slate-950">{children}</h4>,
          p: ({ children }) => <p>{children}</p>,
          a: ({ children, href }) => (
            <a className="font-medium text-sky-700 underline underline-offset-2" href={href} rel="noreferrer" target="_blank">
              {children}
            </a>
          ),
          ul: ({ children }) => <ul className="list-disc space-y-1 pl-5">{children}</ul>,
          ol: ({ children }) => <ol className="list-decimal space-y-1 pl-5">{children}</ol>,
          li: ({ children }) => <li>{children}</li>,
          blockquote: ({ children }) => <blockquote className="border-l-2 border-slate-300 pl-3 text-slate-600">{children}</blockquote>,
          code: ({ children, className }) => (
            <code className={`${className ?? ''} rounded bg-slate-200 px-1 py-0.5 font-mono text-[0.85em] text-slate-900`}>{children}</code>
          ),
          pre: ({ children }) => <pre className="overflow-x-auto rounded-md bg-slate-950 p-3 text-xs leading-5 text-slate-50">{children}</pre>,
          hr: () => <hr className="border-slate-200" />,
          table: ({ children }) => <table className="w-full border-collapse text-left text-sm">{children}</table>,
          th: ({ children }) => <th className="border border-slate-200 bg-slate-100 px-2 py-1 font-semibold">{children}</th>,
          td: ({ children }) => <td className="border border-slate-200 px-2 py-1">{children}</td>,
        }}
      >
        {markdown}
      </ReactMarkdown>
    </div>
  );
}

export default App;

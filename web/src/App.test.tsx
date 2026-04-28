import { fireEvent, render, screen, within } from '@testing-library/react';
import App from './App';

describe('App', () => {
  it('renders the board and wiki workspace surface', () => {
    render(<App />);

    expect(screen.getByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.getByText('Ready for review')).toBeInTheDocument();
    expect(screen.getAllByText('Deployment checklist').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /new card/i })).toBeInTheDocument();
  });

  it('updates the card detail panel when a card is selected', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: /view card wire auth session cookie flow/i }));

    const detail = screen.getByRole('complementary', { name: /card detail/i });
    expect(within(detail).getByRole('heading', { name: /wire auth session cookie flow/i })).toBeInTheDocument();
    expect(within(detail).getByText(/Owner MS/)).toBeInTheDocument();
    expect(within(detail).getByText(/Priority High/)).toBeInTheDocument();
  });

  it('creates a new planned card in memory', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: /new card/i }));
    fireEvent.change(screen.getByLabelText(/card title/i), {
      target: { value: 'Run local smoke test' },
    });
    fireEvent.change(screen.getByLabelText(/owner initials/i), {
      target: { value: 'QA' },
    });
    fireEvent.click(screen.getByRole('button', { name: /create card/i }));

    const board = screen.getByRole('region', { name: /kanban board/i });
    expect(within(board).getByText('Run local smoke test')).toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: /create card/i })).not.toBeInTheDocument();
    expect(screen.getByText(/Owner QA/)).toBeInTheDocument();
  });

  it('filters cards and wiki pages with workspace search', () => {
    render(<App />);

    fireEvent.change(screen.getByLabelText(/search workspace/i), {
      target: { value: 'deployment' },
    });

    const board = screen.getByRole('region', { name: /kanban board/i });
    const wiki = screen.getByRole('complementary', { name: /wiki pages/i });

    expect(within(board).getByText('Deployment checklist')).toBeInTheDocument();
    expect(within(board).queryByText('Wire auth session cookie flow')).not.toBeInTheDocument();
    expect(within(wiki).getByText('Deployment checklist')).toBeInTheDocument();
    expect(within(wiki).queryByText('Onboarding notes')).not.toBeInTheDocument();
  });

  it('switches between board, wiki, and settings views', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: /wiki/i }));
    expect(screen.getByRole('heading', { name: /wiki pages/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /platform board/i })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /settings/i }));
    expect(screen.getByRole('heading', { name: /workspace settings/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /boards/i }));
    expect(screen.getByRole('heading', { name: /platform board/i })).toBeInTheDocument();
  });
});

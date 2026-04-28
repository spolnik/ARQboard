import { render, screen } from '@testing-library/react';
import App from './App';

describe('App', () => {
  it('renders the board and wiki workspace surface', () => {
    render(<App />);

    expect(screen.getByRole('heading', { name: /platform board/i })).toBeInTheDocument();
    expect(screen.getByText('Ready for review')).toBeInTheDocument();
    expect(screen.getAllByText('Deployment checklist').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /new card/i })).toBeInTheDocument();
  });
});

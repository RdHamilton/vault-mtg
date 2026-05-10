import { render, screen } from '@testing-library/react';
import Setup from './Setup';

describe('Setup', () => {
  it('renders heading and coming soon message', () => {
    render(<Setup />);

    expect(screen.getByRole('heading', { name: /setup/i })).toBeInTheDocument();
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });

  it('displays daemon setup text', () => {
    render(<Setup />);

    expect(screen.getByText(/daemon setup will be available here/i)).toBeInTheDocument();
  });
});

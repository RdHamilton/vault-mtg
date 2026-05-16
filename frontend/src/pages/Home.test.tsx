import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { render } from '../test/utils/testUtils';
import Home from './Home';

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock Clerk's useUser
const mockUseUser = vi.fn(() => ({
  user: { firstName: 'Ray', username: 'rayhamilton' },
  isLoaded: true,
  isSignedIn: true,
}));
vi.mock('@clerk/react', async () => {
  const actual = await vi.importActual('@clerk/react');
  return {
    ...actual,
    useUser: () => mockUseUser(),
  };
});

describe('Home Page (#2005)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockClear();
    mockUseUser.mockReturnValue({
      user: { firstName: 'Ray', username: 'rayhamilton' },
      isLoaded: true,
      isSignedIn: true,
    });
  });

  describe('AC1/AC4: Route and render', () => {
    it('renders the home page', () => {
      render(<Home />);
      expect(screen.getByTestId('home-page')).toBeInTheDocument();
    });

    it('renders a personalised welcome heading', () => {
      render(<Home />);
      expect(screen.getByTestId('home-title')).toHaveTextContent('Welcome back, Ray');
    });

    it('falls back to username when firstName is absent', () => {
      mockUseUser.mockReturnValue({
        user: { firstName: null, username: 'rayhamilton' },
        isLoaded: true,
        isSignedIn: true,
      });
      render(<Home />);
      expect(screen.getByTestId('home-title')).toHaveTextContent('Welcome back, rayhamilton');
    });

    it('uses "Planeswalker" when neither firstName nor username is available', () => {
      mockUseUser.mockReturnValue({
        user: null,
        isLoaded: true,
        isSignedIn: false,
      });
      render(<Home />);
      expect(screen.getByTestId('home-title')).toHaveTextContent('Welcome back, Planeswalker');
    });
  });

  describe('AC2: Feature entry points', () => {
    it('renders the feature cards grid', () => {
      render(<Home />);
      expect(screen.getByTestId('home-features')).toBeInTheDocument();
    });

    it('renders Match History entry point', () => {
      render(<Home />);
      expect(screen.getByTestId('home-feature-match-history')).toBeInTheDocument();
      expect(screen.getByText('Match History')).toBeInTheDocument();
    });

    it('renders Draft entry point', () => {
      render(<Home />);
      expect(screen.getByTestId('home-feature-draft')).toBeInTheDocument();
      expect(screen.getByText('Draft')).toBeInTheDocument();
    });

    it('renders Decks entry point', () => {
      render(<Home />);
      expect(screen.getByTestId('home-feature-decks')).toBeInTheDocument();
      expect(screen.getByText('Decks')).toBeInTheDocument();
    });

    it('renders Collection entry point', () => {
      render(<Home />);
      expect(screen.getByTestId('home-feature-collection')).toBeInTheDocument();
      expect(screen.getByText('Collection')).toBeInTheDocument();
    });

    it('navigates to /match-history when Match History card is clicked', async () => {
      const user = userEvent.setup();
      render(<Home />);
      await user.click(screen.getByTestId('home-feature-match-history'));
      expect(mockNavigate).toHaveBeenCalledWith('/match-history');
    });

    it('navigates to /draft when Draft card is clicked', async () => {
      const user = userEvent.setup();
      render(<Home />);
      await user.click(screen.getByTestId('home-feature-draft'));
      expect(mockNavigate).toHaveBeenCalledWith('/draft');
    });

    it('navigates to /decks when Decks card is clicked', async () => {
      const user = userEvent.setup();
      render(<Home />);
      await user.click(screen.getByTestId('home-feature-decks'));
      expect(mockNavigate).toHaveBeenCalledWith('/decks');
    });

    it('navigates to /collection when Collection card is clicked', async () => {
      const user = userEvent.setup();
      render(<Home />);
      await user.click(screen.getByTestId('home-feature-collection'));
      expect(mockNavigate).toHaveBeenCalledWith('/collection');
    });
  });
});

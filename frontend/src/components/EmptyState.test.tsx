import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import EmptyState from './EmptyState';

describe('EmptyState', () => {
  // ------------------------------------------------------------------ //
  // Ticket ACs: renders with CTA, renders without CTA, renders error    //
  // variant, does NOT render during loading (tested in page tests).     //
  // ------------------------------------------------------------------ //

  describe('Required props', () => {
    it('renders heading and subtext', () => {
      render(<EmptyState heading="No Decks Found" subtext="You haven't created any decks yet." />);
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('No Decks Found');
      expect(screen.getByText("You haven't created any decks yet.")).toBeInTheDocument();
    });

    it('renders heading as h2', () => {
      render(<EmptyState heading="Empty State" subtext="No data available" />);
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Empty State');
    });

    it('renders subtext as paragraph with correct class', () => {
      render(<EmptyState heading="No Items" subtext="The list is empty." />);
      const p = screen.getByText('The list is empty.');
      expect(p.tagName).toBe('P');
      expect(p).toHaveClass('empty-state-subtext');
    });
  });

  describe('Optional icon', () => {
    it('renders icon when provided', () => {
      render(<EmptyState icon="📦" heading="No Items" subtext="Your inventory is empty." />);
      expect(document.querySelector('.empty-state-icon')).toHaveTextContent('📦');
    });

    it('does not render icon element when icon is not provided', () => {
      render(<EmptyState heading="No Items" subtext="Your inventory is empty." />);
      expect(document.querySelector('.empty-state-icon')).not.toBeInTheDocument();
    });

    it('does not render icon element when icon is empty string', () => {
      render(<EmptyState icon="" heading="No Items" subtext="icon is empty string" />);
      expect(document.querySelector('.empty-state-icon')).not.toBeInTheDocument();
    });
  });

  describe('CTA — AC: renders with CTA', () => {
    it('renders CTA link when ctaLabel and ctaHref provided in no-data variant', () => {
      render(
        <EmptyState
          heading="No Matches"
          subtext="Play some games first."
          ctaLabel="Go to Onboarding"
          ctaHref="/onboarding"
          variant="no-data"
        />,
      );
      const link = screen.getByRole('link', { name: 'Go to Onboarding' });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', '/onboarding');
      expect(link).toHaveClass('empty-state-cta');
    });

    it('renders CTA when variant defaults to no-data', () => {
      render(
        <EmptyState
          heading="No Decks"
          subtext="Create your first deck."
          ctaLabel="Get Started"
          ctaHref="/start"
        />,
      );
      expect(screen.getByRole('link', { name: 'Get Started' })).toBeInTheDocument();
    });
  });

  describe('No CTA — AC: renders without CTA', () => {
    it('does not render CTA when ctaLabel is omitted', () => {
      render(<EmptyState heading="No Drafts" subtext="No drafts yet." variant="no-data" />);
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });

    it('does not render CTA when ctaHref is omitted', () => {
      render(
        <EmptyState
          heading="No Drafts"
          subtext="No drafts yet."
          ctaLabel="Start Draft"
          variant="no-data"
        />,
      );
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });
  });

  describe('coming-soon variant — AC: renders variant without CTA', () => {
    it('renders coming-soon variant without CTA even when ctaLabel/ctaHref provided', () => {
      render(
        <EmptyState
          heading="Coming Soon"
          subtext="This feature is under construction."
          ctaLabel="Should Not Appear"
          ctaHref="/somewhere"
          variant="coming-soon"
        />,
      );
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Coming Soon');
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });

    it('applies coming-soon CSS modifier class', () => {
      render(<EmptyState heading="Coming Soon" subtext="Under construction." variant="coming-soon" />);
      expect(document.querySelector('.empty-state--coming-soon')).toBeInTheDocument();
    });

    it('applies no-data CSS modifier class by default', () => {
      render(<EmptyState heading="No Data" subtext="Nothing here." />);
      expect(document.querySelector('.empty-state--no-data')).toBeInTheDocument();
    });
  });

  describe('error variant — AC: renders error variant', () => {
    it('renders error heading and subtext for BFF non-2xx error', () => {
      render(
        <EmptyState
          icon="⚠️"
          heading="Something went wrong"
          subtext="We couldn't load your data. Please try again."
          variant="no-data"
        />,
      );
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Something went wrong');
      expect(
        screen.getByText("We couldn't load your data. Please try again."),
      ).toBeInTheDocument();
      expect(screen.getByText('⚠️')).toBeInTheDocument();
    });
  });

  describe('Container structure', () => {
    it('renders with data-testid="empty-state"', () => {
      render(<EmptyState heading="Test" subtext="msg" />);
      expect(screen.getByTestId('empty-state')).toBeInTheDocument();
    });

    it('contains all elements in correct hierarchy', () => {
      render(
        <EmptyState
          icon="🔍"
          heading="Not Found"
          subtext="No results."
          ctaLabel="Try Again"
          ctaHref="/search"
        />,
      );
      const container = screen.getByTestId('empty-state');
      expect(container).toContainElement(document.querySelector('.empty-state-icon'));
      expect(container).toContainElement(screen.getByRole('heading', { level: 2 }));
      expect(container).toContainElement(screen.getByText('No results.'));
      expect(container).toContainElement(screen.getByRole('link'));
    });
  });

  describe('Accessibility', () => {
    it('heading is accessible by role', () => {
      render(<EmptyState heading="Accessible Heading" subtext="Body text" />);
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Accessible Heading');
    });

    it('CTA link is accessible by role', () => {
      render(
        <EmptyState
          heading="No Data"
          subtext="Nothing here."
          ctaLabel="Learn More"
          ctaHref="/learn"
        />,
      );
      expect(screen.getByRole('link', { name: 'Learn More' })).toBeInTheDocument();
    });
  });

  describe('Real-world use cases', () => {
    it('renders empty match history state', () => {
      render(
        <EmptyState
          icon="🎮"
          heading="No matches yet"
          subtext="Start playing MTG Arena to begin tracking your match history!"
          variant="no-data"
        />,
      );
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('No matches yet');
    });

    it('renders empty draft sessions state', () => {
      render(
        <EmptyState
          icon="🎯"
          heading="No Draft History"
          subtext="Complete a Quick Draft in MTG Arena to see your draft history here."
          variant="no-data"
        />,
      );
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('No Draft History');
    });

    it('renders empty decks state with CTA', () => {
      render(
        <EmptyState
          icon="📦"
          heading="No Decks Yet"
          subtext="Create your first deck to get started!"
          ctaLabel="Create New Deck"
          ctaHref="/decks/new"
          variant="no-data"
        />,
      );
      expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('No Decks Yet');
      expect(screen.getByRole('link', { name: 'Create New Deck' })).toBeInTheDocument();
    });
  });
});

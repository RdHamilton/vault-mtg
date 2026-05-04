import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import CacheDegradedNotice from './CacheDegradedNotice';

describe('CacheDegradedNotice', () => {
  describe('when visible=false', () => {
    it('renders nothing', () => {
      const { container } = render(<CacheDegradedNotice visible={false} />);
      expect(container.firstChild).toBeNull();
    });
  });

  describe('when visible=true', () => {
    it('renders the stale-data message', () => {
      render(<CacheDegradedNotice visible={true} />);
      expect(screen.getByTestId('cache-degraded-notice')).toBeInTheDocument();
      expect(screen.getByText(/ratings data may be stale/i)).toBeInTheDocument();
    });

    it('has role="status" for screen-reader accessibility', () => {
      render(<CacheDegradedNotice visible={true} />);
      expect(screen.getByRole('status')).toBeInTheDocument();
    });

    it('includes a dismiss button', () => {
      render(<CacheDegradedNotice visible={true} />);
      expect(screen.getByRole('button', { name: /dismiss/i })).toBeInTheDocument();
    });

    it('hides the notice after clicking dismiss', () => {
      render(<CacheDegradedNotice visible={true} />);
      const dismissBtn = screen.getByRole('button', { name: /dismiss/i });
      fireEvent.click(dismissBtn);
      expect(screen.queryByTestId('cache-degraded-notice')).not.toBeInTheDocument();
    });
  });
});

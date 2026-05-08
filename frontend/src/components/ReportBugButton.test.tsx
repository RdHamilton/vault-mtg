import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ReportBugButton from './ReportBugButton';

// Hoist mocks so they are available when vi.mock factories run
const { mockOpenDialog, mockGetFeedback, mockUseUser } = vi.hoisted(() => ({
  mockOpenDialog: vi.fn(),
  mockGetFeedback: vi.fn(),
  mockUseUser: vi.fn(),
}));

// Mock @sentry/react
vi.mock('@sentry/react', () => ({
  getFeedback: mockGetFeedback,
}));

// Mock @clerk/react
vi.mock('@clerk/react', () => ({
  useUser: () => mockUseUser(),
}));

describe('ReportBugButton', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetFeedback.mockReturnValue({ openDialog: mockOpenDialog });
  });

  describe('when user is signed in', () => {
    beforeEach(() => {
      mockUseUser.mockReturnValue({
        isSignedIn: true,
        user: {
          emailAddresses: [{ emailAddress: 'ray@example.com' }],
          fullName: 'Ray Hamilton',
          firstName: 'Ray',
          lastName: 'Hamilton',
        },
      });
    });

    it('renders the button', () => {
      render(<ReportBugButton />);
      expect(screen.getByTestId('report-bug-button')).toBeInTheDocument();
      expect(screen.getByText('Report a bug')).toBeInTheDocument();
    });

    it('calls openDialog with user name and email on click', () => {
      render(<ReportBugButton />);
      fireEvent.click(screen.getByTestId('report-bug-button'));
      expect(mockGetFeedback).toHaveBeenCalledTimes(1);
      expect(mockOpenDialog).toHaveBeenCalledWith({
        user: { name: 'Ray Hamilton', email: 'ray@example.com' },
      });
    });

    it('passes undefined for name when fullName is empty', () => {
      mockUseUser.mockReturnValue({
        isSignedIn: true,
        user: {
          emailAddresses: [{ emailAddress: 'ray@example.com' }],
          fullName: '',
          firstName: '',
          lastName: '',
        },
      });
      render(<ReportBugButton />);
      fireEvent.click(screen.getByTestId('report-bug-button'));
      expect(mockOpenDialog).toHaveBeenCalledWith({
        user: { name: undefined, email: 'ray@example.com' },
      });
    });

    it('passes undefined for email when no email addresses', () => {
      mockUseUser.mockReturnValue({
        isSignedIn: true,
        user: {
          emailAddresses: [],
          fullName: 'Ray Hamilton',
          firstName: 'Ray',
          lastName: 'Hamilton',
        },
      });
      render(<ReportBugButton />);
      fireEvent.click(screen.getByTestId('report-bug-button'));
      expect(mockOpenDialog).toHaveBeenCalledWith({
        user: { name: 'Ray Hamilton', email: undefined },
      });
    });

    it('does nothing when Sentry feedback integration is unavailable', () => {
      mockGetFeedback.mockReturnValue(undefined);
      render(<ReportBugButton />);
      // Should not throw
      fireEvent.click(screen.getByTestId('report-bug-button'));
      expect(mockOpenDialog).not.toHaveBeenCalled();
    });
  });

  describe('when user is not signed in', () => {
    beforeEach(() => {
      mockUseUser.mockReturnValue({
        isSignedIn: false,
        user: null,
      });
    });

    it('renders nothing', () => {
      render(<ReportBugButton />);
      expect(screen.queryByTestId('report-bug-button')).not.toBeInTheDocument();
    });
  });
});

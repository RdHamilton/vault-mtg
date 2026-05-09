import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ReportBugButton from './ReportBugButton';

// Hoist mocks so they are available when vi.mock factories run
const { mockCreateForm, mockAppendToDom, mockOpen, mockGetFeedback, mockUseUser } = vi.hoisted(() => ({
  mockCreateForm: vi.fn(),
  mockAppendToDom: vi.fn(),
  mockOpen: vi.fn(),
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
    // Sentry v10 feedback API: createForm() returns Promise<FeedbackDialog>.
    mockCreateForm.mockResolvedValue({
      appendToDom: mockAppendToDom,
      open: mockOpen,
    });
    mockGetFeedback.mockReturnValue({ createForm: mockCreateForm });
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

    it('calls createForm with user name and email, then appends and opens the dialog', async () => {
      render(<ReportBugButton />);
      fireEvent.click(screen.getByTestId('report-bug-button'));
      expect(mockGetFeedback).toHaveBeenCalledTimes(1);
      expect(mockCreateForm).toHaveBeenCalledWith({
        useSentryUser: { name: 'Ray Hamilton', email: 'ray@example.com' },
      });
      await waitFor(() => expect(mockAppendToDom).toHaveBeenCalledTimes(1));
      expect(mockOpen).toHaveBeenCalledTimes(1);
    });

    it('passes empty string for name when fullName is empty', async () => {
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
      await waitFor(() => expect(mockCreateForm).toHaveBeenCalled());
      expect(mockCreateForm).toHaveBeenCalledWith({
        useSentryUser: { name: '', email: 'ray@example.com' },
      });
    });

    it('passes empty string for email when no email addresses', async () => {
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
      await waitFor(() => expect(mockCreateForm).toHaveBeenCalled());
      expect(mockCreateForm).toHaveBeenCalledWith({
        useSentryUser: { name: 'Ray Hamilton', email: '' },
      });
    });

    it('does nothing when Sentry feedback integration is unavailable', async () => {
      mockGetFeedback.mockReturnValue(undefined);
      render(<ReportBugButton />);
      // Should not throw
      fireEvent.click(screen.getByTestId('report-bug-button'));
      expect(mockCreateForm).not.toHaveBeenCalled();
      expect(mockAppendToDom).not.toHaveBeenCalled();
      expect(mockOpen).not.toHaveBeenCalled();
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

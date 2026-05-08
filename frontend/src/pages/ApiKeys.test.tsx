import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import ApiKeysPage from './ApiKeys';

// ---------------------------------------------------------------------------
// Mock @clerk/react — only APIKeys component is needed for this page
// ---------------------------------------------------------------------------
vi.mock('@clerk/react', () => ({
  APIKeys: () => (
    <div data-testid="clerk-api-keys">
      <button data-testid="create-api-key-button">Create API key</button>
      <ul data-testid="api-key-list" />
    </div>
  ),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function renderPage() {
  return render(
    <MemoryRouter>
      <ApiKeysPage />
    </MemoryRouter>
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------
describe('ApiKeysPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page container with data-testid', () => {
    renderPage();
    expect(screen.getByTestId('api-keys-page')).toBeInTheDocument();
  });

  it('renders the page title', () => {
    renderPage();
    expect(screen.getByRole('heading', { name: /api keys/i })).toBeInTheDocument();
  });

  it('renders the description paragraph', () => {
    renderPage();
    expect(
      screen.getByText(/create and manage personal api keys/i)
    ).toBeInTheDocument();
  });

  it('renders the api-keys-content container', () => {
    renderPage();
    expect(screen.getByTestId('api-keys-content')).toBeInTheDocument();
  });

  it('renders the Clerk APIKeys component', () => {
    renderPage();
    expect(screen.getByTestId('clerk-api-keys')).toBeInTheDocument();
  });

  it('Clerk APIKeys component renders a create-key button', () => {
    renderPage();
    expect(screen.getByTestId('create-api-key-button')).toBeInTheDocument();
  });

  it('Clerk APIKeys component renders the key list', () => {
    renderPage();
    expect(screen.getByTestId('api-key-list')).toBeInTheDocument();
  });

  it('description mentions one-time visibility of the full key', () => {
    renderPage();
    expect(
      screen.getByText(/full key is only shown once/i)
    ).toBeInTheDocument();
  });
});

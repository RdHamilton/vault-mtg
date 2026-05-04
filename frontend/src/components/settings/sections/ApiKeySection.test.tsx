import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ApiKeySection } from './ApiKeySection';
import { getApiKey, setApiKey } from '@/services/apiClient';

describe('ApiKeySection', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it('shows "No key" status when no key is stored', () => {
    render(<ApiKeySection />);
    expect(screen.getByTestId('api-key-status')).toHaveTextContent(
      'No key — requests will be unauthenticated'
    );
  });

  it('shows "Key configured" status when a key is already stored', () => {
    setApiKey('existing-key');
    render(<ApiKeySection />);
    expect(screen.getByTestId('api-key-status')).toHaveTextContent('Key configured');
  });

  it('renders the key input and save button', () => {
    render(<ApiKeySection />);
    expect(screen.getByTestId('api-key-input')).toBeInTheDocument();
    expect(screen.getByTestId('api-key-save-button')).toBeInTheDocument();
  });

  it('Save button is disabled when input is empty', () => {
    render(<ApiKeySection />);
    expect(screen.getByTestId('api-key-save-button')).toBeDisabled();
  });

  it('Save button is enabled when input has value', () => {
    render(<ApiKeySection />);
    fireEvent.change(screen.getByTestId('api-key-input'), {
      target: { value: 'abc123' },
    });
    expect(screen.getByTestId('api-key-save-button')).not.toBeDisabled();
  });

  it('saves the key to localStorage when Save is clicked', () => {
    render(<ApiKeySection />);

    fireEvent.change(screen.getByTestId('api-key-input'), {
      target: { value: 'my-api-key' },
    });
    fireEvent.click(screen.getByTestId('api-key-save-button'));

    expect(getApiKey()).toBe('my-api-key');
  });

  it('calls onKeyChange callback after saving', () => {
    const onKeyChange = vi.fn();
    render(<ApiKeySection onKeyChange={onKeyChange} />);

    fireEvent.change(screen.getByTestId('api-key-input'), {
      target: { value: 'cb-key' },
    });
    fireEvent.click(screen.getByTestId('api-key-save-button'));

    expect(onKeyChange).toHaveBeenCalledWith('cb-key');
  });

  it('clears input after saving', () => {
    render(<ApiKeySection />);

    const input = screen.getByTestId('api-key-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'some-key' } });
    fireEvent.click(screen.getByTestId('api-key-save-button'));

    expect(input.value).toBe('');
  });

  it('shows Save button label "Saved!" briefly after saving', () => {
    render(<ApiKeySection />);

    fireEvent.change(screen.getByTestId('api-key-input'), {
      target: { value: 'flash-key' },
    });
    fireEvent.click(screen.getByTestId('api-key-save-button'));

    expect(screen.getByTestId('api-key-save-button')).toHaveTextContent('Saved!');
  });

  it('shows Remove button when a key is stored', () => {
    setApiKey('stored-key');
    render(<ApiKeySection />);
    expect(screen.getByTestId('api-key-clear-button')).toBeInTheDocument();
  });

  it('does NOT show Remove button when no key is stored', () => {
    render(<ApiKeySection />);
    expect(screen.queryByTestId('api-key-clear-button')).not.toBeInTheDocument();
  });

  it('clears the stored key when Remove is clicked', () => {
    setApiKey('key-to-clear');
    render(<ApiKeySection />);

    fireEvent.click(screen.getByTestId('api-key-clear-button'));

    expect(getApiKey()).toBe('');
  });

  it('calls onKeyChange with empty string when Remove is clicked', () => {
    setApiKey('removable-key');
    const onKeyChange = vi.fn();
    render(<ApiKeySection onKeyChange={onKeyChange} />);

    fireEvent.click(screen.getByTestId('api-key-clear-button'));

    expect(onKeyChange).toHaveBeenCalledWith('');
  });

  it('saves the key on Enter key press', () => {
    render(<ApiKeySection />);

    const input = screen.getByTestId('api-key-input');
    fireEvent.change(input, { target: { value: 'enter-key' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(getApiKey()).toBe('enter-key');
  });

  it('ignores Enter press when input is empty', () => {
    render(<ApiKeySection />);

    const input = screen.getByTestId('api-key-input');
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(getApiKey()).toBe('');
  });

  it('trims whitespace from the key before saving', () => {
    render(<ApiKeySection />);

    fireEvent.change(screen.getByTestId('api-key-input'), {
      target: { value: '  trimmed-key  ' },
    });
    fireEvent.click(screen.getByTestId('api-key-save-button'));

    expect(getApiKey()).toBe('trimmed-key');
  });
});

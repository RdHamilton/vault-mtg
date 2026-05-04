import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import Download from './Download';

describe('Download Page', () => {
  it('should render the download page container', () => {
    render(<Download />);
    expect(screen.getByTestId('download-page')).toBeInTheDocument();
  });

  it('should render the DaemonDownload section', () => {
    render(<Download />);
    expect(screen.getByTestId('daemon-download-section')).toBeInTheDocument();
  });

  it('should display the page title', () => {
    render(<Download />);
    expect(screen.getByText('Get Started with MTGA Companion')).toBeInTheDocument();
  });
});

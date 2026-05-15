import type { Meta, StoryObj } from '@storybook/react';
import ErrorState from './ErrorState';
import './ErrorState.css';

/**
 * ErrorState — a molecule that composes an icon, a title, optional error
 * detail, and optional help text into a full error placeholder. Rendered in
 * place of page content when a data fetch fails.
 */
const meta: Meta<typeof ErrorState> = {
  title: 'Molecules/ErrorState',
  component: ErrorState,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    message: {
      control: 'text',
      description: 'Primary error title',
    },
    error: {
      control: 'text',
      description: 'Underlying error — an Error object or a string',
    },
    helpText: {
      control: 'text',
      description: 'Optional guidance shown below the error detail',
    },
  },
};

export default meta;
type Story = StoryObj<typeof ErrorState>;

export const Default: Story = {
  args: {
    message: 'Failed to load your match history',
  },
};

export const WithErrorDetail: Story = {
  args: {
    message: 'Failed to load your match history',
    error: 'Request timed out after 30s',
  },
};

export const WithHelpText: Story = {
  args: {
    message: 'Could not reach the VaultMTG API',
    error: 'NetworkError: connection refused',
    helpText: 'Check your internet connection and try again. If the problem persists, the service may be temporarily unavailable.',
  },
};

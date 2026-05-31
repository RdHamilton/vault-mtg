import { expect, within } from 'storybook/test';
import type { Meta, StoryObj } from '@storybook/react';
import Toast from './Toast';
import './Toast.css';

const meta: Meta<typeof Toast> = {
  title: 'Atoms/Toast',
  component: Toast,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    type: {
      control: { type: 'radio' },
      options: ['success', 'info', 'warning', 'error'],
    },
    duration: { control: { type: 'number' } },
  },
};

export default meta;
type Story = StoryObj<typeof Toast>;

/**
 * Play function: verifies the toast is visible and displays the expected
 * message text. `duration: 9999999` prevents auto-dismiss during the test.
 * Chromatic snapshots this post-render visible state.
 */
export const Success: Story = {
  args: {
    message: 'Match synced successfully.',
    type: 'success',
    duration: 9999999,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const toast = canvas.getByText('Match synced successfully.');
    await expect(toast).toBeVisible();
  },
};

export const Info: Story = {
  args: {
    message: 'Daemon connected. Listening for game events.',
    type: 'info',
    duration: 9999999,
  },
};

export const Warning: Story = {
  args: {
    message: 'Cache is stale — ratings may be outdated.',
    type: 'warning',
    duration: 9999999,
  },
};

export const Error: Story = {
  args: {
    message: 'Failed to sync collection. Check your connection.',
    type: 'error',
    duration: 9999999,
  },
};

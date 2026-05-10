import type { Meta, StoryObj } from '@storybook/react';
import Toast from './Toast';
import './Toast.css';

const meta: Meta<typeof Toast> = {
  title: 'Components/Toast',
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

export const Success: Story = {
  args: {
    message: 'Match synced successfully.',
    type: 'success',
    duration: 9999999,
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

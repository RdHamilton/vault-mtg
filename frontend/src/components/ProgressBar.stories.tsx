import type { Meta, StoryObj } from '@storybook/react';
import ProgressBar from './ProgressBar';
import './ProgressBar.css';

const meta: Meta<typeof ProgressBar> = {
  title: 'Atoms/ProgressBar',
  component: ProgressBar,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
  argTypes: {
    progress: { control: { type: 'range', min: 0, max: 100, step: 1 } },
    size: {
      control: { type: 'radio' },
      options: ['small', 'medium', 'large'],
    },
    variant: {
      control: { type: 'radio' },
      options: ['primary', 'success', 'warning', 'error'],
    },
  },
};

export default meta;
type Story = StoryObj<typeof ProgressBar>;

export const Default: Story = {
  args: {
    progress: 45,
    label: 'Syncing cards',
    showPercentage: true,
    variant: 'primary',
    size: 'medium',
  },
};

export const Complete: Story = {
  args: {
    progress: 100,
    label: 'Sync complete',
    showPercentage: true,
    variant: 'success',
    size: 'medium',
  },
};

export const WithDetail: Story = {
  args: {
    progress: 60,
    label: 'Downloading set data',
    detail: 'DSK — Duskmourn: House of Horror',
    showPercentage: true,
    variant: 'primary',
    size: 'medium',
    estimatedTimeRemaining: 45000,
  },
};

export const WithCancel: Story = {
  args: {
    progress: 30,
    label: 'Syncing collection',
    detail: '3,120 of 10,400 cards processed',
    showCancel: true,
    onCancel: () => {},
    showPercentage: true,
    variant: 'primary',
    size: 'large',
  },
};

export const Indeterminate: Story = {
  args: {
    progress: 0,
    label: 'Connecting to daemon...',
    indeterminate: true,
    showPercentage: false,
    variant: 'primary',
    size: 'medium',
  },
};

export const Warning: Story = {
  args: {
    progress: 75,
    label: 'Cache usage',
    showPercentage: true,
    variant: 'warning',
    size: 'small',
  },
};

import { expect, within } from 'storybook/test';
import type { Meta, StoryObj } from '@storybook/react';
import LoadingButton from './LoadingButton';
import './LoadingButton.css';

const meta: Meta<typeof LoadingButton> = {
  title: 'Atoms/LoadingButton',
  component: LoadingButton,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    variant: {
      control: { type: 'select' },
      options: ['primary', 'danger', 'pause', 'resume', 'recalculate', 'clear-cache', 'default'],
    },
    loading: { control: 'boolean' },
    disabled: { control: 'boolean' },
  },
};

export default meta;
type Story = StoryObj<typeof LoadingButton>;

export const Default: Story = {
  args: {
    loading: false,
    loadingText: 'Saving...',
    onClick: () => {},
    children: 'Save Changes',
    variant: 'primary',
  },
};

/**
 * Play function: verifies the button is disabled and displays the loading text
 * when `loading` is true. Chromatic snapshots this spinner/disabled state.
 */
export const Loading: Story = {
  args: {
    loading: true,
    loadingText: 'Saving...',
    onClick: () => {},
    children: 'Save Changes',
    variant: 'primary',
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const btn = canvas.getByRole('button');
    await expect(btn).toBeDisabled();
    await expect(btn).toHaveTextContent('Saving...');
  },
};

export const Danger: Story = {
  args: {
    loading: false,
    loadingText: 'Deleting...',
    onClick: () => {},
    children: 'Delete Account',
    variant: 'danger',
  },
};

export const DangerLoading: Story = {
  args: {
    loading: true,
    loadingText: 'Deleting...',
    onClick: () => {},
    children: 'Delete Account',
    variant: 'danger',
  },
};

export const Disabled: Story = {
  args: {
    loading: false,
    loadingText: 'Saving...',
    onClick: () => {},
    children: 'Save Changes',
    variant: 'primary',
    disabled: true,
  },
};

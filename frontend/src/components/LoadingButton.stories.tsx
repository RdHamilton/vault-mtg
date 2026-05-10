import type { Meta, StoryObj } from '@storybook/react';
import LoadingButton from './LoadingButton';
import './LoadingButton.css';

const meta: Meta<typeof LoadingButton> = {
  title: 'Components/LoadingButton',
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

export const Loading: Story = {
  args: {
    loading: true,
    loadingText: 'Saving...',
    onClick: () => {},
    children: 'Save Changes',
    variant: 'primary',
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

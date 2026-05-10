import type { Meta, StoryObj } from '@storybook/react';
import LoadingSpinner from './LoadingSpinner';
import './LoadingSpinner.css';

const meta: Meta<typeof LoadingSpinner> = {
  title: 'Components/LoadingSpinner',
  component: LoadingSpinner,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    size: {
      control: { type: 'radio' },
      options: ['small', 'medium', 'large'],
    },
  },
};

export default meta;
type Story = StoryObj<typeof LoadingSpinner>;

export const Default: Story = {
  args: {
    message: 'Loading...',
    size: 'medium',
  },
};

export const Small: Story = {
  args: {
    message: 'Loading data...',
    size: 'small',
  },
};

export const Large: Story = {
  args: {
    message: 'Syncing your collection...',
    size: 'large',
  },
};

export const NoMessage: Story = {
  args: {
    message: '',
    size: 'medium',
  },
};

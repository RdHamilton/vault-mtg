import type { Meta, StoryObj } from '@storybook/react';
import EmptyState from './EmptyState';
import './EmptyState.css';

const meta: Meta<typeof EmptyState> = {
  title: 'Components/EmptyState',
  component: EmptyState,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    variant: {
      control: { type: 'radio' },
      options: ['no-data', 'coming-soon'],
    },
  },
};

export default meta;
type Story = StoryObj<typeof EmptyState>;

export const NoData: Story = {
  args: {
    heading: 'No matches found',
    subtext: 'Play your first game in MTG Arena and your match history will appear here.',
    variant: 'no-data',
  },
};

export const NoDataWithCTA: Story = {
  args: {
    heading: 'No matches found',
    subtext: 'Play your first game in MTG Arena and your match history will appear here.',
    variant: 'no-data',
    ctaLabel: 'Get Started',
    ctaHref: '/setup',
  },
};

export const NoDataWithIcon: Story = {
  args: {
    icon: '🃏',
    heading: 'No cards in collection',
    subtext: 'Your collection will sync automatically once the daemon is connected.',
    variant: 'no-data',
    ctaLabel: 'Go to Setup',
    ctaHref: '/setup',
  },
};

export const ComingSoon: Story = {
  args: {
    icon: '🚀',
    heading: 'Coming Soon',
    subtext: 'This feature is under construction. Check back in a future update.',
    variant: 'coming-soon',
  },
};

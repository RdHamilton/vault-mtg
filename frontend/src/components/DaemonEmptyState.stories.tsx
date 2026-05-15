import type { Meta, StoryObj } from '@storybook/react';
import DaemonEmptyState from './DaemonEmptyState';

const meta: Meta<typeof DaemonEmptyState> = {
  title: 'Components/DaemonEmptyState',
  component: DaemonEmptyState,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof DaemonEmptyState>;

export const MatchHistory: Story = {
  args: {
    page: 'match_history',
    heading: 'No matches yet',
    subtext:
      'Install and connect the VaultMTG daemon to start tracking your MTG Arena matches automatically.',
  },
};

export const Collection: Story = {
  args: {
    page: 'collection',
    heading: 'Collection not synced',
    subtext:
      'Your card collection will appear here once the VaultMTG daemon is running and connected.',
  },
};

export const Decks: Story = {
  args: {
    page: 'decks',
    heading: 'No decks found',
    subtext:
      'Connect the VaultMTG daemon to sync your MTG Arena deck list automatically.',
  },
};

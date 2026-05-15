import type { Meta, StoryObj } from '@storybook/react';
import { CFBRatingBadge } from './CFBRatingBadge';
import './CFBRatingBadge.css';

/**
 * CFBRatingBadge — an atom that renders a ChannelFireball limited rating as a
 * colored chip. It takes a numeric rating (0.0–5.0) and shows either the
 * letter grade or the raw number.
 */
const meta: Meta<typeof CFBRatingBadge> = {
  title: 'Atoms/CFBRatingBadge',
  component: CFBRatingBadge,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    rating: {
      control: { type: 'range', min: 0, max: 5, step: 0.1 },
      description: 'CFB limited rating on a 0.0–5.0 scale',
    },
    size: {
      control: { type: 'radio' },
      options: ['small', 'medium', 'large'],
      description: 'Badge size variant',
    },
    showLabel: {
      control: 'boolean',
      description: 'Whether to render the "CFB" label prefix',
    },
    showNumeric: {
      control: 'boolean',
      description: 'Show the numeric rating instead of the letter grade',
    },
    commentary: {
      control: 'text',
      description: 'Optional tooltip text shown on hover',
    },
  },
};

export default meta;
type Story = StoryObj<typeof CFBRatingBadge>;

export const Default: Story = {
  args: {
    rating: 3.8,
    size: 'medium',
    showLabel: true,
    showNumeric: false,
  },
};

export const TopRated: Story = {
  args: {
    rating: 4.9,
    size: 'medium',
    showLabel: true,
    commentary: 'Bomb — first-pickable in any deck.',
  },
};

export const LowRated: Story = {
  args: {
    rating: 0.6,
    size: 'medium',
    showLabel: true,
  },
};

export const Numeric: Story = {
  args: {
    rating: 3.25,
    size: 'medium',
    showLabel: true,
    showNumeric: true,
  },
};

export const Small: Story = {
  args: {
    rating: 2.4,
    size: 'small',
    showLabel: false,
  },
};

export const Large: Story = {
  args: {
    rating: 4.3,
    size: 'large',
    showLabel: true,
  },
};

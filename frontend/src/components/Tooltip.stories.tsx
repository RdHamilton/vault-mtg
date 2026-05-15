import type { Meta, StoryObj } from '@storybook/react';
import Tooltip from './Tooltip';
import './Tooltip.css';

/**
 * Tooltip — an atom that shows a small hint bubble when its child is hovered
 * or focused. The bubble is hidden until interaction, so each story renders a
 * visible trigger element; hover (or tab to) the trigger in the canvas to see
 * the tooltip.
 */
const meta: Meta<typeof Tooltip> = {
  title: 'Atoms/Tooltip',
  component: Tooltip,
  parameters: {
    // Generous padding so the tooltip bubble has room to render in all
    // four positions without being clipped by the canvas edge.
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    content: {
      control: 'text',
      description: 'Text shown inside the tooltip bubble',
    },
    position: {
      control: { type: 'radio' },
      options: ['top', 'bottom', 'left', 'right'],
      description: 'Placement of the bubble relative to the trigger',
    },
    delay: {
      control: { type: 'number' },
      description: 'Hover delay in milliseconds before the bubble appears',
    },
  },
  decorators: [
    (Story) => (
      <div style={{ padding: '4rem' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof Tooltip>;

export const Top: Story = {
  args: {
    content: 'Win rate over your last 20 games.',
    position: 'top',
    children: <button type="button">Hover me</button>,
  },
};

export const Bottom: Story = {
  args: {
    content: 'Synced 2 minutes ago.',
    position: 'bottom',
    children: <button type="button">Hover me</button>,
  },
};

export const Left: Story = {
  args: {
    content: 'CFB limited rating.',
    position: 'left',
    children: <button type="button">Hover me</button>,
  },
};

export const Right: Story = {
  args: {
    content: 'Only counts ranked matches.',
    position: 'right',
    children: <button type="button">Hover me</button>,
  },
};

export const NoDelay: Story = {
  args: {
    content: 'Appears instantly.',
    position: 'top',
    delay: 0,
    children: <button type="button">Hover me</button>,
  },
};

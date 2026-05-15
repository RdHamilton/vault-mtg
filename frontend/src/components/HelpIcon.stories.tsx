import type { Meta, StoryObj } from '@storybook/react';
import HelpIcon from './HelpIcon';
import './HelpIcon.css';

/**
 * HelpIcon — an atom rendering a "?" button that opens a contextual help
 * popover on click. The popover is closed by default; click the icon in the
 * canvas to open it. Stories cover the size variants and a content-rich
 * example.
 */
const meta: Meta<typeof HelpIcon> = {
  title: 'Atoms/HelpIcon',
  component: HelpIcon,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    title: {
      control: 'text',
      description: 'Heading shown at the top of the popover',
    },
    position: {
      control: { type: 'radio' },
      options: ['top', 'bottom', 'left', 'right'],
      description: 'Placement of the popover relative to the icon',
    },
    size: {
      control: { type: 'radio' },
      options: ['small', 'medium', 'large'],
      description: 'Size of the help icon button',
    },
  },
  decorators: [
    (Story) => (
      <div style={{ padding: '6rem' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof HelpIcon>;

export const Default: Story = {
  args: {
    title: 'Win rate',
    size: 'small',
    position: 'bottom',
    children: 'Your win rate is the percentage of matches you have won out of all matches played in the selected period.',
  },
};

export const Medium: Story = {
  args: {
    title: 'Draft grade',
    size: 'medium',
    position: 'bottom',
    children: 'Draft grades score your pick quality against the community consensus for each pack.',
  },
};

export const Large: Story = {
  args: {
    title: 'Metagame share',
    size: 'large',
    position: 'right',
    children: 'Metagame share estimates how common an archetype is in the current ranked ladder.',
  },
};

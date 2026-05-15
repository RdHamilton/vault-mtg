import type { Meta, StoryObj } from '@storybook/react';
import EnvBadge from './EnvBadge';
import './EnvBadge.css';

/**
 * EnvBadge — an atom that renders a small environment chip in non-production
 * builds (development, preview, staging). It is hidden entirely in production.
 *
 * The component takes no props: its label is derived from `import.meta.env`
 * (`VITE_ENV_LABEL` or the Vite `MODE`). Inside Storybook the build mode is
 * `development`, so the badge renders with the "development" styling. The
 * `Staging` and `Preview` stories below illustrate the other variants by
 * rendering the `.env-badge--*` classes directly — useful as a visual
 * reference and a stable Chromatic snapshot for each variant.
 */
const meta: Meta<typeof EnvBadge> = {
  title: 'Atoms/EnvBadge',
  component: EnvBadge,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof EnvBadge>;

/** The live component as it renders inside Storybook (Vite MODE = development). */
export const Development: Story = {};

/** Visual reference for the staging variant styling. */
export const Staging: Story = {
  render: () => (
    <span className="env-badge env-badge--staging" data-testid="env-badge">
      staging
    </span>
  ),
};

/** Visual reference for the preview-deployment variant styling. */
export const Preview: Story = {
  render: () => (
    <span className="env-badge env-badge--preview" data-testid="env-badge">
      preview
    </span>
  ),
};

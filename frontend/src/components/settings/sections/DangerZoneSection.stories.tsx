import { expect, userEvent, within } from 'storybook/test';
import type { Meta, StoryObj } from '@storybook/react';
import { DangerZoneSection } from './DangerZoneSection';

/**
 * DangerZoneSection — Settings accordion section that lets the authenticated
 * user uninstall the local VaultMTG daemon.
 *
 * Extracted from DataRecoverySection in #2027 so that log-replay (Data
 * Recovery) and daemon lifecycle (Danger Zone) are distinct top-level concerns.
 *
 * The component accepts an `onUninstallDaemon` prop (REST API adapter pattern)
 * so stories control the full interaction lifecycle without a live BFF.
 */
const meta: Meta<typeof DangerZoneSection> = {
  title: 'Organisms/DangerZoneSection',
  component: DangerZoneSection,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
  argTypes: {
    isConnected: {
      control: 'boolean',
      description: 'Whether the daemon is currently connected. Uninstall is disabled when false.',
    },
  },
};

export default meta;
type Story = StoryObj<typeof DangerZoneSection>;

/**
 * Default idle state — daemon is connected, uninstall button is enabled.
 */
export const Connected: Story = {
  args: {
    isConnected: true,
    onUninstallDaemon: () =>
      new Promise((resolve) =>
        setTimeout(
          () =>
            resolve(
              'Daemon uninstalled. On macOS, drag VaultMTG to the Trash to remove the app bundle.',
            ),
          1500,
        ),
      ),
  },
};

/**
 * Play function: clicks "Uninstall VaultMTG Daemon" and verifies the
 * confirmation panel (checkbox + Confirm Uninstall button) is rendered.
 * Chromatic will snapshot this post-interaction state.
 */
export const ConfirmationOpen: Story = {
  args: {
    isConnected: true,
    // Never resolves — keeps the story in the confirmation state so Chromatic
    // can snapshot the confirmation panel without a race against the success panel.
    onUninstallDaemon: () => new Promise(() => {}),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);

    // Click the initial "Uninstall VaultMTG Daemon" danger button.
    const uninstallBtn = canvas.getByTestId('danger-zone-uninstall-button');
    await userEvent.click(uninstallBtn);

    // Confirmation panel must now be visible.
    await expect(canvas.getByRole('button', { name: /confirm uninstall/i })).toBeInTheDocument();
    await expect(canvas.getByRole('checkbox')).toBeInTheDocument();
  },
};

/**
 * Daemon is offline — the uninstall button is disabled with a hint.
 */
export const Disconnected: Story = {
  args: {
    isConnected: false,
    onUninstallDaemon: () => Promise.resolve(''),
  },
};

/**
 * No `onUninstallDaemon` prop — the entire section is hidden.
 * Use this when the platform does not support daemon uninstall.
 */
export const Hidden: Story = {
  args: {
    isConnected: true,
    onUninstallDaemon: undefined,
  },
};

/**
 * Simulates the backend returning an error during uninstall.
 * Click "Uninstall VaultMTG Daemon" → "Confirm Uninstall" to trigger.
 */
export const UninstallError: Story = {
  args: {
    isConnected: true,
    onUninstallDaemon: () =>
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Daemon did not respond to shutdown signal.')), 1000),
      ),
  },
};

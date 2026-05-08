import { APIKeys } from '@clerk/react';
import './ApiKeys.css';

/**
 * API Keys page — lets authenticated users create, view, and revoke Clerk API keys.
 * Uses the Clerk built-in <APIKeys /> component which handles all key management UI.
 * Route: /api-keys (protected via ProtectedRoute in App.tsx)
 */
const ApiKeysPage = () => {
  return (
    <div className="page-container" data-testid="api-keys-page">
      <div className="api-keys-header">
        <h1 className="page-title">API Keys</h1>
        <p className="api-keys-description">
          Create and manage personal API keys for programmatic access to VaultMTG.
          Copy each key when it is created — the full key is only shown once.
        </p>
      </div>

      <div className="api-keys-content" data-testid="api-keys-content">
        <APIKeys />
      </div>
    </div>
  );
};

export default ApiKeysPage;

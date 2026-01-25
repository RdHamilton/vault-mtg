import { useState, useEffect } from 'react';
import { decks as decksApi, cards as cardsApi } from '@/services/api';
import type { DeckPermutation, DeckPermutationDiff } from '@/services/api/decks';
import LoadingSpinner from './LoadingSpinner';
import './DeckHistoryModal.css';

interface DeckHistoryModalProps {
  deckId: string;
  deckName: string;
  isOpen: boolean;
  onClose: () => void;
  onRestore?: () => void;
}

function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export default function DeckHistoryModal({
  deckId,
  deckName,
  isOpen,
  onClose,
  onRestore,
}: DeckHistoryModalProps) {
  const [permutations, setPermutations] = useState<DeckPermutation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedPermId, setSelectedPermId] = useState<number | null>(null);
  const [diff, setDiff] = useState<DeckPermutationDiff | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [editingName, setEditingName] = useState<number | null>(null);
  const [newName, setNewName] = useState('');
  const [cardNames, setCardNames] = useState<Record<number, string>>({});

  // Load permutations
  useEffect(() => {
    if (!isOpen) return;

    const loadPermutations = async () => {
      setLoading(true);
      setError(null);
      try {
        const data = await decksApi.getDeckPermutations(deckId);
        // Sort by version number descending (newest first)
        const sorted = [...data].sort((a, b) => b.versionNumber - a.versionNumber);
        setPermutations(sorted);

        // Select the current permutation by default, or the latest if none is current
        const currentPerm = sorted.find((p) => p.isCurrent);
        if (currentPerm) {
          setSelectedPermId(currentPerm.id);
        } else if (sorted.length > 0) {
          setSelectedPermId(sorted[0].id);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load deck history');
      } finally {
        setLoading(false);
      }
    };

    loadPermutations();
  }, [deckId, isOpen]);

  // Load diff when selection changes
  useEffect(() => {
    if (!selectedPermId || permutations.length < 2) {
      setDiff(null);
      return;
    }

    // Get the previous version to diff against
    const selectedIdx = permutations.findIndex((p) => p.id === selectedPermId);
    if (selectedIdx === -1 || selectedIdx >= permutations.length - 1) {
      setDiff(null);
      return;
    }

    const previousPerm = permutations[selectedIdx + 1];

    const loadDiff = async () => {
      setDiffLoading(true);
      try {
        const diffData = await decksApi.getDeckPermutationDiff(
          deckId,
          previousPerm.id,
          selectedPermId
        );
        setDiff(diffData);
      } catch {
        // Diff might not be available for all permutations
        setDiff(null);
      } finally {
        setDiffLoading(false);
      }
    };

    loadDiff();
  }, [selectedPermId, permutations, deckId]);

  // Fetch card names when diff changes
  useEffect(() => {
    if (!diff) return;

    const fetchCardNames = async () => {
      // Collect all unique card IDs from the diff
      const cardIds = new Set<number>();
      diff.addedCards.forEach((c) => cardIds.add(c.card_id));
      diff.removedCards.forEach((c) => cardIds.add(c.card_id));
      diff.changedCards.forEach((c) => cardIds.add(c.card_id));

      // Filter out IDs we already have names for
      const idsToFetch = Array.from(cardIds).filter((id) => !cardNames[id]);

      if (idsToFetch.length === 0) return;

      // Fetch card details for each ID
      const newNames: Record<number, string> = {};
      await Promise.all(
        idsToFetch.map(async (cardId) => {
          try {
            const card = await cardsApi.getCardByArenaId(cardId);
            newNames[cardId] = card.Name;
          } catch {
            newNames[cardId] = `Card #${cardId}`;
          }
        })
      );

      setCardNames((prev) => ({ ...prev, ...newNames }));
    };

    fetchCardNames();
  }, [diff, cardNames]);

  // Helper to get card name with fallback
  const getCardName = (cardId: number): string => {
    return cardNames[cardId] || `Card #${cardId}`;
  };

  const handleRestore = async (permId: number) => {
    if (!confirm('Are you sure you want to restore this version? Your current deck will be replaced.')) {
      return;
    }

    setRestoring(true);
    try {
      await decksApi.restoreDeckPermutation(deckId, permId);
      onRestore?.();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to restore version');
    } finally {
      setRestoring(false);
    }
  };

  const handleNameUpdate = async (permId: number) => {
    try {
      await decksApi.updateDeckPermutationName(deckId, permId, newName);
      setPermutations((prev) =>
        prev.map((p) => (p.id === permId ? { ...p, versionName: newName || null } : p))
      );
      setEditingName(null);
      setNewName('');
    } catch {
      // Silently fail name update
    }
  };

  const startEditingName = (perm: DeckPermutation) => {
    setEditingName(perm.id);
    setNewName(perm.versionName || '');
  };

  if (!isOpen) return null;

  const selectedPerm = permutations.find((p) => p.id === selectedPermId);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="deck-history-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Deck History: {deckName}</h2>
          <button className="close-button" onClick={onClose}>
            &times;
          </button>
        </div>

        <div className="modal-content">
          {loading && <LoadingSpinner message="Loading deck history..." />}

          {error && (
            <div className="error-message">
              <p>{error}</p>
            </div>
          )}

          {!loading && !error && permutations.length === 0 && (
            <div className="empty-state">
              <p>No version history available for this deck.</p>
              <p className="hint">
                Changes to your deck are automatically tracked when you modify cards.
              </p>
            </div>
          )}

          {!loading && !error && permutations.length > 0 && (
            <div className="history-layout">
              {/* Version list */}
              <div className="version-list">
                <h3>Versions</h3>
                {permutations.map((perm) => (
                  <div
                    key={perm.id}
                    className={`version-item ${selectedPermId === perm.id ? 'selected' : ''}`}
                    onClick={() => setSelectedPermId(perm.id)}
                  >
                    <div className="version-header">
                      <span className="version-number">
                        v{perm.versionNumber}
                        {perm.isCurrent && <span className="current-badge"> (Current)</span>}
                      </span>
                      {editingName === perm.id ? (
                        <div className="name-edit">
                          <input
                            type="text"
                            value={newName}
                            onChange={(e) => setNewName(e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') handleNameUpdate(perm.id);
                              if (e.key === 'Escape') setEditingName(null);
                            }}
                            autoFocus
                            placeholder="Version name"
                          />
                          <button onClick={() => handleNameUpdate(perm.id)}>Save</button>
                        </div>
                      ) : (
                        <span
                          className="version-name"
                          onClick={(e) => {
                            e.stopPropagation();
                            startEditingName(perm);
                          }}
                        >
                          {perm.versionName || 'Click to name'}
                        </span>
                      )}
                    </div>
                    <div className="version-meta">
                      <span className="version-date">{formatDate(perm.createdAt)}</span>
                      <span className="version-stats">
                        {perm.matchesPlayed > 0 ? (
                          <>
                            {perm.matchesWon}W-{perm.matchesPlayed - perm.matchesWon}L (
                            {perm.matchWinRate.toFixed(1)}%)
                          </>
                        ) : (
                          'No matches'
                        )}
                      </span>
                    </div>
                  </div>
                ))}
              </div>

              {/* Version details */}
              <div className="version-details">
                {selectedPerm && (
                  <>
                    <div className="details-header">
                      <h3>
                        Version {selectedPerm.versionNumber}
                        {selectedPerm.versionName && ` - ${selectedPerm.versionName}`}
                      </h3>
                      <button
                        className="restore-button"
                        onClick={() => handleRestore(selectedPerm.id)}
                        disabled={restoring || selectedPerm.isCurrent}
                      >
                        {restoring ? 'Restoring...' : 'Restore This Version'}
                      </button>
                    </div>

                    <div className="details-stats">
                      <div className="stat">
                        <span className="stat-label">Created</span>
                        <span className="stat-value">
                          {formatDate(selectedPerm.createdAt)}
                        </span>
                      </div>
                      <div className="stat">
                        <span className="stat-label">Matches</span>
                        <span className="stat-value">
                          {selectedPerm.matchesPlayed} played
                        </span>
                      </div>
                      <div className="stat">
                        <span className="stat-label">Match Win Rate</span>
                        <span className="stat-value">
                          {selectedPerm.matchWinRate.toFixed(1)}%
                        </span>
                      </div>
                      <div className="stat">
                        <span className="stat-label">Game Win Rate</span>
                        <span className="stat-value">
                          {selectedPerm.gameWinRate.toFixed(1)}%
                        </span>
                      </div>
                    </div>

                    {diffLoading && <LoadingSpinner message="Loading changes..." />}

                    {!diffLoading && diff && (
                      <div className="version-diff">
                        <h4>Changes from previous version</h4>

                        {diff.addedCards.length > 0 && (
                          <div className="diff-section added">
                            <h5>Added Cards</h5>
                            <ul>
                              {diff.addedCards.map((card, idx) => (
                                <li key={idx}>
                                  +{card.quantity} {getCardName(card.card_id)} ({card.board})
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}

                        {diff.removedCards.length > 0 && (
                          <div className="diff-section removed">
                            <h5>Removed Cards</h5>
                            <ul>
                              {diff.removedCards.map((card, idx) => (
                                <li key={idx}>
                                  -{card.quantity} {getCardName(card.card_id)} ({card.board})
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}

                        {diff.changedCards.length > 0 && (
                          <div className="diff-section changed">
                            <h5>Changed Cards</h5>
                            <ul>
                              {diff.changedCards.map((card, idx) => (
                                <li key={idx}>
                                  {getCardName(card.card_id)}: {card.old_quantity} &rarr;{' '}
                                  {card.new_quantity} ({card.board})
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}

                        {diff.addedCards.length === 0 &&
                          diff.removedCards.length === 0 &&
                          diff.changedCards.length === 0 && (
                            <p className="no-changes">No card changes detected</p>
                          )}
                      </div>
                    )}

                    {!diffLoading && !diff && selectedPermId !== permutations[permutations.length - 1]?.id && (
                      <p className="no-diff">Select a version to see changes from the previous version.</p>
                    )}
                  </>
                )}
              </div>
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button className="cancel-button" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

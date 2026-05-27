import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { trackEvent } from '@/services/analytics';
import { collection, cards as cardsApi } from '@/services/api';
import { gui } from '@/types/models';
import { useDownload } from '@/context/DownloadContext';
import SetCompletionPanel from '../components/SetCompletion';
import './Collection.css';

// Color icon mapping
const colorIcons: Record<string, string> = {
  W: 'https://svgs.scryfall.io/card-symbols/W.svg',
  U: 'https://svgs.scryfall.io/card-symbols/U.svg',
  B: 'https://svgs.scryfall.io/card-symbols/B.svg',
  R: 'https://svgs.scryfall.io/card-symbols/R.svg',
  G: 'https://svgs.scryfall.io/card-symbols/G.svg',
};

// Rarity colors
const rarityColors: Record<string, string> = {
  common: '#1a1a1a',
  uncommon: '#6b7c8d',
  rare: '#d4af37',
  mythic: '#e67e22',
};

interface FilterState {
  searchTerm: string;
  setCode: string;
  rarity: string;
  colors: string[];
  ownedOnly: boolean;
  sortBy: string;
  sortDesc: boolean;
}

const ITEMS_PER_PAGE = 50;

export default function Collection() {
  const [cards, setCards] = useState<gui.CollectionCard[]>([]);
  const [sets, setSets] = useState<gui.SetInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [totalCount, setTotalCount] = useState(0);
  const [filterCount, setFilterCount] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageJumpInput, setPageJumpInput] = useState<string>('1');
  const [showSetCompletion, setShowSetCompletion] = useState(false);
  const [collectionValue, setCollectionValue] = useState<{ totalValueUsd: number } | null>(null);

  const { startDownload, updateProgress, completeDownload } = useDownload();
  const autoRefreshRef = useRef<boolean>(false);
  const isLoadingRef = useRef<boolean>(false);
  const isAutoRefreshingRef = useRef<boolean>(false);
  const autoRefreshTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const viewedFiredRef = useRef(false);

  const [filters, setFilters] = useState<FilterState>({
    searchTerm: '',
    setCode: '',
    rarity: '',
    colors: [],
    ownedOnly: true,
    sortBy: 'name',
    sortDesc: false,
  });

  // Debounced search term
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState('');

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchTerm(filters.searchTerm);
    }, 300);
    return () => clearTimeout(timer);
  }, [filters.searchTerm]);

  const loadCollection = useCallback(async (isAutoRefresh = false) => {
    // Prevent multiple simultaneous requests
    if (isLoadingRef.current) {
      return;
    }

    isLoadingRef.current = true;
    isAutoRefreshingRef.current = isAutoRefresh;

    // Only show loading spinner for user-initiated loads, not auto-refresh
    if (!isAutoRefresh) {
      setLoading(true);
    }
    setError(null);
    try {
      const apiFilter = {
        set_code: filters.setCode,
        rarity: filters.rarity,
        colors: filters.colors,
        owned_only: filters.ownedOnly,
      };

      const response = await collection.getCollectionWithMetadata(apiFilter);
      // Note: REST API doesn't support search/sort/pagination server-side
      // The component handles this with client-side filtering
      // Normalize to array to prevent crashes when API returns null/undefined
      const normalizedCards = Array.isArray(response?.cards) ? response.cards : [];
      setCards(normalizedCards);
      setTotalCount(normalizedCards.length);
      setFilterCount(normalizedCards.length);

      // Analytics: feature_collection_viewed — once per mount when data is non-empty
      if (normalizedCards.length > 0 && !viewedFiredRef.current) {
        viewedFiredRef.current = true;
        trackEvent({
          name: 'feature_collection_viewed',
          properties: { card_count: normalizedCards.length },
        });
      }

      // Show download progress if cards were fetched from Scryfall
      const unknownFetched = response?.unknownCardsFetched ?? 0;
      const unknownRemaining = response?.unknownCardsRemaining ?? 0;
      const downloadId = 'collection-card-lookup';

      if (unknownFetched > 0 && unknownRemaining > 0) {
        // Cards were successfully fetched and more remain - continue auto-refresh
        const totalUnknown = unknownRemaining + unknownFetched;
        const progress = Math.round(((totalUnknown - unknownRemaining) / totalUnknown) * 100);

        startDownload(downloadId, `Fetching card info from Scryfall...`);
        updateProgress(downloadId, progress);

        // Auto-refresh to continue fetching remaining cards
        if (!autoRefreshRef.current) {
          autoRefreshRef.current = true;
          autoRefreshTimeoutRef.current = setTimeout(() => {
            autoRefreshRef.current = false;
            autoRefreshTimeoutRef.current = null;
            loadCollection(true); // Pass true to indicate auto-refresh
          }, 500);
        }
      } else if (unknownFetched > 0) {
        // All cards fetched, complete the download
        completeDownload(downloadId);
      }
      // If unknownFetched === 0, don't auto-refresh (all lookups failed or no cards to fetch)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load collection');
      console.error('Failed to load collection:', err);
    } finally {
      isLoadingRef.current = false;
      isAutoRefreshingRef.current = false;
      setLoading(false);
    }
  }, [filters.setCode, filters.rarity, filters.colors, filters.ownedOnly, startDownload, updateProgress, completeDownload]);

  const loadSets = useCallback(async () => {
    try {
      const setInfo = await cardsApi.getAllSetInfo();
      // Normalize to array to prevent crashes when API returns null/undefined
      setSets(Array.isArray(setInfo) ? setInfo : []);
    } catch (err) {
      console.error('Failed to load sets:', err);
    }
  }, []);

  const loadCollectionValue = useCallback(async () => {
    try {
      const value = await collection.getCollectionValue();
      setCollectionValue(value);
    } catch (err) {
      console.error('Failed to load collection value:', err);
    }
  }, []);

  // Load collection and sets on mount
  useEffect(() => {
    loadCollection();
    loadSets();
    loadCollectionValue();

    // Cleanup: clear auto-refresh timeout on unmount
    return () => {
      if (autoRefreshTimeoutRef.current) {
        clearTimeout(autoRefreshTimeoutRef.current);
        autoRefreshTimeoutRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Reload collection when filters change (but not on mount).
  // loadCollection is recreated by useCallback whenever its filter deps change
  // (filters.setCode, filters.rarity, filters.colors, filters.ownedOnly), so
  // depending on loadCollection here guarantees we always call the freshest
  // closure — the one that captures the current filter values.  Without
  // loadCollection in the dep array the effect calls a stale closure and the
  // server-side filters (set_code, rarity, colors, owned_only) silently have
  // no effect.  This was the root cause of bug #1974.
  const isInitialMount = useRef(true);
  useEffect(() => {
    if (isInitialMount.current) {
      isInitialMount.current = false;
      return;
    }
    loadCollection();
  }, [loadCollection]);

  // Reset page when filters change
  useEffect(() => {
    setCurrentPage(1);
  }, [debouncedSearchTerm, filters.setCode, filters.rarity, filters.colors, filters.ownedOnly, filters.sortBy, filters.sortDesc]);

  const handleFilterChange = (key: keyof FilterState, value: string | string[] | boolean) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
  };

  const handleColorToggle = (color: string) => {
    setFilters((prev) => ({
      ...prev,
      colors: prev.colors.includes(color)
        ? prev.colors.filter((c) => c !== color)
        : [...prev.colors, color],
    }));
  };

  // Process cards: filter by search, sort, and paginate
  const processedCards = useMemo(() => {
    let result = [...cards];

    // Filter by search term (client-side)
    if (debouncedSearchTerm) {
      const searchLower = debouncedSearchTerm.toLowerCase();
      result = result.filter(
        (card) =>
          card.name?.toLowerCase().includes(searchLower) ||
          card.setCode?.toLowerCase().includes(searchLower)
      );
    }

    // Sort
    const rarityOrder: Record<string, number> = {
      mythic: 4,
      rare: 3,
      uncommon: 2,
      common: 1,
    };

    result.sort((a, b) => {
      let comparison = 0;
      switch (filters.sortBy) {
        case 'name':
          comparison = (a.name || '').localeCompare(b.name || '');
          break;
        case 'quantity':
          comparison = (a.quantity || 0) - (b.quantity || 0);
          break;
        case 'rarity':
          comparison =
            (rarityOrder[a.rarity?.toLowerCase() || 'common'] || 0) -
            (rarityOrder[b.rarity?.toLowerCase() || 'common'] || 0);
          break;
        case 'cmc':
          comparison = (a.cmc || 0) - (b.cmc || 0);
          break;
        case 'price':
          comparison = (a.priceUsd || 0) - (b.priceUsd || 0);
          break;
        default:
          comparison = 0;
      }
      return filters.sortDesc ? -comparison : comparison;
    });

    return result;
  }, [cards, debouncedSearchTerm, filters.sortBy, filters.sortDesc]);

  // Update filter count when processed cards change
  useEffect(() => {
    setFilterCount(processedCards.length);
  }, [processedCards.length]);

  // Paginate
  const paginatedCards = useMemo(() => {
    const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
    return processedCards.slice(startIndex, startIndex + ITEMS_PER_PAGE);
  }, [processedCards, currentPage]);

  const totalPages = Math.ceil(processedCards.length / ITEMS_PER_PAGE);

  // Keep page-jump input in sync when page changes via First/Prev/Next/Last
  useEffect(() => {
    setPageJumpInput(String(currentPage));
  }, [currentPage]);

  const handlePageJump = useCallback(() => {
    const parsed = parseInt(pageJumpInput, 10);
    if (isNaN(parsed) || parsed < 1 || parsed > totalPages) {
      setPageJumpInput(String(currentPage));
      return;
    }
    setCurrentPage(parsed);
  }, [pageJumpInput, currentPage, totalPages]);

  // Build windowed page buttons: show ±2 pages around current (AC standard pattern)
  const windowedPages = useMemo(() => {
    const pages: number[] = [];
    const start = Math.max(1, currentPage - 2);
    const end = Math.min(totalPages, currentPage + 2);
    for (let p = start; p <= end; p++) {
      pages.push(p);
    }
    return pages;
  }, [currentPage, totalPages]);

  if (loading && cards.length === 0) {
    return (
      <div className="collection-page loading-state" data-testid="collection-loading">
        <div className="loading-spinner"></div>
        <p>Loading collection...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="collection-page error-state" data-testid="collection-error">
        <div className="error-icon">!</div>
        <h2>Error Loading Collection</h2>
        <p>{error}</p>
        <button onClick={() => loadCollection()} className="retry-button" data-testid="collection-retry-button">
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="collection-page" data-testid="collection-page">
      {/* Header with stats */}
      <div className="collection-header" data-testid="collection-header">
        <div className="header-title">
          <h1>Collection</h1>
          <div className="collection-stats-summary" data-testid="collection-stats">
            <span className="stat-item">
              <span className="stat-label">Cards in Set:</span>
              <span className="stat-value">{filterCount}</span>
            </span>
            <span className="stat-separator">|</span>
            <span className="stat-item">
              <span className="stat-label">Total Cards:</span>
              <span className="stat-value">{totalCount}</span>
            </span>
            {collectionValue && collectionValue.totalValueUsd > 0 && (
              <>
                <span className="stat-separator">|</span>
                <span className="stat-item collection-value">
                  <span className="stat-label">Est. Value:</span>
                  <span className="stat-value price-value">
                    ${collectionValue.totalValueUsd.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                </span>
              </>
            )}
          </div>
        </div>
        {filters.setCode && (
          <button
            className="set-completion-button"
            onClick={() => setShowSetCompletion(!showSetCompletion)}
            data-testid="collection-toggle-set-completion"
          >
            {showSetCompletion ? 'Hide' : 'Show'} Set Completion
          </button>
        )}
      </div>

      {/* Filter Controls */}
      <div className="collection-filters">
        <div className="filter-row">
          {/* Search */}
          <div className="filter-group search-group">
            <input
              type="text"
              placeholder="Search by name..."
              value={filters.searchTerm}
              onChange={(e) => handleFilterChange('searchTerm', e.target.value)}
              className="search-input"
              data-testid="collection-search-input"
            />
          </div>

          {/* Set Filter */}
          <div className="filter-group">
            <select
              value={filters.setCode}
              onChange={(e) => handleFilterChange('setCode', e.target.value)}
              className="filter-select"
              data-testid="collection-set-filter"
            >
              <option value="">All Sets</option>
              {sets.map((set) => (
                <option key={set.code} value={set.code}>
                  {set.name} ({set.code.toUpperCase()})
                </option>
              ))}
            </select>
          </div>

          {/* Rarity Filter */}
          <div className="filter-group">
            <select
              value={filters.rarity}
              onChange={(e) => handleFilterChange('rarity', e.target.value)}
              className="filter-select"
              data-testid="collection-rarity-filter"
            >
              <option value="">All Rarities</option>
              <option value="common">Common</option>
              <option value="uncommon">Uncommon</option>
              <option value="rare">Rare</option>
              <option value="mythic">Mythic</option>
            </select>
          </div>

          {/* Sort */}
          <div className="filter-group">
            <select
              value={`${filters.sortBy}-${filters.sortDesc ? 'desc' : 'asc'}`}
              onChange={(e) => {
                const [sortBy, direction] = e.target.value.split('-');
                handleFilterChange('sortBy', sortBy);
                handleFilterChange('sortDesc', direction === 'desc');
              }}
              className="filter-select"
              data-testid="collection-sort-select"
            >
              <option value="name-asc">Name (A-Z)</option>
              <option value="name-desc">Name (Z-A)</option>
              <option value="quantity-desc">Quantity (High)</option>
              <option value="quantity-asc">Quantity (Low)</option>
              <option value="rarity-desc">Rarity (High)</option>
              <option value="rarity-asc">Rarity (Low)</option>
              <option value="cmc-asc">CMC (Low)</option>
              <option value="cmc-desc">CMC (High)</option>
              <option value="price-desc">Price (High)</option>
              <option value="price-asc">Price (Low)</option>
            </select>
          </div>
        </div>

        <div className="filter-row secondary">
          {/* Color Filters */}
          <span className="filter-label">Colors:</span>
          <div className="color-buttons">
            {['W', 'U', 'B', 'R', 'G'].map((color) => (
              <button
                key={color}
                className={`color-button ${filters.colors.includes(color) ? 'active' : ''}`}
                onClick={() => handleColorToggle(color)}
                title={color === 'W' ? 'White' : color === 'U' ? 'Blue' : color === 'B' ? 'Black' : color === 'R' ? 'Red' : 'Green'}
                data-testid={`collection-color-button-${color}`}
              >
                <img src={colorIcons[color]} alt={color} className="color-icon" />
              </button>
            ))}
          </div>

          {/* Owned Only Toggle */}
          <label className="toggle-label">
            <input
              type="checkbox"
              checked={filters.ownedOnly}
              onChange={(e) => handleFilterChange('ownedOnly', e.target.checked)}
              data-testid="collection-owned-only-checkbox"
            />
            Owned only
          </label>

          {/* Result Count */}
          <div className="filter-results">
            Showing {filterCount} of {totalCount} cards
          </div>
        </div>
      </div>

      {/* Set Completion Panel */}
      {showSetCompletion && filters.setCode && (
        <div className="set-completion-container">
          <SetCompletionPanel
            setCode={filters.setCode}
            onClose={() => setShowSetCompletion(false)}
          />
        </div>
      )}

      {/* Card Grid */}
      {processedCards.length === 0 ? (
        <div className="empty-state" data-testid="collection-empty">
          <div className="empty-icon">!</div>
          <h2>No Cards Found</h2>
          <p>
            {filters.searchTerm || filters.setCode || filters.rarity || filters.colors.length > 0
              ? 'Try adjusting your filters'
              : 'Your collection is empty. Start playing to add cards!'}
          </p>
        </div>
      ) : (
        <>
          <div className="card-grid" data-testid="collection-card-grid">
            {paginatedCards.map((card) => {
              // Check if we have a real card image (not the card back placeholder)
              const isCardBackPlaceholder = card.imageUri?.includes('back.png');
              const hasImage = card.imageUri && card.imageUri !== '' && !isCardBackPlaceholder;
              return (
                <div
                  key={`${card.cardId}-${card.setCode}`}
                  className={`collection-card ${card.quantity === 0 ? 'not-owned' : ''} ${!hasImage ? 'no-image' : ''}`}
                >
                  {hasImage ? (
                    <>
                      <img
                        src={card.imageUri}
                        alt={card.name || `Card #${card.arenaId}`}
                        style={{ width: '100%', borderRadius: '12px' }}
                        onError={(e) => {
                          const target = e.target as HTMLImageElement;
                          // Hide broken image and show fallback info
                          target.style.display = 'none';
                          const parent = target.parentElement;
                          if (parent && !parent.querySelector('.card-info-fallback')) {
                            parent.classList.add('no-image');
                          }
                        }}
                      />
                      {card.priceUsd !== undefined && card.priceUsd > 0 && (
                        <div className="card-price-badge">
                          ${card.priceUsd.toFixed(2)}
                        </div>
                      )}
                    </>
                  ) : (
                    <div className="card-info-fallback">
                      <div className="card-fallback-name">{card.name || 'Unknown Card'}</div>
                      {card.setCode ? (
                        <div className="card-fallback-set">{card.setCode.toUpperCase()}</div>
                      ) : (
                        <div className="card-fallback-hint">
                          Card #{card.arenaId}
                          <br />
                          <span className="download-hint">Download set in Settings</span>
                        </div>
                      )}
                      {card.manaCost && <div className="card-fallback-mana">{card.manaCost}</div>}
                      {card.rarity && (
                        <div
                          className="card-fallback-rarity"
                          style={{ color: rarityColors[card.rarity.toLowerCase()] || '#888' }}
                        >
                          {card.rarity}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="pagination">
              <button
                className="page-button"
                disabled={currentPage === 1}
                onClick={() => setCurrentPage(1)}
                data-testid="collection-pagination-first"
              >
                First
              </button>
              <button
                className="page-button"
                disabled={currentPage === 1}
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                data-testid="collection-pagination-prev"
              >
                Previous
              </button>

              {/* Windowed page buttons ±2 around current page */}
              {windowedPages[0] > 1 && <span className="page-ellipsis">…</span>}
              {windowedPages.map((p) => (
                <button
                  key={p}
                  className={`page-button${p === currentPage ? ' page-button--active' : ''}`}
                  onClick={() => setCurrentPage(p)}
                  data-testid={p === currentPage ? 'collection-pagination-current' : undefined}
                  aria-current={p === currentPage ? 'page' : undefined}
                >
                  {p}
                </button>
              ))}
              {windowedPages[windowedPages.length - 1] < totalPages && <span className="page-ellipsis">…</span>}

              {/* Page-jump input (AC1/AC2/AC3) */}
              <label className="page-jump-label" htmlFor="collection-page-jump">
                Go to page
                <input
                  id="collection-page-jump"
                  className="page-jump-input"
                  type="number"
                  min={1}
                  max={totalPages}
                  value={pageJumpInput}
                  data-testid="collection-page-jump"
                  onChange={(e) => setPageJumpInput(e.target.value)}
                  onBlur={handlePageJump}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handlePageJump();
                  }}
                />
              </label>

              <button
                className="page-button"
                disabled={currentPage === totalPages}
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                data-testid="collection-pagination-next"
              >
                Next
              </button>
              <button
                className="page-button"
                disabled={currentPage === totalPages}
                onClick={() => setCurrentPage(totalPages)}
                data-testid="collection-pagination-last"
              >
                Last
              </button>
            </div>
          )}
        </>
      )}

      {/* Loading overlay for filter changes */}
      {loading && cards.length > 0 && (
        <div className="loading-overlay">
          <div className="loading-spinner small"></div>
        </div>
      )}
    </div>
  );
}

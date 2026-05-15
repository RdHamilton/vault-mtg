/**
 * Decks API service.
 *
 * Phase 2 PR #9: cloud-data deck CRUD, cards, tags, permutations,
 * import/export, and library reads now hit the BFF directly via
 * apiClient. Deck-builder + recommendation endpoints (build-around,
 * generate, suggest, recommendations/*) are documented BFF stubs
 * pending the ML pipeline.
 *
 * Plan tracker: .claude/plans/spa-route-migration.md
 */

import { get, post, put, del } from '../apiClient';
import { models, gui } from '@/types/models';

// ---------------------------------------------------------------------------
// BFF wire shapes
// ---------------------------------------------------------------------------
//
// The BFF deck-detail endpoint (GET /decks/:id) returns a flat object that
// embeds the deck metadata alongside the cards array.  This does NOT match the
// frontend's gui.DeckWithCards type (which nests metadata under a "deck" key
// and uses PascalCase card fields).  The mappers below normalise the BFF wire
// shape into the types the SPA components expect.
//
// See services/bff/internal/api/handlers/decks.go: deckWithCardsResponse.

/**
 * Raw card shape returned by the BFF deck-detail endpoint.
 * Field names are camelCase as serialised by encoding/json.
 */
interface BffDeckCardRaw {
  cardId: number;
  quantity: number;
  board: string;
  fromDraftPick: boolean;
  name: string;
  setCode: string;
  manaCost: string;
  cmc: number;
  typeLine: string;
  rarity: string;
  imageUri: string;
  colors: string[];
}

/**
 * Raw deck-detail response from the BFF (after the { data: ... } envelope is
 * unwrapped by apiClient).  The deck metadata is flat — there is no nested
 * "deck" key.
 */
interface BffDeckDetailRaw {
  id: string;
  name: string;
  format: string;
  source: string;
  draftEventId?: string | null;
  matchesPlayed: number;
  matchesWon: number;
  gamesPlayed: number;
  gamesWon: number;
  winRate: number;
  isAppCreated: boolean;
  createdAt: string;
  modifiedAt: string;
  lastPlayed?: string | null;
  colorIdentity?: string;
  description?: string;
  cardCount: number;
  tags: string[];
  cards: BffDeckCardRaw[];
}

/**
 * Map a raw BFF card row to the models.DeckCard shape expected by the SPA.
 *
 * The BFF sends camelCase fields (cardId, fromDraftPick …).  models.DeckCard
 * reads PascalCase keys (CardID, FromDraftPick …) from the source object.
 */
function mapBffCard(raw: BffDeckCardRaw): models.DeckCard {
  return new models.DeckCard({
    ID: 0, // not provided by detail endpoint; unused by DeckBuilder
    DeckID: '', // not provided by detail endpoint; unused by DeckBuilder
    CardID: raw.cardId,
    Quantity: raw.quantity,
    Board: raw.board,
    FromDraftPick: raw.fromDraftPick ?? false,
  });
}

/**
 * Map a raw BFF deck-detail response to the gui.DeckWithCards shape.
 *
 * Constructs the nested { deck, cards, tags } structure that DeckBuilder
 * and related components rely on.
 */
function mapBffDeckDetail(raw: BffDeckDetailRaw): gui.DeckWithCards {
  const deck = new models.Deck({
    ID: raw.id,
    AccountID: 0, // not returned by detail endpoint; unused by DeckBuilder
    Name: raw.name,
    Format: raw.format,
    Source: raw.source,
    DraftEventID: raw.draftEventId ?? undefined,
    Description: raw.description,
    ColorIdentity: raw.colorIdentity,
    MatchesPlayed: raw.matchesPlayed ?? 0,
    MatchesWon: raw.matchesWon ?? 0,
    GamesPlayed: raw.gamesPlayed ?? 0,
    GamesWon: raw.gamesWon ?? 0,
    CreatedAt: raw.createdAt,
    ModifiedAt: raw.modifiedAt,
    LastPlayed: raw.lastPlayed ?? undefined,
  });

  const cards = (raw.cards ?? []).map(mapBffCard);

  // BFF does not return tags on the detail endpoint — tags come from the list
  // summary row.  Return an empty array so consumers don't break.
  const tags: models.DeckTag[] = [];

  return new gui.DeckWithCards({ deck, cards, tags });
}

/**
 * Map a raw BFF deck-detail response to a models.Deck, discarding card data.
 * Used by createDeck / updateDeck which only need the deck metadata.
 */
function mapBffDeckToModel(raw: BffDeckDetailRaw): models.Deck {
  return new models.Deck({
    ID: raw.id,
    AccountID: 0,
    Name: raw.name,
    Format: raw.format,
    Source: raw.source,
    DraftEventID: raw.draftEventId ?? undefined,
    Description: raw.description,
    ColorIdentity: raw.colorIdentity,
    MatchesPlayed: raw.matchesPlayed ?? 0,
    MatchesWon: raw.matchesWon ?? 0,
    GamesPlayed: raw.gamesPlayed ?? 0,
    GamesWon: raw.gamesWon ?? 0,
    CreatedAt: raw.createdAt,
    ModifiedAt: raw.modifiedAt,
    LastPlayed: raw.lastPlayed ?? undefined,
  });
}

// Re-export types for convenience
export type Deck = models.Deck;
export type DeckWithCards = gui.DeckWithCards;
export type DeckListItem = gui.DeckListItem;
export type DeckStatistics = gui.DeckStatistics;
export type DeckPerformance = models.DeckPerformance;
export type ExportDeckRequest = gui.ExportDeckRequest;
export type ExportDeckResponse = gui.ExportDeckResponse;
export type ImportDeckRequest = gui.ImportDeckRequest;
export type ImportDeckResponse = gui.ImportDeckResponse;
export type SuggestedDeckResponse = gui.SuggestedDeckResponse;
export type ArchetypeClassificationResult = gui.ArchetypeClassificationResult;

/**
 * Request to create a deck.
 */
export interface CreateDeckRequest {
  name: string;
  format: string;
  source: string;
  draft_event_id?: string;
}

/**
 * Request to update a deck.
 */
export interface UpdateDeckRequest {
  name?: string;
  format?: string;
}

/**
 * Request to import a deck.
 */
export interface ImportDeckApiRequest {
  content: string;
  name: string;
  format: string;
}

/**
 * Request to export a deck.
 */
export interface ExportDeckApiRequest {
  format: string;
}

/**
 * Request to suggest decks.
 */
export interface SuggestDecksRequest {
  session_id: string;
}

/**
 * Request to analyze a deck.
 */
export interface AnalyzeDeckRequest {
  deck_id: string;
}

/**
 * Get all decks with optional filtering.
 */
export async function getDecks(options?: {
  format?: string;
  source?: string;
}): Promise<DeckListItem[]> {
  const params = new URLSearchParams();
  if (options?.format) params.set('format', options.format);
  if (options?.source) params.set('source', options.source);

  const query = params.toString();
  return get<DeckListItem[]>(`/decks${query ? `?${query}` : ''}`);
}

/**
 * Get a single deck by ID with cards.
 *
 * The BFF returns a flat object (deckWithCardsResponse); mapBffDeckDetail
 * normalises it into the nested gui.DeckWithCards shape the SPA expects.
 */
export async function getDeck(deckId: string): Promise<DeckWithCards> {
  const raw = await get<BffDeckDetailRaw>(`/decks/${deckId}`);
  return mapBffDeckDetail(raw);
}

/**
 * Create a new deck.
 *
 * The BFF returns a deckWithCardsResponse (flat camelCase); we extract only
 * the deck metadata and return it as models.Deck so callers can read deck.ID.
 */
export async function createDeck(request: CreateDeckRequest): Promise<Deck> {
  const raw = await post<BffDeckDetailRaw>('/decks', request);
  return mapBffDeckToModel(raw);
}

/**
 * Update a deck.
 *
 * The BFF returns a deckWithCardsResponse (flat camelCase); mapBffDeckDetail
 * normalises it into the nested gui.DeckWithCards shape the SPA expects.
 */
export async function updateDeck(deckId: string, request: UpdateDeckRequest): Promise<DeckWithCards> {
  const raw = await put<BffDeckDetailRaw>(`/decks/${deckId}`, request);
  return mapBffDeckDetail(raw);
}

/**
 * Delete a deck.
 */
export async function deleteDeck(deckId: string): Promise<void> {
  return del<void>(`/decks/${deckId}`);
}

/**
 * Get deck statistics.
 */
export async function getDeckStats(deckId: string): Promise<DeckStatistics> {
  return get<DeckStatistics>(`/decks/${deckId}/stats`);
}

/**
 * Get deck performance/matches.
 */
export async function getDeckMatches(deckId: string): Promise<DeckPerformance> {
  return get<DeckPerformance>(`/decks/${deckId}/matches`);
}

/**
 * Get deck mana curve.
 */
export async function getDeckCurve(deckId: string): Promise<DeckStatistics> {
  return get<DeckStatistics>(`/decks/${deckId}/curve`);
}

/**
 * Get deck color distribution.
 */
export async function getDeckColors(deckId: string): Promise<DeckStatistics> {
  return get<DeckStatistics>(`/decks/${deckId}/colors`);
}

/**
 * Export a deck.
 */
export async function exportDeck(
  deckId: string,
  request: ExportDeckApiRequest
): Promise<ExportDeckResponse> {
  return post<ExportDeckResponse>(`/decks/${deckId}/export`, request);
}

/**
 * Import a deck from text.
 */
export async function importDeck(request: ImportDeckApiRequest): Promise<ImportDeckResponse> {
  return post<ImportDeckResponse>('/decks/import', request);
}

/**
 * Parse a deck list without saving.
 */
export async function parseDeckList(content: string): Promise<ImportDeckResponse> {
  return post<ImportDeckResponse>('/decks/parse', { content });
}

/**
 * Response from suggest decks endpoint.
 * Matches backend gui.SuggestDecksResponse.
 */
export interface SuggestDecksApiResponse {
  suggestions: SuggestedDeckResponse[];
  totalCombos: number;
  viableCombos: number;
  bestCombo?: {
    colors: string[];
    name: string;
  };
  error?: string;
}

/**
 * Get deck suggestions for a draft.
 * Returns the full response with suggestions array, totals, and best combo.
 */
export async function suggestDecks(request: SuggestDecksRequest): Promise<SuggestDecksApiResponse> {
  return post<SuggestDecksApiResponse>('/decks/suggest', request);
}

/**
 * Analyze a deck (classify archetype).
 */
export async function analyzeDeck(
  request: AnalyzeDeckRequest
): Promise<ArchetypeClassificationResult> {
  return post<ArchetypeClassificationResult>('/decks/analyze', request);
}

/**
 * Get decks by source (draft, constructed, imported).
 */
export async function getDecksBySource(source: string): Promise<DeckListItem[]> {
  return getDecks({ source });
}

/**
 * Get decks by format.
 */
export async function getDecksByFormat(format: string): Promise<DeckListItem[]> {
  return getDecks({ format });
}

/**
 * Get decks by tags.
 */
export async function getDecksByTags(tags: string[]): Promise<DeckListItem[]> {
  return post<DeckListItem[]>('/decks/by-tags', { tags });
}

/**
 * Get deck library with filters.
 */
export async function getDeckLibrary(filter: gui.DeckLibraryFilter): Promise<DeckListItem[]> {
  return post<DeckListItem[]>('/decks/library', filter);
}

/**
 * Clone a deck.
 *
 * The BFF returns a deckWithCardsResponse (flat camelCase); we extract only
 * the deck metadata and return it as models.Deck so callers can read deck.ID.
 */
export async function cloneDeck(deckId: string, newName: string): Promise<Deck> {
  const raw = await post<BffDeckDetailRaw>(`/decks/${deckId}/clone`, { name: newName });
  return mapBffDeckToModel(raw);
}

/**
 * Get deck by draft event ID.
 *
 * The BFF returns a deckWithCardsResponse (flat camelCase); mapBffDeckDetail
 * normalises it into the nested gui.DeckWithCards shape the SPA expects.
 */
export async function getDeckByDraftEvent(draftEventId: string): Promise<DeckWithCards> {
  const raw = await get<BffDeckDetailRaw>(`/decks/by-draft/${draftEventId}`);
  return mapBffDeckDetail(raw);
}

/**
 * Get deck statistics.
 */
export async function getDeckStatistics(deckId: string): Promise<DeckStatistics> {
  return get<DeckStatistics>(`/decks/${deckId}/statistics`);
}

/**
 * Get deck performance.
 */
export async function getDeckPerformance(deckId: string): Promise<DeckPerformance> {
  return get<DeckPerformance>(`/decks/${deckId}/performance`);
}

/**
 * Validate a draft deck.
 */
export async function validateDraftDeck(deckId: string): Promise<boolean> {
  const result = await get<{ valid: boolean }>(`/decks/${deckId}/validate-draft`);
  return result.valid;
}

/**
 * Apply a suggested deck to an existing deck.
 */
export async function applySuggestedDeck(deckId: string, suggestion: SuggestedDeckResponse): Promise<void> {
  await post(`/decks/apply-suggestion`, { deck_id: deckId, suggestion });
}

/**
 * Get suggested deck export content.
 */
export async function getSuggestedDeckExportContent(suggestion: SuggestedDeckResponse, deckName: string): Promise<string> {
  const result = await post<{ content: string }>('/decks/suggested/export-content', {
    suggestion,
    deck_name: deckName,
  });
  return result.content;
}

/**
 * Classify deck archetype.
 */
export async function classifyDeckArchetype(deckId: string): Promise<ArchetypeClassificationResult> {
  return get<ArchetypeClassificationResult>(`/decks/${deckId}/classify`);
}

/**
 * Add a tag to a deck.
 */
export async function addTag(deckId: string, tag: string): Promise<void> {
  await post(`/decks/${deckId}/tags`, { tag });
}

/**
 * Remove a tag from a deck.
 */
export async function removeTag(deckId: string, tag: string): Promise<void> {
  await del(`/decks/${deckId}/tags/${encodeURIComponent(tag)}`);
}

/**
 * Add a card to a deck.
 */
export async function addCard(request: {
  deck_id: string;
  arena_id: number;
  quantity: number;
  zone: string;
  is_sideboard: boolean;
  from_draft?: boolean;
}): Promise<void> {
  // Map frontend field names to backend expected field names
  const body = {
    cardID: request.arena_id,
    quantity: request.quantity,
    board: request.zone,
    fromDraft: request.from_draft ?? false,
  };
  await post(`/decks/${request.deck_id}/cards`, body);
}

/**
 * Remove one copy of a card from a deck (decrements quantity by 1).
 */
export async function removeCard(request: {
  deck_id: string;
  arena_id: number;
  zone: string;
}): Promise<void> {
  await del(`/decks/${request.deck_id}/cards/${request.arena_id}?zone=${request.zone}`);
}

/**
 * Remove all copies of a card from a deck.
 */
export async function removeAllCopies(request: {
  deck_id: string;
  arena_id: number;
  zone: string;
}): Promise<void> {
  await del(`/decks/${request.deck_id}/cards/${request.arena_id}/all?zone=${request.zone}`);
}

/**
 * Request to build a deck around a seed card.
 */
export interface BuildAroundSeedRequest {
  seed_card_id: number;
  max_results?: number;
  budget_mode?: boolean;
  set_restriction?: 'single' | 'multiple' | 'all';
  allowed_sets?: string[];
}

/**
 * Score breakdown for detailed reasoning about card suggestions.
 */
export interface ScoreBreakdown {
  colorFit: number;  // 0.0-1.0, weight: 25%
  curveFit: number;  // 0.0-1.0, weight: 20%
  synergy: number;   // 0.0-1.0, weight: 30%
  quality: number;   // 0.0-1.0, weight: 15%
  overall: number;   // Final weighted score
}

/**
 * Synergy detail describing a specific synergy between a card and the deck.
 */
export interface SynergyDetail {
  type: 'keyword' | 'theme' | 'creature_type' | 'package';
  name: string;        // e.g., "flying", "tokens", "Elf", "Spellslinger"
  description: string; // e.g., "Matches 3 other flying creatures"
}

/**
 * Card suggestion with ownership information.
 */
export interface CardWithOwnership {
  cardID: number;
  name: string;
  manaCost?: string;
  cmc: number;
  colors: string[];
  typeLine: string;
  rarity?: string;
  imageURI?: string;
  score: number;
  reasoning: string;
  inCollection: boolean;
  ownedCount: number;
  neededCount: number;
  currentCopies: number;     // Copies currently in deck
  recommendedCopies: number; // Recommended total copies (1-4)
  scoreBreakdown?: ScoreBreakdown;
  synergyDetails?: SynergyDetail[];
}

/**
 * Suggested land for a deck.
 */
export interface SuggestedLandResponse {
  cardID: number;
  name: string;
  quantity: number;
  color: string;
}

/**
 * Analysis of seed deck suggestions.
 */
export interface SeedDeckAnalysis {
  colorIdentity: string[];
  keywords: string[];
  themes: string[];
  idealCurve: Record<number, number>;
  suggestedLandCount: number;
  totalCards: number;
  inCollectionCount: number;
  missingCount: number;
  missingWildcardCost: Record<string, number>;
}

/**
 * Response from build around seed endpoint.
 */
export interface BuildAroundSeedResponse {
  seedCard: CardWithOwnership;
  suggestions: CardWithOwnership[];
  lands: SuggestedLandResponse[];
  analysis: SeedDeckAnalysis;
}

/**
 * Build a deck around a seed card.
 */
export async function buildAroundSeed(
  request: BuildAroundSeedRequest
): Promise<BuildAroundSeedResponse> {
  return post<BuildAroundSeedResponse>('/decks/build-around', request);
}

/**
 * Request for iterative deck building suggestions.
 * The API analyzes ALL deck cards collectively to find commonalities
 * (colors, themes, keywords) and suggests cards that complement the deck.
 */
export interface IterativeBuildAroundRequest {
  seed_card_id?: number;      // Optional - API analyzes all deck cards collectively
  deck_card_ids: number[];    // Required - all cards currently in the deck
  max_results?: number;
  budget_mode?: boolean;
  set_restriction?: 'single' | 'multiple' | 'all';
  allowed_sets?: string[];
}

/**
 * Live analysis of the deck being built.
 */
export interface LiveDeckAnalysis {
  colorIdentity: string[];
  keywords: string[];
  themes: string[];
  currentCurve: Record<number, number>;
  recommendedLandCount: number;
  totalCards: number;
  inCollectionCount: number;
}

/**
 * Response from iterative build-around endpoint.
 */
export interface IterativeBuildAroundResponse {
  suggestions: CardWithOwnership[];
  deckAnalysis: LiveDeckAnalysis;
  slotsRemaining: number;
  landSuggestions: SuggestedLandResponse[];
}

/**
 * Get next card suggestions for iterative deck building.
 * Called as user picks cards one-by-one.
 */
export async function suggestNextCards(
  request: IterativeBuildAroundRequest
): Promise<IterativeBuildAroundResponse> {
  return post<IterativeBuildAroundResponse>('/decks/build-around/suggest-next', request);
}

// ==========================================
// Complete Deck Generation (Issue #774)
// ==========================================

/**
 * Archetype profile for deck building.
 * Matches backend ArchetypeProfileResponse.
 */
export interface ArchetypeProfile {
  name: string;
  landCount: number;
  curveTargets: Record<number, number>;
  creatureRatio: number;
  removalCount: number;
  cardAdvantage: number;
  description: string;
  splashTendency: number;
  icon: string;
}

/**
 * Request to generate a complete 60-card deck.
 */
export interface GenerateCompleteDeckRequest {
  seed_card_id: number;
  archetype: 'aggro' | 'midrange' | 'control' | 'tempo' | 'ramp' | 'combo' | 'tokens' | 'aristocrats';
  budget_mode?: boolean;
  /** Set restriction: 'single' = seed card's set, 'multiple' = allowed_sets, 'all' = all Standard-legal sets (default) */
  set_restriction?: 'single' | 'multiple' | 'all';
  allowed_sets?: string[];
  /** Optional existing deck cards to build around (instead of just seed card) */
  deck_card_ids?: number[];
}

/**
 * Score breakdown for card scoring.
 */
export interface ScoreBreakdown {
  colorFit: number;
  curveFit: number;
  synergy: number;
  quality: number;
  overall: number;
}

/**
 * Card with quantity for generated deck.
 */
export interface CardWithQuantity {
  cardID: number;
  name: string;
  manaCost?: string;
  cmc: number;
  colors: string[];
  typeLine: string;
  rarity?: string;
  imageURI?: string;
  quantity: number;
  score: number;
  reasoning: string;
  inCollection: boolean;
  ownedCount: number;
  neededCount: number;
  scoreBreakdown?: ScoreBreakdown;
  synergyDetails?: SynergyDetail[];
}

/**
 * Land with quantity for generated deck.
 */
export interface LandWithQuantity {
  cardID: number;
  name: string;
  quantity: number;
  colors: string[];
  isBasic: boolean;
  entersTapped: boolean;
}

/**
 * Strategy and game plan for a generated deck.
 */
export interface DeckStrategy {
  summary: string;
  gamePlan: string;
  keyCards: string[];
  mulligan: string;
  strengths: string[];
  weaknesses: string[];
}

/**
 * Analysis of a generated deck.
 * Matches backend GeneratedDeckAnalysisResponse.
 */
export interface GeneratedDeckAnalysis {
  totalCards: number;
  spellCount: number;
  landCount: number;
  creatureCount: number;
  nonCreatureCount: number;
  averageCMC: number;
  manaCurve: Record<number, number>;
  colorDistribution: Record<string, number>;
  inCollectionCount: number;
  missingCount: number;
  missingWildcardCost: Record<string, number>;
  archetypeMatch: number;
}

/**
 * Response from complete deck generation.
 */
export interface GenerateCompleteDeckResponse {
  seedCard: CardWithOwnership;
  spells: CardWithQuantity[];
  lands: LandWithQuantity[];
  strategy: DeckStrategy;
  analysis: GeneratedDeckAnalysis;
}

/**
 * Generate a complete 60-card deck from a seed card.
 */
export async function generateCompleteDeck(
  request: GenerateCompleteDeckRequest
): Promise<GenerateCompleteDeckResponse> {
  return post<GenerateCompleteDeckResponse>('/decks/generate', request);
}

/**
 * Get available archetype profiles.
 * Returns a record keyed by archetype name (lowercase).
 */
export async function getArchetypeProfiles(): Promise<Record<string, ArchetypeProfile>> {
  const profiles = await get<ArchetypeProfile[]>('/decks/archetypes');
  const record: Record<string, ArchetypeProfile> = {};
  for (const profile of profiles) {
    record[profile.name.toLowerCase()] = profile;
  }
  return record;
}

// ============================================================================
// Card Performance Analysis (Issue #771)
// ============================================================================

/**
 * Performance metrics for a single card within a deck.
 */
export interface CardPerformance {
  cardId: number;
  cardName: string;
  quantity: number;
  gamesWithCard: number;
  gamesDrawn: number;
  gamesPlayed: number;
  winRateWhenDrawn: number;
  winRateWhenPlayed: number;
  deckWinRate: number;
  playRate: number;
  winContribution: number;
  impactScore: number;
  confidenceLevel: 'high' | 'medium' | 'low';
  sampleSize: number;
  performanceGrade: 'excellent' | 'good' | 'average' | 'poor' | 'bad';
  avgTurnPlayed: number;
  turnPlayedDist?: Record<number, number>;
  mulliganedAway: number;
  mulliganRate: number;
}

/**
 * Full deck performance analysis response.
 */
export interface DeckPerformanceAnalysis {
  deckId: string;
  deckName: string;
  totalMatches: number;
  totalGames: number;
  overallWinRate: number;
  cardPerformance: CardPerformance[];
  bestPerformers: string[];
  worstPerformers: string[];
  analysisDate: string;
}

/**
 * Card recommendation for add/remove/swap.
 */
export interface PerformanceCardRecommendation {
  type: 'add' | 'remove' | 'swap';
  cardId: number;
  cardName: string;
  reason: string;
  impactEstimate: number;
  confidence: 'high' | 'medium' | 'low';
  priority: number;
  swapForCardId?: number;
  swapForCardName?: string;
  basedOnGames: number;
}

/**
 * All recommendations response for a deck.
 */
export interface DeckRecommendationsResponse {
  deckId: string;
  deckName: string;
  currentWinRate: number;
  addRecommendations: PerformanceCardRecommendation[];
  removeRecommendations: PerformanceCardRecommendation[];
  swapRecommendations: PerformanceCardRecommendation[];
  projectedWinRate: number;
}

/**
 * Get card performance metrics for a deck.
 * @param deckId - The deck ID
 * @param options - Optional query parameters
 */
export async function getCardPerformance(
  deckId: string,
  options?: {
    minGames?: number;
    includeLands?: boolean;
  }
): Promise<DeckPerformanceAnalysis> {
  const params = new URLSearchParams();
  if (options?.minGames !== undefined) {
    params.set('min_games', options.minGames.toString());
  }
  if (options?.includeLands) {
    params.set('include_lands', 'true');
  }

  const query = params.toString();
  return get<DeckPerformanceAnalysis>(
    `/decks/${deckId}/card-performance${query ? `?${query}` : ''}`
  );
}

/**
 * Get card add recommendations based on performance data.
 * @param deckId - The deck ID
 * @param options - Optional query parameters
 */
export async function getPerformanceAddRecommendations(
  deckId: string,
  options?: {
    maxResults?: number;
    format?: string;
  }
): Promise<PerformanceCardRecommendation[]> {
  const params = new URLSearchParams();
  if (options?.maxResults !== undefined) {
    params.set('max_results', options.maxResults.toString());
  }
  if (options?.format) {
    params.set('format', options.format);
  }

  const query = params.toString();
  return get<PerformanceCardRecommendation[]>(
    `/decks/${deckId}/recommendations/add${query ? `?${query}` : ''}`
  );
}

/**
 * Get card removal recommendations based on underperformance.
 * @param deckId - The deck ID
 * @param options - Optional query parameters
 */
export async function getPerformanceRemoveRecommendations(
  deckId: string,
  options?: {
    threshold?: number;
  }
): Promise<CardPerformance[]> {
  const params = new URLSearchParams();
  if (options?.threshold !== undefined) {
    params.set('threshold', options.threshold.toString());
  }

  const query = params.toString();
  return get<CardPerformance[]>(
    `/decks/${deckId}/recommendations/remove${query ? `?${query}` : ''}`
  );
}

/**
 * Get card swap recommendations based on performance.
 * @param deckId - The deck ID
 * @param options - Optional query parameters
 */
export async function getPerformanceSwapRecommendations(
  deckId: string,
  options?: {
    maxResults?: number;
    format?: string;
  }
): Promise<PerformanceCardRecommendation[]> {
  const params = new URLSearchParams();
  if (options?.maxResults !== undefined) {
    params.set('max_results', options.maxResults.toString());
  }
  if (options?.format) {
    params.set('format', options.format);
  }

  const query = params.toString();
  return get<PerformanceCardRecommendation[]>(
    `/decks/${deckId}/recommendations/swap${query ? `?${query}` : ''}`
  );
}

/**
 * Get all recommendations (add/remove/swap) for a deck.
 * @param deckId - The deck ID
 * @param options - Optional query parameters
 */
export async function getAllPerformanceRecommendations(
  deckId: string,
  options?: {
    maxResults?: number;
    format?: string;
  }
): Promise<DeckRecommendationsResponse> {
  const params = new URLSearchParams();
  if (options?.maxResults !== undefined) {
    params.set('max_results', options.maxResults.toString());
  }
  if (options?.format) {
    params.set('format', options.format);
  }

  const query = params.toString();
  return get<DeckRecommendationsResponse>(
    `/decks/${deckId}/recommendations/all${query ? `?${query}` : ''}`
  );
}

// ============================================================================
// Deck Permutation Types and Functions (Issue #889)
// ============================================================================

/**
 * Represents a card within a deck permutation snapshot.
 */
export interface DeckPermutationCard {
  card_id: number;
  quantity: number;
  board: string;
}

/**
 * Represents a change in card quantity between permutations.
 */
export interface DeckCardChange {
  card_id: number;
  old_quantity: number;
  new_quantity: number;
  board: string;
}

/**
 * Represents a deck permutation (version).
 */
export interface DeckPermutation {
  id: number;
  deckID: string;
  parentPermutationID?: number | null;
  cards: DeckPermutationCard[];
  versionNumber: number;
  versionName?: string | null;
  changeSummary?: string | null;
  matchesPlayed: number;
  matchesWon: number;
  matchWinRate: number;
  gamesPlayed: number;
  gamesWon: number;
  gameWinRate: number;
  createdAt: string;
  lastPlayedAt?: string | null;
  isCurrent: boolean;
}

/**
 * Represents the diff between two deck permutations.
 */
export interface DeckPermutationDiff {
  fromPermutationID: number;
  toPermutationID: number;
  addedCards: DeckPermutationCard[];
  removedCards: DeckPermutationCard[];
  changedCards: DeckCardChange[];
}

/**
 * Get all permutations (versions) of a deck.
 * @param deckId - The deck ID
 */
export async function getDeckPermutations(deckId: string): Promise<DeckPermutation[]> {
  return get<DeckPermutation[]>(`/decks/${deckId}/permutations`);
}

/**
 * Get the current permutation of a deck.
 * @param deckId - The deck ID
 */
export async function getCurrentDeckPermutation(deckId: string): Promise<DeckPermutation | null> {
  return get<DeckPermutation | null>(`/decks/${deckId}/permutations/current`);
}

/**
 * Get a specific permutation by ID.
 * @param deckId - The deck ID
 * @param permutationId - The permutation ID
 */
export async function getDeckPermutation(
  deckId: string,
  permutationId: number
): Promise<DeckPermutation> {
  return get<DeckPermutation>(`/decks/${deckId}/permutations/${permutationId}`);
}

/**
 * Get the diff between two permutations.
 * @param deckId - The deck ID
 * @param fromPermutationId - The source permutation ID
 * @param toPermutationId - The target permutation ID
 */
export async function getDeckPermutationDiff(
  deckId: string,
  fromPermutationId: number,
  toPermutationId: number
): Promise<DeckPermutationDiff> {
  return get<DeckPermutationDiff>(
    `/decks/${deckId}/permutations/${fromPermutationId}/diff/${toPermutationId}`
  );
}

/**
 * Update the name of a permutation.
 * @param deckId - The deck ID
 * @param permutationId - The permutation ID
 * @param name - The new name
 */
export async function updateDeckPermutationName(
  deckId: string,
  permutationId: number,
  name: string
): Promise<void> {
  await put(`/decks/${deckId}/permutations/${permutationId}/name`, { name });
}

/**
 * Restore a deck to a previous permutation.
 * @param deckId - The deck ID
 * @param permutationId - The permutation ID to restore
 */
export async function restoreDeckPermutation(
  deckId: string,
  permutationId: number
): Promise<void> {
  await post(`/decks/${deckId}/permutations/${permutationId}/restore`, {});
}

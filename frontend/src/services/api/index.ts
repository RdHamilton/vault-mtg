/**
 * API Services Index
 *
 * This module exports all API service functions, providing a centralized
 * access point for REST API communication. These services replace the
 * direct Wails function bindings.
 *
 * Usage:
 *   import { matches, drafts, decks } from '@/services/api';
 *
 *   const stats = await matches.getStats({ format: 'Standard' });
 *   const sessions = await drafts.getActiveDraftSessions();
 *   const deckList = await decks.getDecks();
 */

// API modules
export * as matches from './matches';
export * as drafts from './drafts';
export * as decks from './decks';
export * as cards from './cards';
export * as collection from './collection';
export * as system from './system';
export * as quests from './quests';
export * as meta from './meta';
export * as settings from './settings';
export * as standard from './standard';
export * as gameplays from './gameplays';
export * as notes from './notes';
export * as mlSuggestions from './mlSuggestions';
export * as opponents from './opponents';
export * as bffDraftRatings from './bffDraftRatings';
export * as bffAuth from './bffAuth';
export * as bffHealth from './bffHealth';
export * as bffMatchHistory from './bffMatchHistory';
export * as bffDraftHistory from './bffDraftHistory';
export * as bffDaemons from './bffDaemons';

// Re-export commonly used types
export type {
  StatsFilterRequest,
  TrendAnalysisRequest,
} from './matches';

export type {
  DraftFilterRequest,
  GradePickRequest,
  DraftInsightsRequest,
  WinProbabilityRequest,
} from './drafts';

export type {
  CreateDeckRequest,
  UpdateDeckRequest,
  ImportDeckApiRequest,
  ExportDeckApiRequest,
  SuggestDecksRequest,
  SuggestDecksApiResponse,
  AnalyzeDeckRequest,
} from './decks';

export type {
  CardSearchRequest,
} from './cards';

export type {
  CollectionFilter,
} from './collection';

export type {
  VersionInfo,
  DaemonStatus,
} from './system';

export type {
  StandardSet,
  StandardConfig,
  CardLegality,
  DeckValidationResult,
  RotationAffectedDeck,
  UpcomingRotation,
} from './standard';

export type {
  GamePlay,
  GameStateSnapshot,
  OpponentCard,
  PlayTimelineEntry,
  GamePlaySummary,
} from './gameplays';

export type {
  DeckNote,
  MatchNotes,
  ImprovementSuggestion,
  NoteCategory,
  SuggestionType,
  SuggestionPriority,
  CreateDeckNoteRequest,
  UpdateDeckNoteRequest,
  UpdateMatchNotesRequest,
} from './notes';

export type {
  MLSuggestion,
  MLSuggestionType,
  MLSuggestionReason,
  MLSuggestionResult,
  CardSynergyInfo,
  CardCombinationStats,
  CardPairSynergy,
  SynergyReport,
  UserPlayPatterns,
} from './mlSuggestions';

export type {
  BffCardRating,
  BffColorRating,
  BffDraftRatingsResponse,
  BffDraftRatingsResult,
} from './bffDraftRatings';

export type {
  CreateAPIKeyRequest,
  CreateAPIKeyResponse,
} from './bffAuth';

export type {
  DaemonHealthStatus,
  DaemonHealthResponse,
} from './bffHealth';

export type {
  MatchHistoryItem,
  MatchHistoryParams,
  MatchHistoryResponse,
} from './bffMatchHistory';

export type {
  DraftHistoryItem,
  DraftHistoryParams,
  DraftHistoryResponse,
} from './bffDraftHistory';

export type {
  DaemonDevice,
  ListDaemonsResponse,
} from './bffDaemons';

export type {
  OpponentDeckProfile,
  ObservedCard,
  ExpectedCard,
  StrategicInsight,
  MetaArchetypeMatch,
  MatchupStatistic,
  OpponentAnalysis,
  ArchetypeBreakdownEntry,
  ColorIdentityStatsEntry,
  OpponentHistorySummary,
  ArchetypeExpectedCard,
} from './opponents';

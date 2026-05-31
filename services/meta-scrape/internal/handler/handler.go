// Package handler is the Lambda-facing orchestration layer for the meta-scrape
// service (#177). It refreshes the metagame for every supported format, persists
// the aggregated archetypes, and wires the per-archetype card lists that #175/#176
// deferred to this ticket.
//
// Flow per format:
//  1. scraper.Service.RefreshAll fetches both sources, aggregates, and upserts the
//     archetype rows (mtgzone_archetypes) via the Service's own store.
//  2. The handler derives the card lists from the same aggregated result and, for
//     each archetype, looks up its id (store.ArchetypeIDByKey) and upserts the
//     card rows (store.UpsertArchetypeCards) into mtgzone_archetype_cards.
//
// A single source failing for one format never aborts the whole run: per-format
// errors are collected and logged, and the run reports how many formats succeeded.
// This mirrors the source-failure resilience contract (AC3) of the store layer.
package handler

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/scraper"
	"github.com/RdHamilton/vault-mtg/services/meta-scrape/internal/store"
	"github.com/jackc/pgx/v5"
)

// metaRefresher is the read/aggregate side the handler depends on.
// *scraper.Service satisfies it.
type metaRefresher interface {
	GetSupportedFormats() []string
	RefreshAll(ctx context.Context, format string) (*scraper.AggregatedMeta, error)
	CardListsFromMeta(meta *scraper.AggregatedMeta) []scraper.ArchetypeCardList
}

// cardStore is the write side for the archetype card lists. *store.MetaStore
// satisfies it. RefreshAll already persists the archetype rows themselves; this
// interface covers only the child-table (card-list) wiring owned by #177.
type cardStore interface {
	ArchetypeIDByKey(ctx context.Context, name, format string) (int64, error)
	UpsertArchetypeCards(ctx context.Context, archetypeID int64, cards []store.ArchetypeCard) error
}

// Handler orchestrates a full meta refresh across all supported formats.
type Handler struct {
	svc   metaRefresher
	cards cardStore
}

// New constructs a Handler. cards may be nil to skip the card-list wiring (e.g.
// a refresh-only run); in that case only archetype rows are persisted.
func New(svc metaRefresher, cards cardStore) *Handler {
	return &Handler{svc: svc, cards: cards}
}

// Result summarizes a Run for logging / Lambda response.
type Result struct {
	FormatsTotal      int `json:"formats_total"`
	FormatsSucceeded  int `json:"formats_succeeded"`
	ArchetypesWritten int `json:"archetypes_written"`
	CardListsWritten  int `json:"card_lists_written"`
}

// Handle is the aws-lambda-go entrypoint. The schedule event payload is ignored —
// every invocation runs a full refresh of all formats.
func (h *Handler) Handle(ctx context.Context) (Result, error) {
	return h.Run(ctx)
}

// Run refreshes every supported format and returns an aggregate Result. It
// returns an error only when EVERY format failed; a partial run (at least one
// format succeeded) is reported as success with the failure count logged, so a
// single flaky source never drops the whole scheduled refresh.
func (h *Handler) Run(ctx context.Context) (Result, error) {
	formats := h.svc.GetSupportedFormats()
	res := Result{FormatsTotal: len(formats)}

	var errs []error
	for _, format := range formats {
		written, cardLists, err := h.refreshFormat(ctx, format)
		if err != nil {
			log.Printf("[meta-scrape] format %q failed: %v", format, err)
			errs = append(errs, fmt.Errorf("format %q: %w", format, err))
			continue
		}
		res.FormatsSucceeded++
		res.ArchetypesWritten += written
		res.CardListsWritten += cardLists
	}

	log.Printf("[meta-scrape] run complete: %d/%d formats succeeded, %d archetypes, %d card lists",
		res.FormatsSucceeded, res.FormatsTotal, res.ArchetypesWritten, res.CardListsWritten)

	// Only surface an error when nothing succeeded — that is the genuine
	// "the whole refresh is broken" signal worth paging Sentry / the DLQ.
	if res.FormatsSucceeded == 0 && len(errs) > 0 {
		return res, fmt.Errorf("all %d formats failed: %w", len(errs), errors.Join(errs...))
	}

	return res, nil
}

// refreshFormat refreshes one format and wires its card lists. RefreshAll
// persists the archetype rows; this method then upserts each archetype's card
// list under the id resolved from the natural key.
func (h *Handler) refreshFormat(ctx context.Context, format string) (archetypes, cardLists int, err error) {
	meta, err := h.svc.RefreshAll(ctx, format)
	if err != nil {
		return 0, 0, err
	}
	if meta != nil {
		archetypes = len(meta.Archetypes)
	}

	if h.cards == nil || meta == nil {
		return archetypes, 0, nil
	}

	for _, list := range h.svc.CardListsFromMeta(meta) {
		id, idErr := h.cards.ArchetypeIDByKey(ctx, list.Name, list.Format)
		if idErr != nil {
			// ErrNoRows here is a genuine contract violation: RefreshAll just
			// upserted this archetype, so its key must exist. Surface it.
			if errors.Is(idErr, pgx.ErrNoRows) {
				log.Printf("[meta-scrape] archetype %q/%q missing after upsert (skipping cards): %v",
					list.Name, list.Format, idErr)
				continue
			}
			return archetypes, cardLists, fmt.Errorf("lookup archetype id %q/%q: %w", list.Name, list.Format, idErr)
		}

		if upErr := h.cards.UpsertArchetypeCards(ctx, id, list.Cards); upErr != nil {
			return archetypes, cardLists, fmt.Errorf("upsert cards for %q/%q: %w", list.Name, list.Format, upErr)
		}
		cardLists++
	}

	return archetypes, cardLists, nil
}

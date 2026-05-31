// Package scraper ports the MTGGoldfish and MTGTop8 metagame scrape clients and
// the aggregation service from the pre-purge internal/meta package
// (recovered verbatim from commit 4395c8a2^). It fetches public metagame pages,
// parses archetype meta-share and tournament statistics, aggregates them, and
// persists the result to Postgres via the store.MetaStore write side (#176).
//
// Scrape targets (mtggoldfish.com, mtgtop8.com) are declared as compile-time
// constants in the respective client files — they are never caller-, config-,
// or DB-controlled (security control SS-1). All HTTP egress in this package is
// to those two fixed public hosts.
package scraper

// userAgent is the polite User-Agent header sent on every outbound scrape
// request (control SP-2). It identifies the application so the upstream sites
// can attribute traffic; an empty or default net/http UA is never sent.
const userAgent = "VaultMTG/0.3.6"

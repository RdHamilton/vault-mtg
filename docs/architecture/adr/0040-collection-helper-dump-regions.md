# ADR-040: Collection-Helper --dump-regions Derivation Procedure

| Field       | Value                          |
|-------------|-------------------------------|
| **Status**  | Accepted                       |
| **Date**    | 2026-05-29                     |
| **Author**  | backend-engineer               |
| **Tickets** | vault-mtg-tickets#202, vault-mtg-tickets#237 |

---

## Context

The `collection-agent-helper` binary uses `task_for_pid` + `mach_vm_read` to read the
MTGA process's address space and extract the player's card collection. This path is
read-only and non-intrusive — it does not suspend, instrument, or otherwise disturb
the target process.

When an MTGA patch shifts the Unity heap layout, the collection scan may fail with
"no collection region found". To diagnose whether the failure is H1 (region filter
thresholds too strict) or H2 (Unity struct layout drift), a developer must re-derive
the memory signature offline. The `--dump-regions` mode exists for this purpose.

Because `--dump-regions` writes raw process memory to disk, the output directory
contains sensitive data (card collection IDs, account data) and must be treated as PII.

---

## Decision

### --dump-regions mode

`collection-agent-helper --dump-regions <PID> <outdir>` is the canonical tool for
offline signature derivation. It:

1. Calls `listReadableRegions` (mach_vm_region enumeration, read-only).
2. Calls `readMemory` (mach_vm_read, read-only) for each region >= `minRegionSize`
   with `VM_PROT_READ`.
3. Writes each region to `<outdir>/region_NNNN_0x<addr>.bin` with mode `0o640`.
4. Writes `<outdir>/manifest.json` with mode `0o640` (addresses, sizes, filenames only
   — no card data).
5. Logs a PII reminder: "IMPORTANT: delete `<outdir>` after analysis — contains raw
   process memory (PII)".

The tool must be run as root (same requirement as the production launchd service).
It must NOT be invoked while a debugger is attached to MTGA — see §Safety below.

### Output directory requirements

- `<outdir>` is sanitised with `filepath.Clean` before being passed to `os.MkdirAll`.
  This prevents path-traversal sequences (`../../etc/`) from being honoured.
- All `.bin` region files and `manifest.json` are written with mode `0o640` (root:staff,
  no world-read). Group-read is acceptable for developer workflows on a locked-down
  machine.

### Delete-after-use requirement

**The dump output directory MUST be deleted after offline analysis is complete.**
It contains raw process memory from a live MTGA instance, which includes:

- The player's full card collection (GRP IDs and quantities).
- Any other readable heap/stack data present at dump time.

Retention beyond the analysis session is a PII handling violation. Never commit or
transmit the dump directory. Typical cleanup:

```
rm -rf /tmp/collection-dump
```

The helper binary emits a runtime log reminder after every successful dump run.

### Signature versioning (§G4)

`CollectionSignatureVersion` follows the format `YYYYMMDD-NNN` (e.g. `20260529-001`).
It is bumped after every re-derivation pass. The CloudWatch `COLLECTION_SCAN_DRIFT`
alarm filter is keyed to this string — updating the constant without updating the
alarm filter silently breaks drift detection. Both must be updated together.

Future enhancement (v0.3.5): detect MTGA build string via task port Info.plist lookup
to correlate signatures with specific MTGA patch versions automatically.

### H1 vs H2 disambiguation

- **H1** (region filter too strict): at least one region in the dump returns
  >= `minEntries` hits when `analyze_dump` is run. Adjust `minEntries` or
  `maxFillPct` and update `CollectionSignatureVersion`.
- **H2** (Unity struct layout drift): no region returns hits. Inspect bytes around a
  known GRP ID in a `.bin` file to determine whether the 16-byte
  `[hashCode][next][key][value]` stride is intact. Fix `scanDictEntries` in
  `scanner.go` and update `CollectionSignatureVersion`.

---

## Safety

LLDB (or any debugger) MUST NOT be attached to MTGA during or before the dump.
Debugger attachment injects a SIGTRAP and uses the Mach exception port, suspending
all threads. MTGA/Unity anti-debug logic detects this and crashes the process.

`mach_vm_read` does not suspend the target. It is the same mechanism used by the
production collection scan on every sync. The `--dump-regions` mode is safe by
construction.

---

## Consequences

**Positive:**
- Provides a repeatable, safe procedure for re-deriving the collection signature after
  MTGA patches without requiring a debugger.
- `filepath.Clean` prevents path-traversal writes.
- Consistent `0o640` permissions on all output files.
- Runtime PII delete reminder reduces risk of inadvertent data retention.

**Negative / tradeoffs:**
- Raw dump is ~9.5 GB; disk space must be available on the developer machine.
- Root access required (same as production — not an additional constraint).
- The dump procedure is macOS-only (mach_vm_read is a Darwin API).

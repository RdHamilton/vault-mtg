#!/usr/bin/env python3
"""
lint-azure-signing-runner.py — Asserts that every job using
azure/trusted-signing-action runs on a windows-* runner.

Usage:
    python3 scripts/lint-azure-signing-runner.py <workflow-file>

Exit codes:
    0 — all azure/trusted-signing-action steps are on windows-* runners
    1 — one or more violations found (job name + runner printed to stderr)

Background:
    azure/trusted-signing-action requires a Windows environment. Running it
    on ubuntu-* or macos-* causes the signing step to fail at release time,
    not at PR time. This lint surfaces the misconfiguration early.

    Added as a follow-up to the v0.3.4 daemon release incident
    (vault-mtg-tickets#70, 2026-05-29). See ADR-041.
"""

import sys
import yaml


AZURE_SIGNING_ACTION_PREFIX = "azure/trusted-signing-action"
WINDOWS_RUNNER_PREFIX = "windows-"


def check_file(path: str) -> list[str]:
    """
    Parse the workflow YAML at `path` and return a list of violation strings.
    Each violation describes a job that uses azure/trusted-signing-action on
    a non-windows-* runner.
    """
    with open(path, encoding="utf-8") as f:
        doc = yaml.safe_load(f)

    if not isinstance(doc, dict):
        return [f"ERROR: {path} did not parse as a YAML mapping"]

    jobs = doc.get("jobs")
    if not isinstance(jobs, dict):
        # No jobs key — nothing to check
        return []

    violations = []
    for job_name, job in jobs.items():
        if not isinstance(job, dict):
            continue

        runs_on = job.get("runs-on", "")
        # runs-on may be a list (matrix) or a string; normalise to string for check
        if isinstance(runs_on, list):
            runs_on_str = " ".join(str(r) for r in runs_on)
        else:
            runs_on_str = str(runs_on)

        steps = job.get("steps") or []
        for step in steps:
            if not isinstance(step, dict):
                continue
            uses = step.get("uses", "") or ""
            if not uses.startswith(AZURE_SIGNING_ACTION_PREFIX):
                continue

            # This step uses azure/trusted-signing-action — assert windows-*
            step_name = step.get("name", "<unnamed step>")
            if not runs_on_str.startswith(WINDOWS_RUNNER_PREFIX):
                violations.append(
                    f"  Job '{job_name}' (step: '{step_name}')\n"
                    f"    uses: {uses}\n"
                    f"    runs-on: {runs_on_str!r}  <-- must start with 'windows-'"
                )

    return violations


def main() -> int:
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <workflow-file>", file=sys.stderr)
        return 2

    path = sys.argv[1]
    try:
        violations = check_file(path)
    except FileNotFoundError:
        print(f"ERROR: File not found: {path}", file=sys.stderr)
        return 2
    except yaml.YAMLError as exc:
        print(f"ERROR: Failed to parse {path} as YAML: {exc}", file=sys.stderr)
        return 2

    if violations:
        print("FAIL: azure/trusted-signing-action must only run on windows-* runners.\n")
        print(f"Violations found in {path}:\n")
        for v in violations:
            print(v)
        print(
            "\nFix: move the azure/trusted-signing-action step into a job whose\n"
            "'runs-on' begins with 'windows-' (e.g. 'windows-latest').\n"
            "The action requires a Windows environment and will error on linux/macOS.\n"
            "See ADR-041 and vault-mtg-tickets#70 for background."
        )
        return 1

    print(f"PASS: All azure/trusted-signing-action steps in {path} run on windows-* runners.")
    return 0


if __name__ == "__main__":
    sys.exit(main())

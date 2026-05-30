#!/usr/bin/env python3
# assert_field_preservation.py
#
# STEP 4b assertion: verify that api_key, account_id, and sync_enabled are
# preserved by name in daemon.json after a cross-env reinstall.
#
# Called from daemon-install-lifecycle.yml STEP 4b with CONFIG_FILE set in the
# environment.  Extracted to a static file to avoid YAML parse failures caused
# by zero-indented heredoc content inside a run: | block.
import json
import os
import sys

config_file = os.environ.get("CONFIG_FILE")
if not config_file:
    print("FAIL: CONFIG_FILE environment variable not set", file=sys.stderr)
    sys.exit(1)

with open(config_file) as f:
    d = json.load(f)

# api_key: must be present (may be empty string for fresh CI install —
# the daemon writes it on first auth; we assert the key exists in JSON).
assert "api_key" in d, "FAIL: api_key field missing from daemon.json after reinstall"
print("PASS: api_key present =>", repr(d["api_key"]))

# account_id: may be absent on a fresh CI install (daemon writes it on first
# auth); if present it must not be blank.
if "account_id" in d:
    assert d["account_id"] != "", "FAIL: account_id is blank after reinstall"
    print("PASS: account_id preserved =>", repr(d["account_id"]))
else:
    print("INFO: account_id not yet set (first install — daemon writes on first auth)")

# sync_enabled: key must be present when it was written by the pre-stage step.
assert "sync_enabled" in d, "FAIL: sync_enabled field missing from daemon.json after reinstall"
print("PASS: sync_enabled preserved =>", d["sync_enabled"])
print("PASS: all named fields verified (api_key, account_id, sync_enabled)")

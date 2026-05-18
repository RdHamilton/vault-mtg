#!/usr/bin/env bash
# truncate-staging-db.sh
#
# Wipes all user-data rows from vaultmtg_staging without dropping the schema.
# Reference tables (cards, sets, draft_card_ratings, etc.) are preserved by
# default. Pass --all to also truncate reference tables.
#
# Safe to run repeatedly. Does NOT affect the production database.
#
# Usage:
#   AWS_PROFILE=personal bash infra/scripts/truncate-staging-db.sh
#   AWS_PROFILE=personal bash infra/scripts/truncate-staging-db.sh --all
#
# Add to cron on EC2 for weekly resets (optional, per DBA-3 runbook):
#   0 3 * * 0 AWS_PROFILE=personal bash /opt/vaultmtg/infra/scripts/truncate-staging-db.sh >> /var/log/vaultmtg/truncate-staging.log 2>&1

set -euo pipefail

PROFILE="${AWS_PROFILE:-personal}"
REGION="${AWS_REGION:-us-east-1}"
TRUNCATE_ALL=false

# Source canonical deploy facts from the repo root.
_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${_SCRIPT_DIR}/../../infra/config/deploy-env.sh"

for arg in "$@"; do
    case "$arg" in
        --all) TRUNCATE_ALL=true ;;
        *) echo "Unknown argument: $arg"; exit 1 ;;
    esac
done

echo "[truncate-staging-db] Fetching staging connection details from SSM..."

DB_ENDPOINT=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "$SSM_PROD_DB_ENDPOINT" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_ARN=$(aws ssm get-parameter \
    --profile "$PROFILE" \
    --region  "$REGION" \
    --name    "$SSM_PROD_DB_SECRET_ARN" \
    --query   "Parameter.Value" \
    --output  text)

SECRET_JSON=$(aws secretsmanager get-secret-value \
    --profile   "$PROFILE" \
    --region    "$REGION" \
    --secret-id "$SECRET_ARN" \
    --query     "SecretString" \
    --output    text)

MASTER_PASSWORD=$(echo "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['password'])")
MASTER_USER=$(echo     "$SECRET_JSON" | python3 -c "import json,sys; print(json.load(sys.stdin)['username'])")

# ---------------------------------------------------------------------------
# User-data tables: all tables that have account_id or user_id FKs.
# Order matters for FK constraints — leaf tables first, then parents.
# ---------------------------------------------------------------------------
USER_DATA_TABLES=(
    # Leaf tables (depend on accounts or users)
    "daemon_events"
    "draft_picks"
    "draft_packs"
    "draft_match_results"
    "draft_archetype_stats"
    "draft_sessions"
    "game_plays"
    "game_state_snapshots"
    "games"
    "matches"
    "currency_history"
    "collection"
    "collection_history"
    "collection_new"
    "inventory"
    "inventory_history"
    "rank_history"
    "quests"
    "deck_cards"
    "deck_notes"
    "deck_tags"
    "deck_performance_history"
    "deck_permutations"
    "decks"
    "player_stats"
    "matchup_statistics"
    "opponent_cards_observed"
    "opponent_deck_profiles"
    "user_play_patterns"
    "processed_log_files"
    "api_keys"
    "settings"
    "sync_hashes"
    # Parent tables (depend on users)
    "accounts"
    "users"
)

# ---------------------------------------------------------------------------
# Reference tables: card data, ratings, archetypes — shared across users.
# Only truncated when --all is passed.
# ---------------------------------------------------------------------------
REFERENCE_TABLES=(
    "draft_card_ratings"
    "draft_color_ratings"
    "draft_community_comparison"
    "draft_events"
    "draft_pattern_analysis"
    "draft_temporal_trends"
    "card_affinity"
    "card_combination_stats"
    "card_cooccurrence"
    "card_embeddings"
    "card_frequency"
    "card_individual_stats"
    "card_similarity_cache"
    "cooccurrence_sources"
    "archetype_card_weights"
    "archetype_expected_cards"
    "deck_archetypes"
    "edhrec_card_metadata"
    "edhrec_synergy"
    "edhrec_theme_cards"
    "cfb_ratings"
    "mtgzone_archetype_cards"
    "mtgzone_archetypes"
    "mtgzone_articles"
    "mtgzone_synergies"
    "ml_model_metadata"
    "ml_suggestions"
    "dataset_metadata"
    "improvement_suggestions"
    "recommendation_feedback"
    "standard_config"
    "set_cards"
    "sets"
    "cards"
)

# Build the table list for this run.
TABLES_TO_TRUNCATE=("${USER_DATA_TABLES[@]}")
if [[ "$TRUNCATE_ALL" == "true" ]]; then
    TABLES_TO_TRUNCATE+=("${REFERENCE_TABLES[@]}")
    echo "[truncate-staging-db] WARNING: --all passed; reference tables will also be truncated."
fi

# Join into a comma-separated string for TRUNCATE ... CASCADE.
TABLES_CSV=$(IFS=', '; echo "${TABLES_TO_TRUNCATE[*]}")

echo "[truncate-staging-db] Tables to truncate: $TABLES_CSV"
echo ""
read -r -p "Type YES to proceed with truncation of vaultmtg_staging: " CONFIRM
if [[ "$CONFIRM" != "YES" ]]; then
    echo "[truncate-staging-db] Aborted."
    exit 0
fi

TRUNCATE_SQL="TRUNCATE TABLE ${TABLES_CSV} RESTART IDENTITY CASCADE;"

echo "[truncate-staging-db] Executing truncation..."

PGPASSWORD="$MASTER_PASSWORD" psql \
    -h "$DB_ENDPOINT" \
    -U "$MASTER_USER" \
    -d "$DB_STAGING_NAME" \
    -v ON_ERROR_STOP=1 \
    -c "$TRUNCATE_SQL"

echo ""
echo "[truncate-staging-db] Done. User data removed from vaultmtg_staging."
echo "  Schema and reference tables are intact."
echo ""
echo "To re-run migrations (no-op if already at HEAD):"
echo "  AWS_PROFILE=personal bash infra/scripts/run-staging-migrations.sh"

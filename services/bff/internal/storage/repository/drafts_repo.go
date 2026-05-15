package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidDraftPeriodType is returned by TemporalTrends when the
// period_type argument is not one of the accepted values (week|month).
var ErrInvalidDraftPeriodType = errors.New("period_type must be week|month")

// DraftsRepository serves the Phase 2 /api/v1/drafts/* surface — reads
// against draft_sessions + draft_picks + draft_temporal_trends +
// draft_community_comparison. The existing DraftSessionsRepository
// covers list + upsert; this new repo carries the per-session reads
// and the analytics tables.
type DraftsRepository struct {
	db DB
}

// NewDraftsRepository returns a DraftsRepository backed by db.
func NewDraftsRepository(db DB) *DraftsRepository {
	return &DraftsRepository{db: db}
}

// DraftSessionDetailRow mirrors a draft_sessions row + grade fields.
type DraftSessionDetailRow struct {
	ID                   string
	AccountID            *int64
	EventName            string
	SetCode              string
	DraftType            string
	StartTime            time.Time
	EndTime              *time.Time
	Status               string
	TotalPicks           int
	OverallGrade         *string
	OverallScore         *int
	PickQualityScore     *float64
	ColorDisciplineScore *float64
	DeckCompositionScore *float64
	StrategicScore       *float64
	PredictedWinRate     *float64
	PredictedWinRateMin  *float64
	PredictedWinRateMax  *float64
	PredictionFactors    *string
	PredictedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// DraftFilter narrows ListSessions queries.
type DraftFilter struct {
	Format    string
	SetCode   string
	StartDate *time.Time
	EndDate   *time.Time
	Status    string
	Limit     int
}

// ListSessions returns draft_sessions for the account, newest first.
func (r *DraftsRepository) ListSessions(ctx context.Context, accountID int64, f DraftFilter) ([]DraftSessionDetailRow, error) {
	clauses := []string{"account_id = $1"}
	args := []any{accountID}
	next := 2
	if f.SetCode != "" {
		clauses = append(clauses, "lower(set_code) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.SetCode)
		next++
	}
	if f.Format != "" {
		clauses = append(clauses, "lower(draft_type) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Format)
		next++
	}
	if f.Status != "" {
		clauses = append(clauses, "lower(status) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Status)
		next++
	}
	if f.StartDate != nil {
		clauses = append(clauses, "start_time >= $"+strconv.Itoa(next))
		args = append(args, *f.StartDate)
		next++
	}
	if f.EndDate != nil {
		clauses = append(clauses, "start_time <= $"+strconv.Itoa(next))
		args = append(args, *f.EndDate)
		next++
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	q := `SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time,
	             status, total_picks, overall_grade, overall_score, pick_quality_score,
	             color_discipline_score, deck_composition_score, strategic_score,
	             predicted_win_rate, predicted_win_rate_min, predicted_win_rate_max,
	             prediction_factors, predicted_at, created_at, updated_at
	      FROM draft_sessions
	      WHERE ` + strings.Join(clauses, " AND ") + `
	      ORDER BY start_time DESC
	      LIMIT $` + strconv.Itoa(next)
	args = append(args, limit)
	return r.scanSessions(ctx, q, args...)
}

// GetSession returns the session by id, scoped to account.
func (r *DraftsRepository) GetSession(ctx context.Context, accountID int64, sessionID string) (*DraftSessionDetailRow, error) {
	const q = `SELECT id, account_id, event_name, set_code, draft_type, start_time, end_time,
	                  status, total_picks, overall_grade, overall_score, pick_quality_score,
	                  color_discipline_score, deck_composition_score, strategic_score,
	                  predicted_win_rate, predicted_win_rate_min, predicted_win_rate_max,
	                  prediction_factors, predicted_at, created_at, updated_at
	           FROM draft_sessions
	           WHERE account_id = $1 AND id = $2
	           LIMIT 1`
	rows, err := r.scanSessions(ctx, q, accountID, sessionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// DistinctSets returns the set codes the account has draft sessions for.
func (r *DraftsRepository) DistinctSets(ctx context.Context, accountID int64) ([]string, error) {
	const q = `SELECT DISTINCT set_code FROM draft_sessions
	           WHERE account_id = $1 AND set_code <> ''
	           ORDER BY set_code`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DraftPickRow mirrors draft_picks.
type DraftPickRow struct {
	ID               int64
	SessionID        string
	PackNumber       int
	PickNumber       int
	CardID           string
	Timestamp        time.Time
	PickQualityGrade *string
	PickQualityRank  *int
	PackBestGIHWR    *float64
	PickedCardGIHWR  *float64
	AlternativesJSON *string
}

// PicksForSession returns all picks for sessionID, scoped via
// draft_sessions.account_id.
func (r *DraftsRepository) PicksForSession(ctx context.Context, accountID int64, sessionID string) ([]DraftPickRow, error) {
	const q = `SELECT p.id, p.session_id, p.pack_number, p.pick_number, p.card_id,
	                  p.timestamp, p.pick_quality_grade, p.pick_quality_rank,
	                  p.pack_best_gihwr, p.picked_card_gihwr, p.alternatives_json
	           FROM draft_picks p
	           JOIN draft_sessions s ON s.id = p.session_id
	           WHERE s.account_id = $1 AND p.session_id = $2
	           ORDER BY p.pack_number, p.pick_number`
	rows, err := r.db.QueryContext(ctx, q, accountID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DraftPickRow
	for rows.Next() {
		var p DraftPickRow
		if err := rows.Scan(
			&p.ID, &p.SessionID, &p.PackNumber, &p.PickNumber, &p.CardID,
			&p.Timestamp, &p.PickQualityGrade, &p.PickQualityRank,
			&p.PackBestGIHWR, &p.PickedCardGIHWR, &p.AlternativesJSON,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DraftStatsAggregate is the aggregated stats over a filtered set of
// draft sessions.
type DraftStatsAggregate struct {
	TotalDrafts       int
	CompletedDrafts   int
	AvgOverallScore   *float64
	AvgPickQuality    *float64
	AvgPredictedWR    *float64
	GradeDistribution map[string]int
}

// AggregateStats runs a single grouped query for the filter and
// computes the grade distribution from the result set.
func (r *DraftsRepository) AggregateStats(ctx context.Context, accountID int64, f DraftFilter) (DraftStatsAggregate, error) {
	clauses := []string{"account_id = $1"}
	args := []any{accountID}
	next := 2
	if f.SetCode != "" {
		clauses = append(clauses, "lower(set_code) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.SetCode)
		next++
	}
	if f.Format != "" {
		clauses = append(clauses, "lower(draft_type) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Format)
		next++
	}
	if f.Status != "" {
		clauses = append(clauses, "lower(status) = lower($"+strconv.Itoa(next)+")")
		args = append(args, f.Status)
		next++
	}
	if f.StartDate != nil {
		clauses = append(clauses, "start_time >= $"+strconv.Itoa(next))
		args = append(args, *f.StartDate)
		next++
	}
	if f.EndDate != nil {
		clauses = append(clauses, "start_time <= $"+strconv.Itoa(next))
		args = append(args, *f.EndDate)
		next++
	}
	q := `SELECT
	        COUNT(*),
	        COUNT(*) FILTER (WHERE lower(status) = 'completed'),
	        AVG(overall_score) FILTER (WHERE overall_score IS NOT NULL),
	        AVG(pick_quality_score) FILTER (WHERE pick_quality_score IS NOT NULL),
	        AVG(predicted_win_rate) FILTER (WHERE predicted_win_rate IS NOT NULL)
	      FROM draft_sessions
	      WHERE ` + strings.Join(clauses, " AND ")
	var s DraftStatsAggregate
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(
		&s.TotalDrafts, &s.CompletedDrafts, &s.AvgOverallScore, &s.AvgPickQuality, &s.AvgPredictedWR,
	); err != nil {
		return DraftStatsAggregate{}, err
	}
	gradeQ := `SELECT COALESCE(overall_grade, ''), COUNT(*)
	           FROM draft_sessions
	           WHERE ` + strings.Join(clauses, " AND ") + `
	             AND overall_grade IS NOT NULL
	           GROUP BY overall_grade
	           ORDER BY overall_grade`
	rows, err := r.db.QueryContext(ctx, gradeQ, args...)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	s.GradeDistribution = map[string]int{}
	for rows.Next() {
		var grade string
		var count int
		if err := rows.Scan(&grade, &count); err != nil {
			return s, err
		}
		s.GradeDistribution[grade] = count
	}
	return s, rows.Err()
}

// CommunityComparisonRow mirrors draft_community_comparison.
type CommunityComparisonRow struct {
	SetCode             string
	DraftFormat         string
	UserWinRate         float64
	CommunityAvgWinRate float64
	PercentileRank      *float64
	SampleSize          int
	CalculatedAt        time.Time
}

// CommunityComparisons returns every cached comparison.
func (r *DraftsRepository) CommunityComparisons(ctx context.Context) ([]CommunityComparisonRow, error) {
	const q = `SELECT set_code, draft_format, user_win_rate, community_avg_win_rate,
	                  percentile_rank, sample_size, calculated_at
	           FROM draft_community_comparison
	           ORDER BY calculated_at DESC`
	return r.scanComparisons(ctx, q)
}

// CommunityComparisonForSet returns a single comparison row for the set.
// Returns nil when none is cached.
func (r *DraftsRepository) CommunityComparisonForSet(ctx context.Context, setCode, format string) (*CommunityComparisonRow, error) {
	clauses := "lower(set_code) = lower($1)"
	args := []any{setCode}
	if format != "" {
		clauses += " AND lower(draft_format) = lower($2)"
		args = append(args, format)
	}
	q := `SELECT set_code, draft_format, user_win_rate, community_avg_win_rate,
	             percentile_rank, sample_size, calculated_at
	      FROM draft_community_comparison
	      WHERE ` + clauses + `
	      ORDER BY calculated_at DESC
	      LIMIT 1`
	rows, err := r.scanComparisons(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// TemporalTrendRow mirrors draft_temporal_trends.
type TemporalTrendRow struct {
	PeriodType    string
	PeriodStart   time.Time
	PeriodEnd     time.Time
	SetCode       string
	DraftsCount   int
	MatchesPlayed int
	MatchesWon    int
	AvgDraftGrade *float64
	CalculatedAt  time.Time
}

// TemporalTrends returns trend rows filtered by period type. setCode may
// be empty to match all sets; numPeriods caps the result count.
// periodType must be "week" or "month"; anything else returns ErrInvalidDraftPeriodType.
func (r *DraftsRepository) TemporalTrends(ctx context.Context, periodType, setCode string, numPeriods int) ([]TemporalTrendRow, error) {
	if periodType != "week" && periodType != "month" {
		return nil, ErrInvalidDraftPeriodType
	}
	if numPeriods <= 0 {
		numPeriods = 12
	}
	if numPeriods > 100 {
		numPeriods = 100
	}
	clauses := "period_type = $1"
	args := []any{periodType}
	if setCode != "" {
		clauses += " AND lower(set_code) = lower($2)"
		args = append(args, setCode)
	}
	q := `SELECT period_type, period_start, period_end, set_code,
	             drafts_count, matches_played, matches_won, avg_draft_grade,
	             calculated_at
	      FROM draft_temporal_trends
	      WHERE ` + clauses + `
	      ORDER BY period_start DESC
	      LIMIT ` + strconv.Itoa(numPeriods)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TemporalTrendRow
	for rows.Next() {
		var t TemporalTrendRow
		if err := rows.Scan(
			&t.PeriodType, &t.PeriodStart, &t.PeriodEnd, &t.SetCode,
			&t.DraftsCount, &t.MatchesPlayed, &t.MatchesWon,
			&t.AvgDraftGrade, &t.CalculatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// LearningCurve returns the trend rows narrowed to one set, oldest
// first. The handler folds these into the SPA's LearningCurveResponse.
func (r *DraftsRepository) LearningCurve(ctx context.Context, setCode string) ([]TemporalTrendRow, error) {
	const q = `SELECT period_type, period_start, period_end, set_code,
	                  drafts_count, matches_played, matches_won, avg_draft_grade,
	                  calculated_at
	           FROM draft_temporal_trends
	           WHERE lower(set_code) = lower($1)
	           ORDER BY period_start`
	rows, err := r.db.QueryContext(ctx, q, setCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TemporalTrendRow
	for rows.Next() {
		var t TemporalTrendRow
		if err := rows.Scan(
			&t.PeriodType, &t.PeriodStart, &t.PeriodEnd, &t.SetCode,
			&t.DraftsCount, &t.MatchesPlayed, &t.MatchesWon,
			&t.AvgDraftGrade, &t.CalculatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RecommendationFeedbackStatsRow is the aggregated counts pulled from
// recommendation_feedback for /feedback/stats.
type RecommendationFeedbackStatsRow struct {
	TotalRecommendations int
	Accepted             int
	Rejected             int
	WinRateImpact        *float64
}

// RecommendationFeedbackStats returns the aggregate feedback stats. The
// schema's recommendation_feedback table carries the raw events; we
// aggregate inline. WinRateImpact is the proportion of acted-on
// recommendations that won (a coarse proxy until a richer outcome score
// is added).
func (r *DraftsRepository) RecommendationFeedbackStats(ctx context.Context, accountID int64) (RecommendationFeedbackStatsRow, error) {
	const q = `SELECT COUNT(*),
	                  COUNT(*) FILTER (WHERE lower(action) = 'accepted'),
	                  COUNT(*) FILTER (WHERE lower(action) = 'rejected'),
	                  CASE WHEN COUNT(*) FILTER (WHERE outcome_result IS NOT NULL) > 0
	                       THEN COUNT(*) FILTER (WHERE outcome_result = 'win')::DOUBLE PRECISION
	                          / COUNT(*) FILTER (WHERE outcome_result IS NOT NULL)
	                       ELSE NULL END
	           FROM recommendation_feedback
	           WHERE account_id = $1`
	var s RecommendationFeedbackStatsRow
	if err := r.db.QueryRowContext(ctx, q, accountID).Scan(
		&s.TotalRecommendations, &s.Accepted, &s.Rejected, &s.WinRateImpact,
	); err != nil {
		// recommendation_feedback table exists but might be empty for new
		// accounts; treat ErrNoRows as zero counts.
		if err == sql.ErrNoRows {
			return RecommendationFeedbackStatsRow{}, nil
		}
		return RecommendationFeedbackStatsRow{}, err
	}
	return s, nil
}

// scanSessions centralises draft_sessions row decoding.
func (r *DraftsRepository) scanSessions(ctx context.Context, q string, args ...any) ([]DraftSessionDetailRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DraftSessionDetailRow
	for rows.Next() {
		var s DraftSessionDetailRow
		if err := rows.Scan(
			&s.ID, &s.AccountID, &s.EventName, &s.SetCode, &s.DraftType,
			&s.StartTime, &s.EndTime, &s.Status, &s.TotalPicks,
			&s.OverallGrade, &s.OverallScore, &s.PickQualityScore,
			&s.ColorDisciplineScore, &s.DeckCompositionScore, &s.StrategicScore,
			&s.PredictedWinRate, &s.PredictedWinRateMin, &s.PredictedWinRateMax,
			&s.PredictionFactors, &s.PredictedAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// scanComparisons centralises draft_community_comparison row decoding.
func (r *DraftsRepository) scanComparisons(ctx context.Context, q string, args ...any) ([]CommunityComparisonRow, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommunityComparisonRow
	for rows.Next() {
		var c CommunityComparisonRow
		if err := rows.Scan(
			&c.SetCode, &c.DraftFormat, &c.UserWinRate, &c.CommunityAvgWinRate,
			&c.PercentileRank, &c.SampleSize, &c.CalculatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

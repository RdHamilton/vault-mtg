package seventeenlands_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchCardRatings(t *testing.T) {
	t.Run("returns ratings on 200", func(t *testing.T) {
		fixture := []seventeenlands.CardRating{
			{MtgaID: 12345, Name: "Lightning Bolt", ALSA: 1.5, ATA: 1.8, GIHWR: 0.62, SeenCount: 1000},
			{MtgaID: 67890, Name: "Island", ALSA: 8.0, ATA: 8.5, GIHWR: 0.55, SeenCount: 500},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/card_ratings/data", r.URL.Path)
			assert.Equal(t, "FDN", r.URL.Query().Get("expansion"))
			assert.Equal(t, "PremierDraft", r.URL.Query().Get("format"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fixture)
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		ratings, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

		require.NoError(t, err)
		require.Len(t, ratings, 2)
		assert.Equal(t, 12345, ratings[0].MtgaID)
		assert.Equal(t, "Lightning Bolt", ratings[0].Name)
		assert.InDelta(t, 1.5, ratings[0].ALSA, 0.001)
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		_, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "503")
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not-json"))
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		_, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

		require.Error(t, err)
	})
}

func TestFetchColorRatings(t *testing.T) {
	t.Run("returns color ratings on 200", func(t *testing.T) {
		fixture := []seventeenlands.ColorRating{
			{ColorCombination: "WU", WinRate: 0.58, GamesPlayed: 5000},
			{ColorCombination: "BG", WinRate: 0.52, GamesPlayed: 3200},
			{ColorCombination: "R", WinRate: 0.49, GamesPlayed: 2100},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/color_ratings/data", r.URL.Path)
			assert.Equal(t, "FDN", r.URL.Query().Get("expansion"))
			assert.Equal(t, "PremierDraft", r.URL.Query().Get("format"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(fixture)
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		ratings, err := client.FetchColorRatings(context.Background(), "FDN", "PremierDraft")

		require.NoError(t, err)
		require.Len(t, ratings, 3)
		assert.Equal(t, "WU", ratings[0].ColorCombination)
		assert.InDelta(t, 0.58, ratings[0].WinRate, 0.001)
		assert.Equal(t, 5000, ratings[0].GamesPlayed)
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		_, err := client.FetchColorRatings(context.Background(), "FDN", "PremierDraft")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not-json"))
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		_, err := client.FetchColorRatings(context.Background(), "FDN", "PremierDraft")

		require.Error(t, err)
	})

	t.Run("returns empty slice when no color data", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
		}))
		defer srv.Close()

		client := seventeenlands.NewClientWithBase(srv.URL, srv.Client())
		ratings, err := client.FetchColorRatings(context.Background(), "FDN", "PremierDraft")

		require.NoError(t, err)
		assert.Empty(t, ratings)
	})
}

// --- AC2/AC3: retry + backoff tests using NewClientWithOptions ---

// TestFetchCardRatings_RetryOn429 verifies that the client retries a 429 response
// and succeeds on the subsequent attempt.
func TestFetchCardRatings_RetryOn429(t *testing.T) {
	fixture := []seventeenlands.CardRating{
		{MtgaID: 1, Name: "Lightning Bolt", ALSA: 1.5},
	}

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call: return 429.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Second call: return success.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	}))
	defer srv.Close()

	// 2 max attempts, 1 ms base backoff so the test runs quickly.
	client := seventeenlands.NewClientWithOptions(srv.URL, srv.Client(), 2, 1*time.Millisecond)
	ratings, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, 1, ratings[0].MtgaID)
	assert.Equal(t, int32(2), callCount.Load(), "expected exactly 2 HTTP calls: 1 retry + 1 success")
}

// TestFetchCardRatings_RetryOn5xx verifies that the client retries a 503 response
// and succeeds on the subsequent attempt.
func TestFetchCardRatings_RetryOn5xx(t *testing.T) {
	fixture := []seventeenlands.CardRating{
		{MtgaID: 99, Name: "Island", ALSA: 8.0},
	}

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	}))
	defer srv.Close()

	client := seventeenlands.NewClientWithOptions(srv.URL, srv.Client(), 2, 1*time.Millisecond)
	ratings, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, int32(2), callCount.Load(), "expected 2 HTTP calls: 1 retry + 1 success")
}

// TestFetchCardRatings_RetryExhausted verifies that after exhausting all attempts the
// client returns the final retryable status as an error (not a nil-error success).
func TestFetchCardRatings_RetryExhausted(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	// 3 total attempts, all return 429.
	client := seventeenlands.NewClientWithOptions(srv.URL, srv.Client(), 3, 1*time.Millisecond)
	_, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Equal(t, int32(3), callCount.Load(), "expected exactly 3 total attempts")
}

// TestFetchCardRatings_NoRetryOn4xx verifies that 4xx responses (other than 429) are
// not retried — they are permanent client errors.
func TestFetchCardRatings_NoRetryOn4xx(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := seventeenlands.NewClientWithOptions(srv.URL, srv.Client(), 3, 1*time.Millisecond)
	_, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Equal(t, int32(1), callCount.Load(), "404 must not be retried")
}

// TestFetchCardRatings_BackoffTiming verifies that backoff durations grow exponentially.
// The test uses a 10 ms base backoff with 3 total attempts (1 initial + 2 retries) and
// measures total elapsed time. 10ms + 20ms = 30ms minimum; we assert >= 25ms to allow
// for scheduler jitter.
func TestFetchCardRatings_BackoffTiming(t *testing.T) {
	fixture := []seventeenlands.CardRating{{MtgaID: 1, Name: "Opt"}}

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	}))
	defer srv.Close()

	const baseBackoff = 10 * time.Millisecond
	client := seventeenlands.NewClientWithOptions(srv.URL, srv.Client(), 3, baseBackoff)

	start := time.Now()
	ratings, err := client.FetchCardRatings(context.Background(), "FDN", "PremierDraft")
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, int32(3), callCount.Load())

	// attempt 1 → sleep baseBackoff (10ms), attempt 2 → sleep 2*baseBackoff (20ms)
	minExpected := baseBackoff + 2*baseBackoff - 5*time.Millisecond // subtract jitter budget
	assert.GreaterOrEqual(t, elapsed, minExpected,
		"elapsed %v should be >= %v (two backoff sleeps)", elapsed, minExpected)
}

// TestFetchColorRatings_RetryOn429 mirrors TestFetchCardRatings_RetryOn429 for the
// /color_ratings/data endpoint.
func TestFetchColorRatings_RetryOn429(t *testing.T) {
	fixture := []seventeenlands.ColorRating{
		{ColorCombination: "WU", WinRate: 0.58, GamesPlayed: 5000},
	}

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	}))
	defer srv.Close()

	client := seventeenlands.NewClientWithOptions(srv.URL, srv.Client(), 2, 1*time.Millisecond)
	ratings, err := client.FetchColorRatings(context.Background(), "FDN", "PremierDraft")

	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, int32(2), callCount.Load())
}

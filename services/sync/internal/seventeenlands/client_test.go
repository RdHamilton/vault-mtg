package seventeenlands_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonehamilton/mtga-sync/internal/seventeenlands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchCardRatings(t *testing.T) {
	t.Run("returns ratings on 200", func(t *testing.T) {
		fixture := []seventeenlands.CardRating{
			{Name: "Lightning Bolt", ALSA: 1.5, ATA: 1.8, GIHWR: 0.62, SeenCount: 1000},
			{Name: "Island", ALSA: 8.0, ATA: 8.5, GIHWR: 0.55, SeenCount: 500},
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

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ramonehamilton/MTGA-Companion/internal/gui"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage"
	"github.com/ramonehamilton/MTGA-Companion/internal/storage/models"
)

func setupTestStorage(t *testing.T) (*storage.Service, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := storage.DefaultConfig(dbPath)
	config.AutoMigrate = true
	db, err := storage.Open(config)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	service := storage.NewService(db)
	cleanup := func() {
		_ = service.Close()
		_ = db.Close()
	}

	return service, cleanup
}

func TestCFBHandler_GetCFBRatings_WithPreseededData(t *testing.T) {
	storageService, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Seed a CFB rating into the database
	cfbRepo := storageService.NewCFBRatingsRepo()
	err := cfbRepo.UpsertRating(ctx, &models.CFBRating{
		CardName:      "Lightning Bolt",
		SetCode:       "ECL",
		LimitedRating: 4.5,
		LimitedScore:  0.9,
		Author:        "MTG Arena Zone",
		ImportedAt:    time.Now(),
		UpdatedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to seed CFB rating: %v", err)
	}

	// Create facade with services (no MTGAZoneFetcher needed for pre-seeded data)
	services := &gui.Services{
		Context: ctx,
		Storage: storageService,
	}
	facade := gui.NewCardFacade(services)
	handler := NewCFBHandler(facade)

	// Create request with chi URL param
	r := chi.NewRouter()
	r.Get("/cfb/{setCode}", handler.GetCFBRatings)

	req := httptest.NewRequest(http.MethodGet, "/cfb/ECL", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify response contains the seeded rating
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected at least one rating in response, got empty slice")
	}
	found := false
	for _, r := range resp.Data {
		if name, ok := r["cardName"].(string); ok && name == "Lightning Bolt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected seeded card 'Lightning Bolt' in response, got: %s", rec.Body.String())
	}
}

func TestCFBHandler_GetCFBRatings_EmptySetReturnsOK(t *testing.T) {
	storageService, cleanup := setupTestStorage(t)
	defer cleanup()

	// No MTGAZoneFetcher = nil auto-fetch, empty DB = empty response
	services := &gui.Services{
		Context: context.Background(),
		Storage: storageService,
	}
	facade := gui.NewCardFacade(services)
	handler := NewCFBHandler(facade)

	r := chi.NewRouter()
	r.Get("/cfb/{setCode}", handler.GetCFBRatings)

	req := httptest.NewRequest(http.MethodGet, "/cfb/UNKNOWN", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty set, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCFBHandler_GetCFBRatings_MissingSetCode(t *testing.T) {
	storageService, cleanup := setupTestStorage(t)
	defer cleanup()

	services := &gui.Services{
		Context: context.Background(),
		Storage: storageService,
	}
	facade := gui.NewCardFacade(services)
	handler := NewCFBHandler(facade)

	req := httptest.NewRequest(http.MethodGet, "/cfb/", nil)
	rec := httptest.NewRecorder()

	// Call handler directly (no chi routing) so setCode is empty
	handler.GetCFBRatings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing set code, got %d", rec.Code)
	}
}

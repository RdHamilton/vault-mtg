package sentryhook

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

func TestInit_EmptyDSNReturnsErrDisabled(t *testing.T) {
	if err := Init("", "v1.2.3", "http://localhost:8080/api/v1"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Init(empty dsn) = %v, want %v", err, ErrDisabled)
	}
}

func TestEnvironmentFromCloudAPIURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://api.vaultmtg.app/api/v1", "production"},
		{"https://staging-api.vaultmtg.app/api/v1", "staging"},
		{"http://localhost:8080/api/v1", "development"},
		{"", "development"},
	}
	for _, c := range cases {
		if got := environmentFromCloudAPIURL(c.url); got != c.want {
			t.Errorf("environmentFromCloudAPIURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestScrub_BearerToken(t *testing.T) {
	in := "request failed: Authorization=Bearer abc.def.ghi-jkl"
	out := Scrub(in)
	if out == in {
		t.Fatalf("Scrub did not redact bearer token: %q", out)
	}
	if want := "[REDACTED]"; !contains(out, want) {
		t.Errorf("Scrub output missing %q sentinel: %q", want, out)
	}
}

func TestScrub_ClerkPublishableKey(t *testing.T) {
	in := "client init: pk_live_Y2xlcmsuc3RnLWFwcC52YXVsdG10Zy5hcHAk failed"
	out := Scrub(in)
	if out == in {
		t.Fatalf("Scrub did not redact Clerk pk_live key: %q", out)
	}
}

func TestScrub_ClerkSecretKey(t *testing.T) {
	in := "CLERK_SECRET_KEY=sk_test_AbCdEfGhIjKlMnOpQrStUvWxYz exposed"
	out := Scrub(in)
	if contains(out, "sk_test_AbCdEfGhIjKlMnOpQrStUvWxYz") {
		t.Errorf("Scrub did not redact sk_test_ key: %q", out)
	}
}

func TestScrub_SentryDSN(t *testing.T) {
	in := "init with dsn https://deadbeefcafe1234@o12345.ingest.sentry.io/67890 ok"
	out := Scrub(in)
	if contains(out, "deadbeefcafe1234") || contains(out, "o12345.ingest.sentry.io") {
		t.Errorf("Scrub did not redact Sentry DSN: %q", out)
	}
}

func TestScrub_APIKeyPattern(t *testing.T) {
	in := `config: api_key="aabbccddeeff00112233" loaded`
	out := Scrub(in)
	if contains(out, "aabbccddeeff00112233") {
		t.Errorf("Scrub did not redact api_key: %q", out)
	}
}

func TestScrub_JWT(t *testing.T) {
	in := "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NSJ9.signature-bytes-here"
	out := Scrub(in)
	if contains(out, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("Scrub did not redact JWT: %q", out)
	}
}

func TestScrub_Idempotent(t *testing.T) {
	clean := "no secrets here, just a friendly message"
	if got := Scrub(clean); got != clean {
		t.Errorf("Scrub mutated clean string: %q -> %q", clean, got)
	}
}

func TestScrubEvent_RedactsMessageAndException(t *testing.T) {
	evt := &sentry.Event{
		Message: "Authorization=Bearer secret-token-here failed",
		Exception: []sentry.Exception{
			{Value: "dispatch panic: pk_live_LeakedKeyValue"},
		},
		Contexts: map[string]sentry.Context{
			"daemon": {
				"context": "running with sk_live_ExtraLeak",
				"count":   42,
			},
		},
		Breadcrumbs: []*sentry.Breadcrumb{
			{Message: "fetched Bearer xyz789-token"},
		},
	}
	out := scrubEvent(evt, nil)
	if contains(out.Message, "secret-token-here") {
		t.Errorf("scrubEvent left bearer in Message: %q", out.Message)
	}
	if contains(out.Exception[0].Value, "pk_live_LeakedKeyValue") {
		t.Errorf("scrubEvent left Clerk key in Exception: %q", out.Exception[0].Value)
	}
	daemonCtx, ok := out.Contexts["daemon"]
	if !ok {
		t.Fatal("scrubEvent dropped the daemon Context")
	}
	if s, ok := daemonCtx["context"].(string); !ok || contains(s, "sk_live_ExtraLeak") {
		t.Errorf("scrubEvent left Clerk secret in Context: %v", daemonCtx["context"])
	}
	// Non-string Context values must pass through untouched.
	if v, ok := daemonCtx["count"].(int); !ok || v != 42 {
		t.Errorf("scrubEvent mutated non-string Context value: %v", daemonCtx["count"])
	}
	if contains(out.Breadcrumbs[0].Message, "xyz789-token") {
		t.Errorf("scrubEvent left bearer in Breadcrumb: %q", out.Breadcrumbs[0].Message)
	}
}

func TestScrubEvent_NilEvent(t *testing.T) {
	if got := scrubEvent(nil, nil); got != nil {
		t.Errorf("scrubEvent(nil) = %v, want nil", got)
	}
}

func TestScrubEvent_RedactsAuthorizationHeader(t *testing.T) {
	evt := &sentry.Event{
		Request: &sentry.Request{
			Headers: map[string]string{
				"Authorization": "Bearer abc.def.ghi",
				"Cookie":        "session=xyz",
				"User-Agent":    "vaultmtg-daemon/v1.0",
			},
		},
	}
	out := scrubEvent(evt, nil)
	if got := out.Request.Headers["Authorization"]; got != "[REDACTED]" {
		t.Errorf("Authorization header not fully redacted: %q", got)
	}
	if got := out.Request.Headers["Cookie"]; got != "[REDACTED]" {
		t.Errorf("Cookie header not fully redacted: %q", got)
	}
	if got := out.Request.Headers["User-Agent"]; got != "vaultmtg-daemon/v1.0" {
		t.Errorf("User-Agent was mutated unexpectedly: %q", got)
	}
}

// fakeTransport is a sentry.Transport that records every event it receives.
// Used by TestPanicCapture to verify the panic→Sentry pipeline end-to-end
// without contacting the real Sentry ingest endpoint.
type fakeTransport struct {
	events []*sentry.Event
}

func (f *fakeTransport) Configure(_ sentry.ClientOptions) {}
func (f *fakeTransport) SendEvent(event *sentry.Event)    { f.events = append(f.events, event) }
func (f *fakeTransport) Flush(_ time.Duration) bool       { return true }
func (f *fakeTransport) FlushWithContext(_ context.Context) bool {
	return true
}
func (f *fakeTransport) Close() {}

func TestPanicCapture_EndToEnd(t *testing.T) {
	// AC: "Integration test verifies panic → Sentry path (mock Sentry transport)"
	// Wire a fake transport into the Sentry client, fire a panic inside a
	// goroutine that uses sentry.CurrentHub().Recover, and assert the event
	// was delivered with the expected release tag.
	tr := &fakeTransport{}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://test@test.ingest.sentry.io/12345",
		Release:   "v1.2.3-test",
		Transport: tr,
	})
	if err != nil {
		t.Fatalf("sentry.Init with fake transport: %v", err)
	}

	// Simulate the goroutine recover pattern used in services/daemon/internal/daemon/service.go.
	func() {
		defer func() {
			if r := recover(); r != nil {
				sentry.CurrentHub().Recover(r)
			}
		}()
		panic("simulated daemon panic for test")
	}()

	sentry.Flush(2 * time.Second)

	if len(tr.events) == 0 {
		t.Fatal("no events captured by fake transport")
	}
	got := tr.events[0]
	if got.Release != "v1.2.3-test" {
		t.Errorf("event release = %q, want v1.2.3-test", got.Release)
	}
	// The Recover call records the panic value either as an Exception (typed
	// error panic) or as the event Message (string panic). Verify it survives
	// in either location.
	hay := got.Message
	for _, ex := range got.Exception {
		hay += "|" + ex.Value
	}
	if !contains(hay, "simulated daemon panic for test") {
		t.Errorf("panic message not found in event (msg=%q exceptions=%+v)", got.Message, got.Exception)
	}
}

// contains is a substring helper kept local so the tests have zero external
// dependencies beyond sentry-go itself.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

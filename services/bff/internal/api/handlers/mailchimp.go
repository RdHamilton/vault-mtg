package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// MailchimpHTTPClient implements mailchimpClient using the Mailchimp Marketing
// API v3. It requires an API key (format: <key>-<datacenter>) and the list ID
// of the audience to subscribe members to.
//
// Authentication uses HTTP Basic Auth with "anystring" as the username and the
// API key as the password, per Mailchimp API spec.
type MailchimpHTTPClient struct {
	apiKey     string
	listID     string
	datacenter string
	httpClient *http.Client
}

// NewMailchimpHTTPClient constructs a client from an API key (e.g.
// "abc123-us1") and listID. Returns an error if the key format is invalid.
func NewMailchimpHTTPClient(apiKey, listID string) (*MailchimpHTTPClient, error) {
	parts := strings.SplitN(apiKey, "-", 2)
	if len(parts) != 2 || parts[1] == "" {
		return nil, fmt.Errorf("mailchimp: invalid API key format (expected <key>-<datacenter>)")
	}
	return &MailchimpHTTPClient{
		apiKey:     apiKey,
		listID:     listID,
		datacenter: parts[1],
		httpClient: &http.Client{},
	}, nil
}

// mailchimpMember is the request body for the Mailchimp add-or-update member call.
type mailchimpMember struct {
	EmailAddress string `json:"email_address"`
	Status       string `json:"status"`
}

// AddMember subscribes email to the configured Mailchimp audience list.
// Uses PUT /3.0/lists/{list_id}/members/{subscriber_hash} (upsert semantics)
// so the call is idempotent on the Mailchimp side.
//
// The subscriber hash is MD5(lowercase(email)) per Mailchimp API spec.
func (c *MailchimpHTTPClient) AddMember(ctx context.Context, email string) error {
	hash := mailchimpSubscriberHash(email)
	url := fmt.Sprintf("https://%s.api.mailchimp.com/3.0/lists/%s/members/%s",
		c.datacenter, c.listID, hash)

	body, err := json.Marshal(mailchimpMember{
		EmailAddress: email,
		Status:       "subscribed",
	})
	if err != nil {
		return fmt.Errorf("mailchimp: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailchimp: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("anystring", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailchimp: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailchimp: unexpected status %d", resp.StatusCode)
	}
	return nil
}

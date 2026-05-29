// export_test.go exposes internal Client fields for use in black-box tests
// (package scryfall_test). This file is compiled only during `go test`.
package scryfall

import "time"

// NewClientForTest returns a Client constructed by NewClient, exposed so
// package scryfall_test can inspect internal fields without breaking
// encapsulation in production code.
func NewClientForTest() *Client {
	return NewClient()
}

// BulkTimeout returns the Timeout of the bulk-download HTTP client. It is used
// by TestNewClient_BulkClientHasNoTimeout to assert that the bulk client has
// no transport-level timeout (Timeout == 0), catching any future regression.
func (c *Client) BulkTimeout() time.Duration {
	return c.bulkHTTPClient.Timeout
}

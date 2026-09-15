// Package hiveclient implements application/inspection.HiveVerifier
// against the real hive-service over HTTP.
package hiveclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
)

const requestTimeout = 5 * time.Second

// Client verifies hive ownership by forwarding the caller's own access
// token to hive-service's GET /api/v1/hives/{id}, and trusting
// hive-service's own ownership check: a 200 means whoever holds that
// token owns that hive (and, transitively, its apiary - hive-service's
// own check is itself transitive against apiary-service), a 404 means
// they don't. This service never queries hive or apiary ownership itself.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client that calls hive-service at baseURL (e.g.
// "http://hive-service:8080").
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: requestTimeout},
	}
}

// hiveDetailResponse decodes just the one field this client needs off
// hive-service's GET /api/v1/hives/{id} response - it carries many more
// (name, notes, images, ...) that this service has no use for.
type hiveDetailResponse struct {
	Writable bool `json:"writable"`
}

// Verify implements application/inspection.HiveVerifier.
func (c *Client) Verify(ctx context.Context, accessToken string, hiveID uuid.UUID) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/hives/%s", c.baseURL, hiveID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("hiveclient: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("hiveclient: call hive-service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var body hiveDetailResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return false, fmt.Errorf("hiveclient: decode response: %w", err)
		}
		return body.Writable, nil
	case http.StatusNotFound:
		return false, appinspection.ErrHiveNotFound
	default:
		// Anything else (401, 5xx, ...) is unexpected for a token this
		// service already verified itself: fail closed with a distinct,
		// observable error rather than silently treating it as "not
		// found", which would mask a real problem (e.g. hive-service
		// misconfigured or unreachable) as a client-facing 404.
		return false, fmt.Errorf("hiveclient: unexpected status %d from hive-service", resp.StatusCode)
	}
}

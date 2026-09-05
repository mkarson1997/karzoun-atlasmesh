package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type HTTPChecker struct { client *http.Client }
func NewHTTPChecker(timeout time.Duration) *HTTPChecker { if timeout <= 0 { timeout = 2*time.Second }; return &HTTPChecker{client:&http.Client{Timeout:timeout}} }
func (c *HTTPChecker) Check(ctx context.Context, target string) error {
	parsed, err := url.Parse(target); if err != nil { return fmt.Errorf("parse target: %w", err) }
	if parsed.Scheme != "http" && parsed.Scheme != "https" { return errors.New("health target must use http or https") }
	if parsed.Host == "" { return errors.New("health target requires a host") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil); if err != nil { return fmt.Errorf("build request: %w", err) }
	resp, err := c.client.Do(req); if err != nil { return err }; defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 { return fmt.Errorf("unhealthy status %d", resp.StatusCode) }
	return nil
}

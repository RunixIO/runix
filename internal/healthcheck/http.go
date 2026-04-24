package healthcheck

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Checker) checkHTTP(ctx context.Context) error {
	req, err := http.NewRequest("GET", c.config.URL, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

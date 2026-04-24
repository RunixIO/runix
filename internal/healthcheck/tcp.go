package healthcheck

import (
	"context"
	"net"
)

func (c *Checker) checkTCP(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.config.TCPEndpoint)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

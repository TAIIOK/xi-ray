package service

import (
	"context"
	"net"
	"strconv"
	"time"
)

func TCPLatencyMs(ctx context.Context, host string, port int) (int, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return int(time.Since(start).Milliseconds()), nil
}

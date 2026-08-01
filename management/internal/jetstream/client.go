package jetstream

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

type Client struct {
	conn *nats.Conn
	js   jsapi.JetStream
}

func Connect(cfg config.Config) (*Client, error) {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.Timeout(cfg.ConnectTimeout),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	}
	if cfg.NATSCreds != "" {
		opts = append(opts, nats.UserCredentials(cfg.NATSCreds))
	} else if cfg.NATSUser != "" {
		opts = append(opts, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword))
	}
	conn, err := nats.Connect(strings.Join(strings.Split(cfg.NATSURL, ","), ","), opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	js, err := jsapi.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create JetStream client: %w", err)
	}
	return &Client{conn: conn, js: js}, nil
}

func (c *Client) Ready(ctx context.Context) error {
	_, err := c.js.AccountInfo(ctx)
	return err
}

func (c *Client) AccountInfo(ctx context.Context) (*jsapi.AccountInfo, error) {
	return c.js.AccountInfo(ctx)
}

func (c *Client) ServerURL() string { return c.conn.ConnectedUrl() }

func (c *Client) Close() {
	if err := c.conn.Drain(); err != nil {
		c.conn.Close()
	}
}

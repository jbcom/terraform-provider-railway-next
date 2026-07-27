package client

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	genqlient "github.com/Khan/genqlient/graphql"
)

const DefaultEndpoint = "https://backboard.railway.com/graphql/v2"

type TokenType string

const (
	TokenTypeAccount   TokenType = "account"
	TokenTypeWorkspace TokenType = "workspace"
	TokenTypeProject   TokenType = "project"
)

type Config struct {
	Token      string
	TokenType  TokenType
	Endpoint   string
	Timeout    time.Duration
	Version    string
	HTTPDoer   HTTPDoer
	MaxRetries int
}

type Client struct {
	graphql genqlient.Client
}

func New(config Config) (*Client, error) {
	normalized, err := normalize(config)
	if err != nil {
		return nil, err
	}
	doer := newRailwayDoer(normalized)
	return &Client{graphql: genqlient.NewClient(normalized.Endpoint, doer)}, nil
}

func (c *Client) GraphQL() genqlient.Client {
	return c.graphql
}

func normalize(config Config) (Config, error) {
	config.Token = strings.TrimSpace(config.Token)
	if config.Token == "" {
		config.Token = strings.TrimSpace(os.Getenv("RAILWAY_API_TOKEN"))
	}
	if config.Token == "" {
		config.Token = strings.TrimSpace(os.Getenv("RAILWAY_TOKEN"))
	}
	if config.Token == "" {
		return Config{}, errors.New("Railway token is required; configure token or set RAILWAY_API_TOKEN or RAILWAY_TOKEN")
	}

	if config.TokenType == "" {
		// Account and workspace tokens share Bearer authentication. Project
		// tokens cannot be inferred from their opaque value and must be explicit.
		config.TokenType = TokenTypeAccount
	}
	switch config.TokenType {
	case TokenTypeAccount, TokenTypeWorkspace, TokenTypeProject:
	default:
		return Config{}, fmt.Errorf("unsupported Railway token type %q", config.TokenType)
	}

	config.Endpoint = strings.TrimSpace(config.Endpoint)
	if config.Endpoint == "" {
		config.Endpoint = strings.TrimSpace(os.Getenv("RAILWAY_GRAPHQL_ENDPOINT"))
	}
	if config.Endpoint == "" {
		config.Endpoint = DefaultEndpoint
	}
	parsed, err := url.Parse(config.Endpoint)
	if err != nil || parsed.Host == "" {
		return Config{}, fmt.Errorf("invalid Railway GraphQL endpoint")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopback(parsed.Hostname())) {
		return Config{}, errors.New("Railway GraphQL endpoint must use HTTPS (HTTP is allowed only for loopback tests)")
	}
	if parsed.User != nil {
		return Config{}, errors.New("Railway GraphQL endpoint must not contain user information")
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Timeout < time.Second {
		return Config{}, errors.New("Railway request timeout must be at least one second")
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.MaxRetries < 0 || config.MaxRetries > 5 {
		return Config{}, errors.New("Railway read retry count must be between zero and five")
	}
	if config.Version == "" {
		config.Version = "dev"
	}
	return config, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

package miniflux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
	"github.com/teran/mcp-miniflux/domain/requestid"

	"github.com/sirupsen/logrus"
	"resty.dev/v3"
)

// ErrorKind classifies upstream failures into the SPEC §6.5 taxonomy.
type ErrorKind int

const (
	// ErrorNotFound maps to an upstream 404.
	ErrorNotFound ErrorKind = iota
	// ErrorAuth maps to an upstream 401/403.
	ErrorAuth
	// ErrorTransient maps to upstream 429/5xx, network errors or timeouts.
	ErrorTransient
)

// APIError is the typed error returned for upstream failures (SPEC §6.5). The
// Message must never contain secrets (L05).
type APIError struct {
	Kind    ErrorKind
	Status  int
	Message string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// ErrorKind returns the §6.5 taxonomy classification of the failure so the
// application layer can map it to an MCP/JSON-RPC outcome without an
// application→infrastructure edge (SPEC §6.5).
func (e *APIError) ErrorKind() dmf.ErrorKind {
	return dmf.ErrorKind(e.Kind)
}

// StatusCode returns the upstream HTTP status code (0 for network/timeout
// errors).
func (e *APIError) StatusCode() int {
	return e.Status
}

// MetricsRecorder receives one observation per upstream call (O03).
type MetricsRecorder interface {
	ObserveUpstream(statusCode int, dur time.Duration, bytesIn, bytesOut int64)
}

// Client is a thin, strongly-typed wrapper around the Miniflux HTTP API
// (SPEC §4). It is stateless; every method maps 1:1 to a single upstream
// request (X01).
type Client struct {
	cfg    clientConfig
	client *resty.Client
}

type clientConfig struct {
	httpClient   *http.Client
	defaultToken string
	metrics      MetricsRecorder
	logger       *logrus.Logger
	timeout      time.Duration
}

// defaultTimeout bounds every upstream call unless overridden by WithTimeout or
// an injected *http.Client (X03/N26). resty inherits this from the http.Client.
const defaultTimeout = 30 * time.Second

// Option configures the Client via New.
type Option func(*clientConfig) error

// WithHTTPClient injects the underlying *http.Client (used to inject a custom
// transport in tests). An injected client carries its own Timeout, which takes
// precedence over the default and over WithTimeout (X03/N26).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) error {
		if hc == nil {
			return errors.New("miniflux: WithHTTPClient requires a non-nil *http.Client")
		}
		c.httpClient = hc
		return nil
	}
}

// WithTimeout sets the per-call timeout applied to the default http.Client
// (X03/N26). It is ignored when an *http.Client is injected via WithHTTPClient,
// since an injected client owns its own Timeout. A non-positive duration is an
// error.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) error {
		if d <= 0 {
			return errors.New("miniflux: WithTimeout requires a positive duration")
		}
		c.timeout = d
		return nil
	}
}

// WithDefaultToken sets the MINIFLUX_API_TOKEN fallback used when no inbound
// X-Auth-Token is present (SPEC §3.1).
func WithDefaultToken(token string) Option {
	return func(c *clientConfig) error {
		c.defaultToken = token
		return nil
	}
}

// WithMetrics wires an O03 upstream metrics recorder.
func WithMetrics(r MetricsRecorder) Option {
	return func(c *clientConfig) error {
		c.metrics = r
		return nil
	}
}

// WithLogger wires a logrus logger used to emit a per-request outbound access
// log (L09/L04GO). Each record carries the request_id threaded through ctx (and
// propagated to Miniflux as X-Request-ID) and never contains the auth token or
// other secrets (L05).
func WithLogger(l *logrus.Logger) Option {
	return func(c *clientConfig) error {
		c.logger = l
		return nil
	}
}

// New builds a Miniflux client rooted at baseURL. An empty baseURL is an error.
func New(baseURL string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("miniflux: base URL is required")
	}

	cfg := clientConfig{timeout: defaultTimeout}
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: cfg.timeout}
	}

	rc := resty.NewWithClient(cfg.httpClient).SetBaseURL(baseURL)
	return &Client{cfg: cfg, client: rc}, nil
}

// AsPort exposes c through the domain port interface (SPEC §6.1, C07GO). The
// composition root uses it to pass the Miniflux client into the application
// layer as the dmf.Client port rather than as the concrete infrastructure type,
// keeping the application→domain dependency edge clean.
func AsPort(c *Client) dmf.Client { return c }

// ResolveToken returns the inbound X-Auth-Token when non-empty, else the
// configured default token (SPEC §3.1).
func (c *Client) ResolveToken(inbound string) string {
	if inbound != "" {
		return inbound
	}
	return c.cfg.defaultToken
}

// WithToken carries the inbound X-Auth-Token through ctx so per-call methods
// honour the pass-through (SPEC §3.1). The context key is owned by the domain
// layer so the application layer can thread the same token into ctx without an
// application→infrastructure edge; this wrapper keeps the infrastructure API
// stable for existing callers.
func WithToken(ctx context.Context, token string) context.Context {
	return dmf.WithToken(ctx, token)
}

// TokenFromContext reads the inbound X-Auth-Token stored by WithToken,
// returning "" when absent. See the domain implementation.
func TokenFromContext(ctx context.Context) string {
	return dmf.TokenFromContext(ctx)
}

// newRequest builds a resty request carrying the resolved X-Auth-Token and,
// when present in ctx, the X-Request-ID correlation header (L09).
func (c *Client) newRequest(ctx context.Context) *resty.Request {
	r := c.client.R().
		SetContext(ctx).
		SetHeader("X-Auth-Token", c.ResolveToken(TokenFromContext(ctx)))
	if id := requestid.RequestIDFromContext(ctx); id != "" {
		r.SetHeader("X-Request-ID", id)
	}
	return r
}

// execute performs a single upstream call, records O03 metrics and classifies
// any failure into the §6.5 taxonomy. result (if non-nil) receives the JSON
// body parsed manually from the raw bytes; parsing is lenient because the
// Miniflux API does not always set a JSON Content-Type on responses and a
// response that fails to decode into the model must not abort an otherwise
// successful call.
func (c *Client) execute(ctx context.Context, r *resty.Request, method, url string, result any) (*resty.Response, error) {
	start := time.Now()
	resp, err := r.Execute(method, url)
	dur := time.Since(start)

	status := 0
	if resp != nil {
		status = resp.StatusCode()
	}
	c.logOutbound(ctx, method, url, status, dur)

	if err != nil {
		c.observe(0, dur, 0, 0)
		return resp, &APIError{Kind: ErrorTransient, Status: 0, Message: err.Error()}
	}

	bytesIn := resp.Size()
	bytesOut := int64(0)
	if raw := resp.RawResponse; raw != nil && raw.Request != nil {
		bytesOut = raw.Request.ContentLength
	}
	c.observe(resp.StatusCode(), dur, bytesIn, bytesOut)

	if apiErr := classifyStatus(resp); apiErr != nil {
		return resp, apiErr
	}

	if result != nil {
		_ = json.Unmarshal(resp.Bytes(), result)
	}
	return resp, nil
}

// observe notifies the metrics recorder when one is configured (O03).
func (c *Client) observe(statusCode int, dur time.Duration, bytesIn, bytesOut int64) {
	if c.cfg.metrics != nil {
		c.cfg.metrics.ObserveUpstream(statusCode, dur, bytesIn, bytesOut)
	}
}

// logOutbound emits a per-request outbound access log (L09/L04GO) carrying the
// request_id correlation id. The message and fields contain only method/path/
// status/duration — never the auth token or any secret (L05).
func (c *Client) logOutbound(ctx context.Context, method, path string, status int, dur time.Duration) {
	if c.cfg.logger == nil {
		return
	}
	c.cfg.logger.WithField("request_id", requestid.RequestIDFromContext(ctx)).WithFields(logrus.Fields{
		"method":   method,
		"path":     path,
		"status":   status,
		"duration": dur.Milliseconds(),
	}).Info("miniflux outbound request")
}

// classifyStatus maps an HTTP status to the §6.5 taxonomy. 2xx -> nil (no
// error). The returned message never includes the auth token (L05).
func classifyStatus(resp *resty.Response) error {
	if resp.IsStatusSuccess() {
		return nil
	}

	status := resp.StatusCode()
	msg := sanitizeMessage(resp.String())
	switch status {
	case http.StatusNotFound:
		return &APIError{Kind: ErrorNotFound, Status: status, Message: msg}
	case http.StatusUnauthorized, http.StatusForbidden:
		return &APIError{Kind: ErrorAuth, Status: status, Message: msg}
	default: // 429, 5xx and any other non-2xx
		return &APIError{Kind: ErrorTransient, Status: status, Message: msg}
	}
}

// sanitizeMessage bounds the length of an upstream error message and strips
// control characters so it is safe for logging.
func sanitizeMessage(s string) string {
	if len(s) > 512 {
		s = s[:512]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// --- Read methods (SPEC §4.1) ---

// ListFeeds returns all feed subscriptions, optionally scoped to a category.
func (c *Client) ListFeeds(ctx context.Context, categoryID *int, limit, offset int) ([]dmf.Feed, error) {
	r := c.newRequest(ctx)
	if categoryID != nil {
		r.SetQueryParam("category_id", strconv.Itoa(*categoryID))
	}
	if limit > 0 {
		r.SetQueryParam("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		r.SetQueryParam("offset", strconv.Itoa(offset))
	}

	var feeds []dmf.Feed
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/feeds", &feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

// GetFeed returns a single feed by id.
func (c *Client) GetFeed(ctx context.Context, id int) (*dmf.Feed, error) {
	r := c.newRequest(ctx).SetPathParam("feedID", strconv.Itoa(id))

	var feed dmf.Feed
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/feeds/{feedID}", &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// ListCategories returns categories with per-category entry counts.
func (c *Client) ListCategories(ctx context.Context) ([]dmf.Category, error) {
	r := c.newRequest(ctx).SetQueryParam("counts", "true")

	var cats []dmf.Category
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/categories", &cats); err != nil {
		return nil, err
	}
	return cats, nil
}

// applyEntryFilter encodes a non-empty EntryFilter onto the query string.
func applyEntryFilter(r *resty.Request, filter dmf.EntryFilter) *resty.Request {
	if filter.Status != "" {
		r.SetQueryParam("status", filter.Status)
	}
	if filter.Order != "" {
		r.SetQueryParam("order", filter.Order)
	}
	if filter.Direction != "" {
		r.SetQueryParam("direction", filter.Direction)
	}
	if filter.Limit > 0 {
		r.SetQueryParam("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		r.SetQueryParam("offset", strconv.Itoa(filter.Offset))
	}
	if filter.Search != "" {
		r.SetQueryParam("search", filter.Search)
	}
	if filter.Starred != nil {
		r.SetQueryParam("starred", strconv.FormatBool(*filter.Starred))
	}
	if filter.CategoryID != nil {
		r.SetQueryParam("category_id", strconv.Itoa(*filter.CategoryID))
	}
	if filter.Before != nil {
		r.SetQueryParam("before", filter.Before.Format(time.RFC3339))
	}
	if filter.After != nil {
		r.SetQueryParam("after", filter.After.Format(time.RFC3339))
	}
	return r
}

// ListEntries returns entries across the whole account with the given filters.
func (c *Client) ListEntries(ctx context.Context, filter dmf.EntryFilter) (dmf.FeedEntries, error) {
	r := applyEntryFilter(c.newRequest(ctx), filter)

	var out dmf.FeedEntries
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/entries", &out); err != nil {
		return dmf.FeedEntries{}, err
	}
	return out, nil
}

// GetEntry returns a single entry by id.
func (c *Client) GetEntry(ctx context.Context, id int) (*dmf.Entry, error) {
	r := c.newRequest(ctx).SetPathParam("entryID", strconv.Itoa(id))

	var entry dmf.Entry
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/entries/{entryID}", &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetFeedEntries returns the entries of a single feed with the given filters.
func (c *Client) GetFeedEntries(ctx context.Context, feedID int, filter dmf.EntryFilter) (dmf.FeedEntries, error) {
	r := applyEntryFilter(c.newRequest(ctx), filter).
		SetPathParam("feedID", strconv.Itoa(feedID))

	var out dmf.FeedEntries
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/feeds/{feedID}/entries", &out); err != nil {
		return dmf.FeedEntries{}, err
	}
	return out, nil
}

// GetCounters returns feed-level unread counts plus account totals.
func (c *Client) GetCounters(ctx context.Context) (*dmf.Counters, error) {
	r := c.newRequest(ctx)

	var out dmf.Counters
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/feeds/counters", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMe returns the authenticated user's profile.
func (c *Client) GetMe(ctx context.Context) (*dmf.Me, error) {
	r := c.newRequest(ctx)

	var out dmf.Me
	if _, err := c.execute(ctx, r, http.MethodGet, "/v1/me", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExportOPML returns the full subscription set as an OPML XML document.
func (c *Client) ExportOPML(ctx context.Context) (string, error) {
	r := c.newRequest(ctx)

	resp, err := c.execute(ctx, r, http.MethodGet, "/v1/export", nil)
	if err != nil {
		return "", err
	}
	return resp.String(), nil
}

// validateDiscoverURL enforces the S11/N21 allowlist for the open-world
// discover_subscriptions input: the URL must parse and be absolute with an
// http or https scheme. Anything else is rejected before any upstream call so
// SSRF exposure stays bounded to the trusted Miniflux instance (SPEC §7/S11).
func validateDiscoverURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("miniflux: discover: invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("miniflux: discover: URL must use http or https scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("miniflux: discover: URL must be absolute with a host")
	}
	return nil
}

// Discover probes a URL and returns candidate feeds.
func (c *Client) Discover(ctx context.Context, url string) ([]dmf.DiscoveryResult, error) {
	if err := validateDiscoverURL(url); err != nil {
		return nil, err
	}
	r := c.newRequest(ctx).SetBody(map[string]string{"url": url})

	var out []dmf.DiscoveryResult
	if _, err := c.execute(ctx, r, http.MethodPost, "/v1/discover", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- Write / update methods (SPEC §4.2) ---

// CreateFeed subscribes to a feed.
func (c *Client) CreateFeed(ctx context.Context, req dmf.CreateFeedRequest) (*dmf.Feed, error) {
	r := c.newRequest(ctx).SetBody(req)

	var feed dmf.Feed
	if _, err := c.execute(ctx, r, http.MethodPost, "/v1/feeds", &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// UpdateFeed updates a feed's metadata/credentials.
func (c *Client) UpdateFeed(ctx context.Context, id int, req dmf.UpdateFeedRequest) (*dmf.Feed, error) {
	r := c.newRequest(ctx).
		SetPathParam("feedID", strconv.Itoa(id)).
		SetBody(req)

	var feed dmf.Feed
	if _, err := c.execute(ctx, r, http.MethodPut, "/v1/feeds/{feedID}", &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// RefreshFeed forces a refresh of one feed.
func (c *Client) RefreshFeed(ctx context.Context, id int) error {
	r := c.newRequest(ctx).SetPathParam("feedID", strconv.Itoa(id))
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/feeds/{feedID}/refresh", nil)
	return err
}

// CreateCategory creates a category with the given title.
func (c *Client) CreateCategory(ctx context.Context, title string) (*dmf.Category, error) {
	r := c.newRequest(ctx).SetBody(map[string]string{"title": title})

	var cat dmf.Category
	if _, err := c.execute(ctx, r, http.MethodPost, "/v1/categories", &cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

// UpdateCategory renames a category.
func (c *Client) UpdateCategory(ctx context.Context, id int, title string) error {
	r := c.newRequest(ctx).
		SetPathParam("categoryID", strconv.Itoa(id)).
		SetBody(map[string]string{"title": title})
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/categories/{categoryID}", nil)
	return err
}

// RefreshCategory forces a refresh of all feeds in a category.
func (c *Client) RefreshCategory(ctx context.Context, id int) error {
	r := c.newRequest(ctx).SetPathParam("categoryID", strconv.Itoa(id))
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/categories/{categoryID}/refresh", nil)
	return err
}

// MarkFeedEntriesRead marks every entry in a feed as read.
func (c *Client) MarkFeedEntriesRead(ctx context.Context, feedID int) error {
	r := c.newRequest(ctx).SetPathParam("feedID", strconv.Itoa(feedID))
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/feeds/{feedID}/mark-all-as-read", nil)
	return err
}

// MarkCategoryEntriesRead marks every entry in a category as read.
func (c *Client) MarkCategoryEntriesRead(ctx context.Context, categoryID int) error {
	r := c.newRequest(ctx).SetPathParam("categoryID", strconv.Itoa(categoryID))
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/categories/{categoryID}/mark-all-as-read", nil)
	return err
}

// UpdateEntries bulk-applies a status and/or starred flag to a set of entries.
func (c *Client) UpdateEntries(ctx context.Context, req dmf.UpdateEntriesRequest) error {
	r := c.newRequest(ctx).SetBody(req)
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/entries", nil)
	return err
}

// ToggleEntryBookmark flips the starred state of one entry.
func (c *Client) ToggleEntryBookmark(ctx context.Context, entryID int) error {
	r := c.newRequest(ctx).SetPathParam("entryID", strconv.Itoa(entryID))
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/entries/{entryID}/bookmark", nil)
	return err
}

// UpdateEntry updates an entry's title/content/url.
func (c *Client) UpdateEntry(ctx context.Context, id int, req dmf.UpdateEntryRequest) (*dmf.Entry, error) {
	r := c.newRequest(ctx).
		SetPathParam("entryID", strconv.Itoa(id)).
		SetBody(req)

	var entry dmf.Entry
	if _, err := c.execute(ctx, r, http.MethodPut, "/v1/entries/{entryID}", &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ImportOPML imports subscriptions from an OPML document.
func (c *Client) ImportOPML(ctx context.Context, opml string) error {
	r := c.newRequest(ctx).SetBody(opml)
	_, err := c.execute(ctx, r, http.MethodPost, "/v1/import", nil)
	return err
}

// --- Delete / destructive methods (SPEC §4.3) ---

// DeleteFeed permanently removes a feed subscription.
func (c *Client) DeleteFeed(ctx context.Context, id int) error {
	r := c.newRequest(ctx).SetPathParam("feedID", strconv.Itoa(id))
	_, err := c.execute(ctx, r, http.MethodDelete, "/v1/feeds/{feedID}", nil)
	return err
}

// DeleteCategory permanently removes a category.
func (c *Client) DeleteCategory(ctx context.Context, id int) error {
	r := c.newRequest(ctx).SetPathParam("categoryID", strconv.Itoa(id))
	_, err := c.execute(ctx, r, http.MethodDelete, "/v1/categories/{categoryID}", nil)
	return err
}

// FlushHistory purges history, optionally only older than before.
func (c *Client) FlushHistory(ctx context.Context, before *time.Time) error {
	r := c.newRequest(ctx)
	if before != nil {
		r.SetQueryParam("before", before.Format(time.RFC3339))
	}
	_, err := c.execute(ctx, r, http.MethodPut, "/v1/flush-history", nil)
	return err
}

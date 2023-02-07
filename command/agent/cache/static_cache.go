package cache

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/command/agent/cache/cachememdb"
	"github.com/hashicorp/vault/sdk/helper/consts"
	"github.com/hashicorp/vault/sdk/helper/locksutil"
)

// StaticCache is an implementation of Proxier that handles
// the caching of responses without leases. It passes the
// incoming request to an underlying Proxier implementation.
type StaticCache struct {
	client  *api.Client
	proxier Proxier
	logger  hclog.Logger
	db      *cachememdb.CacheMemDB
	l       *sync.RWMutex

	// idLocks is used during cache lookup to ensure that identical requests made
	// in parallel won't trigger multiple renewal goroutines.
	idLocks []*locksutil.LockEntry

	// How long each item in the cache should be kept before it's considered stale.
	ttl time.Duration

	// Used to enable events system subscriptions so that cached items can be invalidated
	// early if their value changes in Vault.
	subscribeToUpdates bool
}

// LeaseCacheConfig is the configuration for initializing a new
// Lease.
type StaticCacheConfig struct {
	Client             *api.Client
	Proxier            Proxier
	Logger             hclog.Logger
	TTL                string
	SubscribeToUpdates bool
}

// NewLeaseCache creates a new instance of a LeaseCache.
func NewStaticCache(conf *StaticCacheConfig) (*StaticCache, error) {
	if conf == nil {
		return nil, errors.New("nil configuration provided")
	}

	if conf.Proxier == nil || conf.Logger == nil {
		return nil, fmt.Errorf("missing configuration required params: %v", conf)
	}

	if conf.Client == nil {
		return nil, fmt.Errorf("nil API client")
	}

	db, err := cachememdb.New()
	if err != nil {
		return nil, err
	}

	ttl, err := time.ParseDuration(conf.TTL)
	if err != nil {
		return nil, err
	}

	return &StaticCache{
		client:  conf.Client,
		proxier: conf.Proxier,
		logger:  conf.Logger,
		db:      db,
		l:       &sync.RWMutex{},
		idLocks: locksutil.CreateLocks(),

		ttl:                ttl,
		subscribeToUpdates: conf.SubscribeToUpdates,
	}, nil
}

// getCachedResponse checks the cache for a particular request based on its
// computed ID. It returns a non-nil *SendResponse  if an entry is found.
func (c *StaticCache) getCachedResponse(id string) (*SendResponse, error) {
	index, err := c.db.Get(cachememdb.IndexNameID, id)
	if err != nil {
		return nil, err
	}

	if index == nil {
		return nil, nil
	}

	if index.LastRenewed.Add(c.ttl).After(time.Now()) {
		// Stale, evict and return a cache miss
		if err := c.Evict(index); err != nil {
			return nil, err
		}

		return nil, nil
	}

	// Cached request is found, deserialize the response
	reader := bufio.NewReader(bytes.NewReader(index.Response))
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		c.logger.Error("failed to deserialize response", "error", err)
		return nil, err
	}

	sendResp, err := NewSendResponse(&api.Response{Response: resp}, index.Response)
	if err != nil {
		c.logger.Error("failed to create new send response", "error", err)
		return nil, err
	}
	sendResp.CacheMeta.Hit = true

	respTime, err := http.ParseTime(resp.Header.Get("Date"))
	if err != nil {
		c.logger.Error("failed to parse cached response date", "error", err)
		return nil, err
	}
	sendResp.CacheMeta.Age = time.Now().Sub(respTime)

	return sendResp, nil
}

// Send performs a cache lookup on the incoming request. If it's a cache hit,
// it will return the cached response, otherwise it will delegate to the
// underlying Proxier and cache the received response.
func (c *StaticCache) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// Compute the index ID
	id, err := computeIndexID(req)
	if err != nil {
		c.logger.Error("failed to compute cache key", "error", err)
		return nil, err
	}

	idLock := locksutil.LockForKey(c.idLocks, id)
	idLock.Lock()
	defer idLock.Unlock()

	// Check if the response for this request is already in the cache
	resp, err := c.getCachedResponse(id)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		c.logger.Debug("returning cached response", "path", req.Request.URL.Path)
		return resp, nil
	}

	c.logger.Debug("forwarding request from cache", "method", req.Request.Method, "path", req.Request.URL.Path)

	// Pass the request down and get a response
	resp, err = c.proxier.Send(ctx, req)
	if err != nil {
		return resp, err
	}

	// If this is a non-2xx or if the returned response does not contain JSON payload,
	// we skip caching
	if resp.Response.StatusCode >= 300 || resp.Response.Header.Get("Content-Type") != "application/json" {
		return resp, err
	}

	// Get the namespace from the request header
	namespace := req.Request.Header.Get(consts.NamespaceHeaderName)
	// We need to populate an empty value since go-memdb will skip over indexes
	// that contain empty values.
	if namespace == "" {
		namespace = "root/"
	}

	// Build the index to cache based on the response received
	index := &cachememdb.Index{
		ID:          id,
		Namespace:   namespace,
		RequestPath: req.Request.URL.Path,
		LastRenewed: time.Now().UTC(),
	}

	secret, err := api.ParseSecret(bytes.NewReader(resp.ResponseBody))
	if err != nil {
		c.logger.Error("failed to parse response as secret", "error", err)
		return nil, err
	}

	// Fast path for responses with no secrets
	if secret == nil {
		c.logger.Debug("pass-through response; no secret in response", "method", req.Request.Method, "path", req.Request.URL.Path)
		return resp, nil
	}

	// Serialize the response to store it in the cached index
	var respBytes bytes.Buffer
	err = resp.Response.Write(&respBytes)
	if err != nil {
		c.logger.Error("failed to serialize response", "error", err)
		return nil, err
	}

	// Reset the response body for upper layers to read
	if resp.Response.Body != nil {
		resp.Response.Body.Close()
	}
	resp.Response.Body = ioutil.NopCloser(bytes.NewReader(resp.ResponseBody))

	// Set the index's Response
	index.Response = respBytes.Bytes()

	// Add extra information necessary for restoring from persisted cache
	// TODO: Support persistent cache?
	index.RequestMethod = req.Request.Method
	index.RequestToken = req.Token
	index.RequestHeader = req.Request.Header

	// Store the index in the cache
	c.logger.Debug("storing response into the cache", "method", req.Request.Method, "path", req.Request.URL.Path)
	err = c.Set(ctx, index)
	if err != nil {
		c.logger.Error("failed to cache the proxied response", "error", err)
		return nil, err
	}

	if c.subscribeToUpdates {
		reqClone := *req
		reqClone.Request = reqClone.Request.Clone(context.Background())
		c.subscribe(&reqClone)
	}

	return resp, nil
}

// TODO: Only a rough layout of what subscribing will look like.
func (c *StaticCache) subscribe(req *SendRequest) error {
	secretPath := req.Request.URL.Path
	req.Request.URL.Path = "/v1/sys/events/subscribe"
	if req.Request.URL.Scheme == "http" {
		req.Request.URL.Scheme = "ws"
	} else {
		// TODO: support wss
		req.Request.URL.Scheme = "ws"
	}
	conn, _, err := websocket.DefaultDialer.Dial(req.Request.URL.String(), req.Request.Header)
	if err != nil {
		c.logger.Error("failed to create subscription", "error", err, "url", req.Request.URL, "headers", req.Request.Header, "token", req.Token)
		return err
	}

	c.logger.Info("subscribed, waiting for events")
	go func() {
		defer conn.Close()
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				c.logger.Error("Cancelling subscription", "secret path", secretPath, "error", err)
				return
			}
			c.logger.Info("received event", "messageType", messageType, "message", string(message))
			// TODO: Invalidate the cache item.
		}
	}()

	return nil
}

// TODO: Handle cache clear API calls in the static cache as well as lease cache.

// Set stores a single cached response.
func (c *StaticCache) Set(ctx context.Context, index *cachememdb.Index) error {
	if err := c.db.Set(index); err != nil {
		return err
	}

	return nil
}

// Evict clears a single cached response.
func (c *StaticCache) Evict(index *cachememdb.Index) error {
	if err := c.db.Evict(cachememdb.IndexNameID, index.ID); err != nil {
		return err
	}

	return nil
}

// Flush resets the cache.
func (c *StaticCache) Flush() error {
	if err := c.db.Flush(); err != nil {
		return err
	}

	return nil
}

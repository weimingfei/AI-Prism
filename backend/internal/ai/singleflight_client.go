package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ai-prism/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

type singleFlightStageKey struct{}

func WithSingleFlightStage(ctx context.Context, stage string) context.Context {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		stage = "ai-chat"
	}
	return context.WithValue(ctx, singleFlightStageKey{}, stage)
}

func NewClient(cfg config.Config) Client {
	base := NewHTTPClient(cfg.AI)
	if !cfg.SingleFlight.Enabled {
		return base
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		if strings.EqualFold(cfg.SingleFlight.Mode, "distributed") {
			return &singleFlightClient{next: base, cfg: normalizeSingleFlightConfig(cfg.SingleFlight), nodeID: nodeID()}
		}
		return newLocalSingleFlightClient(base, cfg.SingleFlight)
	}
	return &singleFlightClient{
		next:   base,
		redis:  redisClient,
		cfg:    normalizeSingleFlightConfig(cfg.SingleFlight),
		nodeID: nodeID(),
		local:  newL1Cache(cfg.SingleFlight),
	}
}

type singleFlightClient struct {
	next   Client
	redis  *redis.Client
	cfg    config.SingleFlightConfig
	nodeID string
	local  *l1Cache

	mu      sync.Mutex
	running map[string]*localCall
}

type localCall struct {
	done chan struct{}
	resp ChatResponse
	err  error
}

type flightStoredResult struct {
	Response ChatResponse `json:"response"`
}

const (
	// 这组 Lua 脚本把“抢占执行权、心跳保活、结果写入、失败归档”放到 Redis 原子操作里。
	// AI 调用耗时长且费用高，不能只靠本地锁；多实例部署时必须让同一请求只落到一个 Owner。
	acquireScript = `
local status = redis.call('HGET', KEYS[1], 'status')
if not status then
  local token = redis.call('INCR', KEYS[2])
  redis.call('HSET', KEYS[1], 'status', 'PENDING', 'stage', ARGV[1], 'ownerId', ARGV[2], 'ownerToken', token, 'requestKey', ARGV[3], 'createdAt', ARGV[4], 'updatedAt', ARGV[4], 'heartbeatAt', ARGV[4], 'expireAt', ARGV[5], 'retryable', '0')
  redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[6]))
  return 'OWNER_NEW|' .. token
end
if status == 'SUCCEEDED' then return 'REPLAY_SUCCESS|' .. status end
if status == 'FAILED' then
  local retryable = redis.call('HGET', KEYS[1], 'retryable') or '0'
  if retryable == '1' then
    local token = redis.call('INCR', KEYS[2])
    redis.call('HSET', KEYS[1], 'status', 'PENDING', 'stage', ARGV[1], 'ownerId', ARGV[2], 'ownerToken', token, 'requestKey', ARGV[3], 'updatedAt', ARGV[4], 'heartbeatAt', ARGV[4], 'expireAt', ARGV[5], 'errorType', '', 'errorCode', '', 'retryable', '0')
    redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[6]))
    return 'OWNER_TAKEOVER|' .. token
  end
  local errorType = redis.call('HGET', KEYS[1], 'errorType') or ''
  local errorCode = redis.call('HGET', KEYS[1], 'errorCode') or ''
  return 'REPLAY_FAILURE|0|' .. errorType .. '|' .. errorCode
end
local heartbeatAt = tonumber(redis.call('HGET', KEYS[1], 'heartbeatAt') or '0')
local takeoverDetectMillis = tonumber(ARGV[7])
if heartbeatAt > 0 and (tonumber(ARGV[4]) - heartbeatAt) <= takeoverDetectMillis then
  redis.call('HINCRBY', KEYS[1], 'followerCount', 1)
  return 'FOLLOWER_WAIT|' .. (redis.call('HGET', KEYS[1], 'ownerToken') or '')
end
local token = redis.call('INCR', KEYS[2])
redis.call('HSET', KEYS[1], 'status', 'PENDING', 'stage', ARGV[1], 'ownerId', ARGV[2], 'ownerToken', token, 'requestKey', ARGV[3], 'updatedAt', ARGV[4], 'heartbeatAt', ARGV[4], 'expireAt', ARGV[5], 'errorType', '', 'errorCode', '', 'retryable', '0')
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[6]))
return 'OWNER_TAKEOVER|' .. token`
	markRunningScript = `
if redis.call('HGET', KEYS[1], 'ownerId') ~= ARGV[1] then return 0 end
if redis.call('HGET', KEYS[1], 'ownerToken') ~= ARGV[2] then return 0 end
local status = redis.call('HGET', KEYS[1], 'status')
if status ~= 'PENDING' and status ~= 'RUNNING' then return 0 end
redis.call('HSET', KEYS[1], 'status', 'RUNNING', 'updatedAt', ARGV[3], 'heartbeatAt', ARGV[3], 'expireAt', ARGV[4])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[5]))
return 1`
	heartbeatScript = `
if redis.call('HGET', KEYS[1], 'ownerId') ~= ARGV[1] then return 0 end
if redis.call('HGET', KEYS[1], 'ownerToken') ~= ARGV[2] then return 0 end
if redis.call('HGET', KEYS[1], 'status') ~= 'RUNNING' then return 0 end
redis.call('HSET', KEYS[1], 'updatedAt', ARGV[3], 'heartbeatAt', ARGV[3], 'expireAt', ARGV[4], 'lastHeartbeatNode', ARGV[1])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[5]))
return 1`
	storeResultScript = `
if redis.call('HGET', KEYS[1], 'ownerId') ~= ARGV[1] then return 0 end
if redis.call('HGET', KEYS[1], 'ownerToken') ~= ARGV[2] then return 0 end
local status = redis.call('HGET', KEYS[1], 'status')
if status ~= 'PENDING' and status ~= 'RUNNING' then return 0 end
redis.call('HSET', KEYS[2], 'payload', ARGV[3], 'codec', ARGV[4], 'compressed', ARGV[5], 'rawSize', ARGV[6], 'storedSize', ARGV[7], 'checksum', ARGV[8], 'contentType', ARGV[9], 'finishedAt', ARGV[10], 'ownerToken', ARGV[2])
redis.call('PEXPIRE', KEYS[2], tonumber(ARGV[11]))
return 1`
	finishSuccessScript = `
if redis.call('HGET', KEYS[1], 'ownerId') ~= ARGV[1] then return 0 end
if redis.call('HGET', KEYS[1], 'ownerToken') ~= ARGV[2] then return 0 end
local status = redis.call('HGET', KEYS[1], 'status')
if status ~= 'PENDING' and status ~= 'RUNNING' then return 0 end
redis.call('HSET', KEYS[1], 'status', 'SUCCEEDED', 'updatedAt', ARGV[3], 'expireAt', ARGV[4], 'resultRef', KEYS[2], 'errorType', '', 'errorCode', '', 'retryable', '0')
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[5]))
return 1`
	finishFailureScript = `
if redis.call('HGET', KEYS[1], 'ownerId') ~= ARGV[1] then return 0 end
if redis.call('HGET', KEYS[1], 'ownerToken') ~= ARGV[2] then return 0 end
local status = redis.call('HGET', KEYS[1], 'status')
if status ~= 'PENDING' and status ~= 'RUNNING' then return 0 end
redis.call('HSET', KEYS[1], 'status', 'FAILED', 'updatedAt', ARGV[3], 'expireAt', ARGV[4], 'errorType', ARGV[5], 'errorCode', ARGV[6], 'retryable', ARGV[7], 'lastEvent', 'owner_failed')
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[8]))
return 1`
)

func newLocalSingleFlightClient(next Client, cfg config.SingleFlightConfig) Client {
	return &singleFlightClient{
		next:    next,
		cfg:     normalizeSingleFlightConfig(cfg),
		nodeID:  nodeID(),
		local:   newL1Cache(cfg),
		running: map[string]*localCall{},
	}
}

func (c *singleFlightClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	stage := stageFromContext(ctx)
	key := requestKey(stage, request)
	if override, ok := providerOverrideFromContext(ctx); ok {
		key = key + "|" + overrideKey(override)
	}
	if c.local != nil {
		if resp, ok := c.local.get(stage, key); ok {
			return resp, nil
		}
	}
	if c.redis == nil || !c.cfg.DistributedEnabled || strings.EqualFold(c.cfg.Mode, "local") {
		return c.localExecute(ctx, stage, key, request)
	}
	resp, err := c.distributedExecute(ctx, stage, key, request)
	if err != nil && strings.EqualFold(c.cfg.Mode, "hybrid") {
		return c.localExecute(ctx, stage, key, request)
	}
	return resp, err
}

func (c *singleFlightClient) StreamChat(ctx context.Context, request ChatRequest) (<-chan ChatResponse, <-chan error) {
	responses := make(chan ChatResponse, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(responses)
		defer close(errs)
		resp, err := c.Chat(ctx, request)
		if err != nil {
			errs <- err
			return
		}
		responses <- resp
	}()
	return responses, errs
}

func (c *singleFlightClient) distributedExecute(ctx context.Context, stage string, key string, request ChatRequest) (ChatResponse, error) {
	deadline := time.Now().Add(time.Duration(c.cfg.FollowerMaxWaitMillis) * time.Millisecond)
	for attempts := 0; attempts < 3; attempts++ {
		acquired, token, err := c.acquireOrJoin(ctx, stage, key)
		if err != nil {
			return ChatResponse{}, err
		}
		switch acquired {
		case "OWNER_NEW", "OWNER_TAKEOVER":
			return c.ownerExecute(ctx, stage, key, token, request)
		case "REPLAY_SUCCESS":
			if resp, ok := c.readReplay(ctx, stage, key); ok {
				return resp, nil
			}
		case "REPLAY_FAILURE":
			return ChatResponse{}, errors.New("distributed single-flight replay failure")
		case "FOLLOWER_WAIT":
			if resp, ok := c.followerWait(ctx, stage, key, deadline); ok {
				return resp, nil
			}
		}
	}
	return ChatResponse{}, errors.New("distributed single-flight max attempts exceeded")
}

func (c *singleFlightClient) ownerExecute(ctx context.Context, stage string, key string, token string, request ChatRequest) (ChatResponse, error) {
	if !c.markRunning(ctx, key, token) {
		if resp, ok := c.followerWait(ctx, stage, key, time.Now().Add(time.Duration(c.cfg.FollowerMaxWaitMillis)*time.Millisecond)); ok {
			return resp, nil
		}
		return ChatResponse{}, errors.New("distributed single-flight owner lost before running")
	}
	stopHeartbeat := c.startHeartbeat(ctx, key, token)
	defer stopHeartbeat()

	// Owner 真实调用模型；Follower 只等待 Redis 中的结果回放，避免并发重复请求模型。
	resp, err := c.next.Chat(ctx, request)
	if err != nil {
		c.finishFailure(context.Background(), key, token, err)
		return ChatResponse{}, err
	}
	if err := c.storeSuccess(context.Background(), stage, key, token, resp); err != nil {
		return ChatResponse{}, err
	}
	return resp, nil
}

func (c *singleFlightClient) localExecute(ctx context.Context, stage string, key string, request ChatRequest) (ChatResponse, error) {
	if c.running == nil {
		c.running = map[string]*localCall{}
	}
	c.mu.Lock()
	if call, ok := c.running[key]; ok {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ChatResponse{}, ctx.Err()
		case <-call.done:
			return call.resp, call.err
		}
	}
	call := &localCall{done: make(chan struct{})}
	c.running[key] = call
	c.mu.Unlock()

	call.resp, call.err = c.next.Chat(ctx, request)
	if call.err == nil && c.local != nil {
		c.local.put(stage, key, call.resp)
	}
	close(call.done)

	c.mu.Lock()
	delete(c.running, key)
	c.mu.Unlock()
	return call.resp, call.err
}

func (c *singleFlightClient) acquireOrJoin(ctx context.Context, stage string, key string) (string, string, error) {
	now := time.Now().UnixMilli()
	expireAt := now + int64(c.cfg.RunningTTLMillis)
	raw, err := c.redis.Eval(ctx, acquireScript, []string{metaKey(key), "ai:flight:owner-seq"},
		stage, c.nodeID, key, strconv.FormatInt(now, 10), strconv.FormatInt(expireAt, 10),
		strconv.Itoa(c.cfg.RunningTTLMillis), strconv.Itoa(c.cfg.TakeoverDetectMillis),
	).Text()
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(raw, "|")
	action := parts[0]
	token := ""
	if len(parts) > 1 {
		token = parts[1]
	}
	return action, token, nil
}

func (c *singleFlightClient) markRunning(ctx context.Context, key string, token string) bool {
	now := time.Now().UnixMilli()
	result, err := c.redis.Eval(ctx, markRunningScript, []string{metaKey(key)},
		c.nodeID, token, strconv.FormatInt(now, 10), strconv.FormatInt(now+int64(c.cfg.RunningTTLMillis), 10), strconv.Itoa(c.cfg.RunningTTLMillis),
	).Int()
	return err == nil && result == 1
}

func (c *singleFlightClient) startHeartbeat(ctx context.Context, key string, token string) func() {
	interval := time.Duration(c.cfg.HeartbeatIntervalMillis) * time.Millisecond
	if interval <= 0 {
		interval = 3 * time.Second
	}
	heartbeatCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now().UnixMilli()
				_, _ = c.redis.Eval(context.Background(), heartbeatScript, []string{metaKey(key)},
					c.nodeID, token, strconv.FormatInt(now, 10), strconv.FormatInt(now+int64(c.cfg.RunningTTLMillis), 10), strconv.Itoa(c.cfg.RunningTTLMillis),
				).Int()
			}
		}
	}()
	return cancel
}

func (c *singleFlightClient) storeSuccess(ctx context.Context, stage string, key string, token string, resp ChatResponse) error {
	storedResult, err := serializeFlightResult(resp, c.cfg)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	stored, err := c.redis.Eval(ctx, storeResultScript, []string{metaKey(key), resultKey(key)},
		c.nodeID,
		token,
		storedResult.Payload,
		storedResult.Codec,
		boolFlag(storedResult.Compressed),
		strconv.Itoa(storedResult.RawSize),
		strconv.Itoa(storedResult.StoredSize),
		storedResult.Checksum,
		storedResult.ContentType,
		strconv.FormatInt(now, 10),
		strconv.Itoa(c.cfg.ResultTTLMillis),
	).Int()
	if err != nil || stored != 1 {
		return errors.New("failed to store distributed single-flight result")
	}
	finished, err := c.redis.Eval(ctx, finishSuccessScript, []string{metaKey(key), resultKey(key)},
		c.nodeID, token, strconv.FormatInt(now, 10), strconv.FormatInt(now+int64(c.cfg.ResultTTLMillis), 10), strconv.Itoa(c.cfg.ResultTTLMillis),
	).Int()
	if err != nil || finished != 1 {
		return errors.New("failed to finish distributed single-flight success")
	}
	if c.local != nil {
		c.local.put(stage, key, resp)
	}
	return nil
}

func (c *singleFlightClient) finishFailure(ctx context.Context, key string, token string, cause error) {
	now := time.Now().UnixMilli()
	errorType, retryable := classifyFlightError(cause)
	_, _ = c.redis.Eval(ctx, finishFailureScript, []string{metaKey(key)},
		c.nodeID, token, strconv.FormatInt(now, 10), strconv.FormatInt(now+int64(c.cfg.FailedResultTTLMillis), 10),
		errorType, "AI_CALL_FAILED", boolFlag(retryable), strconv.Itoa(c.cfg.FailedResultTTLMillis),
	).Int()
}

func (c *singleFlightClient) followerWait(ctx context.Context, stage string, key string, deadline time.Time) (ChatResponse, bool) {
	interval := time.Duration(c.cfg.PollFallbackIntervalMillis) * time.Millisecond
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		if resp, ok := c.readReplay(ctx, stage, key); ok {
			return resp, true
		}
		select {
		case <-ctx.Done():
			return ChatResponse{}, false
		case <-ticker.C:
		}
	}
	return ChatResponse{}, false
}

func (c *singleFlightClient) readReplay(ctx context.Context, stage string, key string) (ChatResponse, bool) {
	if c.local != nil {
		if resp, ok := c.local.get(stage, key); ok {
			return resp, true
		}
	}
	status, err := c.redis.HGet(ctx, metaKey(key), "status").Result()
	if err != nil || status != "SUCCEEDED" {
		return ChatResponse{}, false
	}
	fields, err := c.redis.HGetAll(ctx, resultKey(key)).Result()
	if err != nil || len(fields) == 0 {
		return ChatResponse{}, false
	}
	resp, err := deserializeFlightResult(fields)
	if err != nil {
		return ChatResponse{}, false
	}
	if c.local != nil {
		c.local.put(stage, key, resp)
	}
	return resp, true
}

func stageFromContext(ctx context.Context) string {
	if ctx != nil {
		if stage, ok := ctx.Value(singleFlightStageKey{}).(string); ok && strings.TrimSpace(stage) != "" {
			return strings.TrimSpace(stage)
		}
	}
	return "ai-chat"
}

func requestKey(stage string, request ChatRequest) string {
	data, _ := json.Marshal(request)
	sum := sha256.Sum256(data)
	return stage + "|" + hex.EncodeToString(sum[:])
}

func overrideKey(cfg config.AIConfig) string {
	data, _ := json.Marshal(cfg)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func metaKey(key string) string {
	return "ai:flight:meta:" + key
}

func resultKey(key string) string {
	return "ai:flight:result:" + key
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nodeID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	if ips, err := net.InterfaceAddrs(); err == nil && len(ips) > 0 {
		return fmt.Sprintf("%s@%d@%s", host, os.Getpid(), ips[0].String())
	}
	return fmt.Sprintf("%s@%d", host, os.Getpid())
}

func normalizeSingleFlightConfig(cfg config.SingleFlightConfig) config.SingleFlightConfig {
	if cfg.Mode == "" {
		cfg.Mode = "hybrid"
	}
	if cfg.RunningTTLMillis <= 0 {
		cfg.RunningTTLMillis = 15000
	}
	if cfg.TakeoverDetectMillis <= 0 {
		cfg.TakeoverDetectMillis = 10000
	}
	if cfg.ResultTTLMillis <= 0 {
		cfg.ResultTTLMillis = 600000
	}
	if cfg.FailedResultTTLMillis <= 0 {
		cfg.FailedResultTTLMillis = 60000
	}
	if cfg.FollowerMaxWaitMillis <= 0 {
		cfg.FollowerMaxWaitMillis = 20000
	}
	if cfg.PollFallbackIntervalMillis <= 0 {
		cfg.PollFallbackIntervalMillis = 2000
	}
	if cfg.HeartbeatIntervalMillis <= 0 {
		cfg.HeartbeatIntervalMillis = 3000
	}
	if cfg.L1CacheMaxSize <= 0 {
		cfg.L1CacheMaxSize = 1000
	}
	if cfg.L1CacheTTLMillis <= 0 {
		cfg.L1CacheTTLMillis = 30000
	}
	cfg.CompressionCodec = normalizeFlightCompressionCodec(cfg.CompressionCodec)
	if cfg.CompressionThresholdBytes <= 0 {
		cfg.CompressionThresholdBytes = 4096
	}
	return cfg
}

func classifyFlightError(err error) (string, bool) {
	if err == nil {
		return "NONE", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "TIMEOUT", true
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELLED", true
	}
	return "PROVIDER", true
}

func boolFlag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

type l1Cache struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxSize int
	items   map[string]l1Entry
}

type l1Entry struct {
	resp      ChatResponse
	expiresAt time.Time
}

func newL1Cache(cfg config.SingleFlightConfig) *l1Cache {
	if !cfg.L1CacheEnabled {
		return nil
	}
	cfg = normalizeSingleFlightConfig(cfg)
	return &l1Cache{
		ttl:     time.Duration(cfg.L1CacheTTLMillis) * time.Millisecond,
		maxSize: cfg.L1CacheMaxSize,
		items:   map[string]l1Entry{},
	}
}

func (c *l1Cache) get(stage string, key string) (ChatResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[stage+"|"+key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(c.items, stage+"|"+key)
		return ChatResponse{}, false
	}
	return entry.resp, true
}

func (c *l1Cache) put(stage string, key string, resp ChatResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= c.maxSize {
		for itemKey := range c.items {
			delete(c.items, itemKey)
			break
		}
	}
	c.items[stage+"|"+key] = l1Entry{resp: resp, expiresAt: time.Now().Add(c.ttl)}
}

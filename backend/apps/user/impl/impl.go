package impl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ai-prism/backend/apps/user"
	"github.com/ai-prism/backend/internal/bloom"
	"github.com/ai-prism/backend/internal/config"
	"github.com/ai-prism/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/infraboard/mcube/v2/ioc"
	"github.com/redis/go-redis/v9"
)

func init() {
	ioc.Controller().Registry(&ServiceImpl{
		users:  map[string]*storedUser{},
		tokens: map[string]string{},
	})
}

var (
	ErrInvalidCredential = errors.New("invalid username or password")
	ErrUserExists        = errors.New("username already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrUnauthorized      = errors.New("unauthorized")
)

type storedUser struct {
	User     *user.User `json:"user"`
	Password string     `json:"password"`
}

type ServiceImpl struct {
	ioc.ObjectImpl

	mu     sync.RWMutex
	redis  *redis.Client
	bloom  *bloom.RedisBloomFilter
	nextID int64
	users  map[string]*storedUser
	tokens map[string]string
}

func (s *ServiceImpl) Name() string {
	return user.AppName
}

func (s *ServiceImpl) Init() error {
	cfg := config.Load()
	s.redis = storage.OpenRedis(cfg.Redis)
	if err := s.redis.Ping(context.Background()).Err(); err != nil {
		s.redis = nil
	}
	if s.redis != nil && cfg.Bloom.UserRegisterEnabled {
		filter, err := bloom.NewRedisBloomFilter(s.redis, bloom.Config{
			Key:                cfg.Bloom.UserRegisterKey,
			ExpectedInsertions: cfg.Bloom.UserRegisterExpectedItems,
			FalsePositiveRate:  cfg.Bloom.UserRegisterFalsePositive,
		})
		if err == nil && filter.TryInit(context.Background()) == nil {
			s.bloom = filter
			if cfg.Bloom.UserRegisterBootstrapScan {
				s.bootstrapUserRegisterBloom(context.Background())
			}
		}
	}
	return nil
}

func (s *ServiceImpl) Register(ctx context.Context, req user.RegisterRequest) (*user.User, error) {
	username := normalizeUsername(req.Username)
	if username == "" || strings.TrimSpace(req.Password) == "" {
		return nil, ErrInvalidCredential
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.userProbablyExists(ctx, username) {
		return nil, ErrUserExists
	}
	unlock := s.acquireRegisterLock(ctx, username)
	if unlock == nil {
		return nil, ErrUserExists
	}
	defer unlock()

	if _, ok := s.users[username]; ok {
		return nil, ErrUserExists
	}
	if s.redisUserExists(ctx, username) {
		s.addUserToBloom(ctx, username)
		return nil, ErrUserExists
	}

	s.nextID = maxInt64(s.nextID+1, time.Now().UnixNano())
	now := time.Now()
	u := &user.User{
		ID:         s.nextID,
		Username:   username,
		RealName:   strings.TrimSpace(req.RealName),
		Phone:      strings.TrimSpace(req.Phone),
		Mail:       strings.TrimSpace(req.Mail),
		Avatar:     "",
		CreateTime: now,
		UpdateTime: now,
		DelFlag:    0,
	}
	s.users[username] = &storedUser{
		User:     u,
		Password: req.Password,
	}
	s.saveUser(ctx, username, s.users[username])
	s.addUserToBloom(ctx, username)
	return cloneUser(u), nil
}

func (s *ServiceImpl) Login(ctx context.Context, req user.LoginRequest) (*user.AuthPayload, error) {
	username := normalizeUsername(req.Username)

	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.getStoredUser(ctx, username)
	if !ok || stored.Password != req.Password {
		return nil, ErrInvalidCredential
	}

	token := fmt.Sprintf("dev-%s", uuid.NewString())
	s.tokens[token] = username
	s.saveToken(ctx, token, username)
	return authPayload(token, stored.User), nil
}

func (s *ServiceImpl) CheckLogin(ctx context.Context, req user.CheckLoginRequest) (*user.AuthPayload, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, ErrUnauthorized
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	username, ok := s.getToken(ctx, token)
	if !ok {
		return nil, ErrUnauthorized
	}
	stored, ok := s.getStoredUser(ctx, username)
	if !ok {
		return nil, ErrUnauthorized
	}
	return authPayload(token, stored.User), nil
}

func (s *ServiceImpl) Logout(ctx context.Context, req user.LogoutRequest) error {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
	s.deleteToken(ctx, token)
	return nil
}

func (s *ServiceImpl) GetUser(ctx context.Context, req user.GetUserRequest) (*user.User, error) {
	username := normalizeUsername(req.Username)

	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.getStoredUser(ctx, username)
	if !ok {
		return nil, ErrUserNotFound
	}
	return cloneUser(stored.User), nil
}

func (s *ServiceImpl) HasUsername(ctx context.Context, req user.HasUsernameRequest) (bool, error) {
	username := normalizeUsername(req.Username)

	s.mu.Lock()
	defer s.mu.Unlock()
	if username == "" {
		return false, nil
	}
	if s.bloom != nil {
		exists, err := s.bloom.Contains(ctx, username)
		if err == nil {
			return !exists, nil
		}
	}
	_, ok := s.getStoredUser(ctx, username)
	return !ok, nil
}

func (s *ServiceImpl) userProbablyExists(ctx context.Context, username string) bool {
	if username == "" {
		return false
	}
	if _, ok := s.users[username]; ok {
		return true
	}
	if s.bloom != nil {
		exists, err := s.bloom.Contains(ctx, username)
		if err == nil {
			return exists
		}
	}
	return s.redisUserExists(ctx, username)
}

func (s *ServiceImpl) getStoredUser(ctx context.Context, username string) (*storedUser, bool) {
	if stored, ok := s.users[username]; ok {
		return stored, true
	}
	if s.redis == nil {
		return nil, false
	}
	data, err := s.redis.Get(ctx, userKey(username)).Bytes()
	if err != nil {
		return nil, false
	}
	var stored storedUser
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, false
	}
	s.users[username] = &stored
	return &stored, true
}

func (s *ServiceImpl) redisUserExists(ctx context.Context, username string) bool {
	if s.redis == nil {
		return false
	}
	count, err := s.redis.Exists(ctx, userKey(username)).Result()
	return err == nil && count > 0
}

func (s *ServiceImpl) addUserToBloom(ctx context.Context, username string) {
	if s.bloom == nil {
		return
	}
	_ = s.bloom.Add(ctx, username)
}

func (s *ServiceImpl) bootstrapUserRegisterBloom(ctx context.Context) {
	if s.redis == nil || s.bloom == nil {
		return
	}
	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, userKey("*"), 1000).Result()
		if err != nil {
			return
		}
		for _, key := range keys {
			username := strings.TrimPrefix(key, userKey(""))
			if username != "" {
				s.addUserToBloom(ctx, username)
			}
		}
		if nextCursor == 0 {
			return
		}
		cursor = nextCursor
	}
}

func (s *ServiceImpl) acquireRegisterLock(ctx context.Context, username string) func() {
	if s.redis == nil {
		return func() {}
	}
	token := uuid.NewString()
	key := registerLockKey(username)
	ok, err := s.redis.SetNX(ctx, key, token, 30*time.Second).Result()
	if err != nil || !ok {
		return nil
	}
	return func() {
		const unlockScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0`
		_, _ = s.redis.Eval(context.Background(), unlockScript, []string{key}, token).Result()
	}
}

func (s *ServiceImpl) saveUser(ctx context.Context, username string, stored *storedUser) {
	if s.redis == nil || stored == nil {
		return
	}
	data, err := json.Marshal(stored)
	if err == nil {
		_ = s.redis.Set(ctx, userKey(username), data, 0).Err()
	}
}

func (s *ServiceImpl) getToken(ctx context.Context, token string) (string, bool) {
	if username, ok := s.tokens[token]; ok {
		return username, true
	}
	if s.redis == nil {
		return "", false
	}
	username, err := s.redis.Get(ctx, tokenKey(token)).Result()
	if err != nil || strings.TrimSpace(username) == "" {
		return "", false
	}
	s.tokens[token] = username
	return username, true
}

func (s *ServiceImpl) saveToken(ctx context.Context, token string, username string) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Set(ctx, tokenKey(token), username, 24*time.Hour).Err()
}

func (s *ServiceImpl) deleteToken(ctx context.Context, token string) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Del(ctx, tokenKey(token)).Err()
}

func userKey(username string) string {
	return "ai-prism:user:" + username
}

func registerLockKey(username string) string {
	return "lingzhi:lock_user-register:" + username
}

func tokenKey(token string) string {
	return "ai-prism:auth:token:" + token
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}

func authPayload(token string, u *user.User) *user.AuthPayload {
	cloned := cloneUser(u)
	return &user.AuthPayload{
		Token:       token,
		User:        cloned,
		CurrentUser: cloned,
	}
}

func cloneUser(u *user.User) *user.User {
	if u == nil {
		return nil
	}
	copied := *u
	return &copied
}

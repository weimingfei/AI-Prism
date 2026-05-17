package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	HTTP         HTTPConfig
	MySQL        MySQLConfig
	Mongo        MongoConfig
	Redis        RedisConfig
	Bloom        BloomConfig
	VectorDB     VectorDBConfig
	AI           AIConfig
	SingleFlight SingleFlightConfig
	ASR          ASRConfig
	MinerU       MinerUConfig
	Xunfei       XunfeiConfig
	EinoDev      EinoDevConfig
}

type HTTPConfig struct {
	Addr string
	Mode string
}

type MySQLConfig struct {
	DSN string
}

type MongoConfig struct {
	URI      string
	Database string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type BloomConfig struct {
	UserRegisterEnabled       bool    `toml:"user_register_enabled"`
	UserRegisterKey           string  `toml:"user_register_key"`
	UserRegisterExpectedItems uint64  `toml:"user_register_expected_items"`
	UserRegisterFalsePositive float64 `toml:"user_register_false_positive"`
	UserRegisterBootstrapScan bool    `toml:"user_register_bootstrap_scan"`
}

type VectorDBConfig struct {
	PostgresDSN string
	Provider    string
}

type AIConfig struct {
	Provider    string  `toml:"provider"`
	BaseURL     string  `toml:"base_url"`
	APIKey      string  `toml:"api_key"`
	Model       string  `toml:"model"`
	Temperature float32 `toml:"temperature"`
}

type SingleFlightConfig struct {
	Enabled                    bool   `toml:"enabled"`
	Mode                       string `toml:"mode"`
	DistributedEnabled         bool   `toml:"distributed_enabled"`
	RunningTTLMillis           int    `toml:"running_ttl_millis"`
	TakeoverDetectMillis       int    `toml:"takeover_detect_millis"`
	ResultTTLMillis            int    `toml:"result_ttl_millis"`
	FailedResultTTLMillis      int    `toml:"failed_result_ttl_millis"`
	FollowerMaxWaitMillis      int    `toml:"follower_max_wait_millis"`
	PollFallbackIntervalMillis int    `toml:"poll_fallback_interval_millis"`
	HeartbeatIntervalMillis    int    `toml:"heartbeat_interval_millis"`
	L1CacheEnabled             bool   `toml:"l1_cache_enabled"`
	L1CacheMaxSize             int    `toml:"l1_cache_max_size"`
	L1CacheTTLMillis           int    `toml:"l1_cache_ttl_millis"`
	CompressionCodec           string `toml:"compression_codec"`
	CompressionThresholdBytes  int    `toml:"compression_threshold_bytes"`
}

type ASRConfig struct {
	BaseURL string
	APIKey  string
}

type MinerUConfig struct {
	Enabled        bool   `toml:"enabled"`
	BaseURL        string `toml:"base_url"`
	APIKey         string `toml:"api_key"`
	ParseEndpoint  string `toml:"parse_endpoint"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

type XunfeiConfig struct {
	AppID      string `toml:"app_id"`
	APIKey     string `toml:"api_key"`
	APISecret  string `toml:"api_secret"`
	ASREnabled bool   `toml:"asr_enabled"`
	ASRWSURL   string `toml:"asr_ws_url"`
	TTSEnabled bool   `toml:"tts_enabled"`
	TTSWSURL   string `toml:"tts_ws_url"`
}

type EinoDevConfig struct {
	Enabled bool   `toml:"enabled"`
	IP      string `toml:"ip"`
	Port    string `toml:"port"`
}

func Load() Config {
	cfg := Config{
		HTTP: HTTPConfig{
			Addr: env("HTTP_ADDR", ":8080"),
			Mode: env("GIN_MODE", "debug"),
		},
		MySQL: MySQLConfig{
			DSN: env("MYSQL_DSN", ""),
		},
		Mongo: MongoConfig{
			URI:      env("MONGO_URI", "mongodb://localhost:27017"),
			Database: env("MONGO_DATABASE", "ai_prism"),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       0,
		},
		Bloom: BloomConfig{
			UserRegisterEnabled:       envBool("BLOOM_USER_REGISTER_ENABLED", true),
			UserRegisterKey:           env("BLOOM_USER_REGISTER_KEY", "userRegisterCachePenetrationBloomFilter"),
			UserRegisterExpectedItems: envUint64("BLOOM_USER_REGISTER_EXPECTED_ITEMS", 100000000),
			UserRegisterFalsePositive: envFloat64("BLOOM_USER_REGISTER_FALSE_POSITIVE", 0.001),
			UserRegisterBootstrapScan: envBool("BLOOM_USER_REGISTER_BOOTSTRAP_SCAN", true),
		},
		VectorDB: VectorDBConfig{
			PostgresDSN: env("PGVECTOR_DSN", ""),
			Provider:    env("VECTOR_PROVIDER", "pgvector"),
		},
		AI: AIConfig{
			Provider: env("AI_PROVIDER", ""),
			BaseURL:  strings.TrimRight(env("AI_BASE_URL", ""), "/"),
			APIKey:   env("AI_API_KEY", ""),
			Model:    env("AI_MODEL", ""),
		},
		SingleFlight: SingleFlightConfig{
			Enabled:                    envBool("AI_SINGLEFLIGHT_ENABLED", true),
			Mode:                       env("AI_SINGLEFLIGHT_MODE", "hybrid"),
			DistributedEnabled:         envBool("AI_SINGLEFLIGHT_DISTRIBUTED_ENABLED", true),
			RunningTTLMillis:           envInt("AI_SINGLEFLIGHT_RUNNING_TTL_MILLIS", 15000),
			TakeoverDetectMillis:       envInt("AI_SINGLEFLIGHT_TAKEOVER_DETECT_MILLIS", 10000),
			ResultTTLMillis:            envInt("AI_SINGLEFLIGHT_RESULT_TTL_MILLIS", 600000),
			FailedResultTTLMillis:      envInt("AI_SINGLEFLIGHT_FAILED_RESULT_TTL_MILLIS", 60000),
			FollowerMaxWaitMillis:      envInt("AI_SINGLEFLIGHT_FOLLOWER_MAX_WAIT_MILLIS", 20000),
			PollFallbackIntervalMillis: envInt("AI_SINGLEFLIGHT_POLL_FALLBACK_INTERVAL_MILLIS", 2000),
			HeartbeatIntervalMillis:    envInt("AI_SINGLEFLIGHT_HEARTBEAT_INTERVAL_MILLIS", 3000),
			L1CacheEnabled:             envBool("AI_SINGLEFLIGHT_L1_CACHE_ENABLED", true),
			L1CacheMaxSize:             envInt("AI_SINGLEFLIGHT_L1_CACHE_MAX_SIZE", 1000),
			L1CacheTTLMillis:           envInt("AI_SINGLEFLIGHT_L1_CACHE_TTL_MILLIS", 30000),
			CompressionCodec:           env("AI_SINGLEFLIGHT_COMPRESSION_CODEC", "gzip"),
			CompressionThresholdBytes:  envInt("AI_SINGLEFLIGHT_COMPRESSION_THRESHOLD_BYTES", 4096),
		},
		ASR: ASRConfig{
			BaseURL: strings.TrimRight(env("ASR_BASE_URL", ""), "/"),
			APIKey:  env("ASR_API_KEY", ""),
		},
		MinerU: MinerUConfig{
			Enabled:        envBool("MINERU_ENABLED", false),
			BaseURL:        strings.TrimRight(env("MINERU_BASE_URL", ""), "/"),
			APIKey:         env("MINERU_API_KEY", ""),
			ParseEndpoint:  env("MINERU_PARSE_ENDPOINT", "/api/v1/pdf/parse"),
			TimeoutSeconds: envInt("MINERU_TIMEOUT_SECONDS", 120),
		},
		Xunfei: XunfeiConfig{
			AppID:      env("XUNFEI_APP_ID", ""),
			APIKey:     env("XUNFEI_API_KEY", ""),
			APISecret:  env("XUNFEI_API_SECRET", ""),
			ASREnabled: envBool("XUNFEI_ASR_ENABLED", false),
			ASRWSURL:   env("XUNFEI_ASR_WS_URL", "wss://iat-api.xfyun.cn/v2/iat"),
			TTSEnabled: envBool("XUNFEI_TTS_ENABLED", false),
			TTSWSURL:   env("XUNFEI_TTS_WS_URL", "wss://tts-api.xfyun.cn/v2/tts"),
		},
		EinoDev: EinoDevConfig{
			Enabled: envBool("EINO_DEV_ENABLED", false),
			IP:      env("EINO_DEV_IP", "127.0.0.1"),
			Port:    env("EINO_DEV_PORT", "52538"),
		},
	}
	loadFromTOML(&cfg)
	applyAIEnvOverrides(&cfg.AI)
	applyBloomEnvOverrides(&cfg.Bloom)
	applySingleFlightEnvOverrides(&cfg.SingleFlight)
	applyMinerUEnvOverrides(&cfg.MinerU)
	applyXunfeiEnvOverrides(&cfg.Xunfei)
	applyEinoDevEnvOverrides(&cfg.EinoDev)
	cfg.AI.BaseURL = strings.TrimRight(cfg.AI.BaseURL, "/")
	cfg.MinerU.BaseURL = strings.TrimRight(cfg.MinerU.BaseURL, "/")
	if strings.TrimSpace(cfg.MinerU.ParseEndpoint) == "" {
		cfg.MinerU.ParseEndpoint = "/api/v1/pdf/parse"
	}
	if cfg.MinerU.TimeoutSeconds <= 0 {
		cfg.MinerU.TimeoutSeconds = 120
	}
	if strings.TrimSpace(cfg.Xunfei.ASRWSURL) == "" {
		cfg.Xunfei.ASRWSURL = "wss://iat-api.xfyun.cn/v2/iat"
	}
	if strings.TrimSpace(cfg.Xunfei.TTSWSURL) == "" {
		cfg.Xunfei.TTSWSURL = "wss://tts-api.xfyun.cn/v2/tts"
	}
	if strings.TrimSpace(cfg.EinoDev.IP) == "" {
		cfg.EinoDev.IP = "127.0.0.1"
	}
	if strings.TrimSpace(cfg.EinoDev.Port) == "" {
		cfg.EinoDev.Port = "52538"
	}
	return cfg
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envUint64(key string, fallback uint64) uint64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat64(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func loadFromTOML(cfg *Config) {
	path := configFilePath()
	if path == "" {
		return
	}

	var file struct {
		AI           AIConfig           `toml:"ai"`
		MySQL        MySQLConfig        `toml:"mysql"`
		Mongo        MongoConfig        `toml:"mongo"`
		Redis        RedisConfig        `toml:"redis"`
		Bloom        BloomConfig        `toml:"bloom"`
		VectorDB     VectorDBConfig     `toml:"vector_db"`
		SingleFlight SingleFlightConfig `toml:"ai_singleflight"`
		MinerU       MinerUConfig       `toml:"mineru"`
		Xunfei       XunfeiConfig       `toml:"xunfei"`
		EinoDev      EinoDevConfig      `toml:"eino_dev"`
	}
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return
	}
	if strings.TrimSpace(file.AI.Provider) != "" {
		cfg.AI.Provider = file.AI.Provider
	}
	if strings.TrimSpace(file.AI.BaseURL) != "" {
		cfg.AI.BaseURL = file.AI.BaseURL
	}
	if strings.TrimSpace(file.AI.APIKey) != "" {
		cfg.AI.APIKey = file.AI.APIKey
	}
	if strings.TrimSpace(file.AI.Model) != "" {
		cfg.AI.Model = file.AI.Model
	}
	if file.AI.Temperature != 0 {
		cfg.AI.Temperature = file.AI.Temperature
	}
	if strings.TrimSpace(file.MySQL.DSN) != "" {
		cfg.MySQL.DSN = file.MySQL.DSN
	}
	if strings.TrimSpace(file.Mongo.URI) != "" {
		cfg.Mongo.URI = file.Mongo.URI
	}
	if strings.TrimSpace(file.Mongo.Database) != "" {
		cfg.Mongo.Database = file.Mongo.Database
	}
	if strings.TrimSpace(file.Redis.Addr) != "" {
		cfg.Redis.Addr = file.Redis.Addr
	}
	if strings.TrimSpace(file.Redis.Password) != "" {
		cfg.Redis.Password = file.Redis.Password
	}
	if file.Redis.DB != 0 {
		cfg.Redis.DB = file.Redis.DB
	}
	applyBloomFileOverrides(&cfg.Bloom, file.Bloom)
	if strings.TrimSpace(file.VectorDB.Provider) != "" {
		cfg.VectorDB.Provider = file.VectorDB.Provider
	}
	if strings.TrimSpace(file.VectorDB.PostgresDSN) != "" {
		cfg.VectorDB.PostgresDSN = file.VectorDB.PostgresDSN
	}
	applySingleFlightFileOverrides(&cfg.SingleFlight, file.SingleFlight)
	if file.MinerU.Enabled {
		cfg.MinerU.Enabled = true
	}
	if strings.TrimSpace(file.MinerU.BaseURL) != "" {
		cfg.MinerU.BaseURL = file.MinerU.BaseURL
	}
	if strings.TrimSpace(file.MinerU.APIKey) != "" {
		cfg.MinerU.APIKey = file.MinerU.APIKey
	}
	if strings.TrimSpace(file.MinerU.ParseEndpoint) != "" {
		cfg.MinerU.ParseEndpoint = file.MinerU.ParseEndpoint
	}
	if file.MinerU.TimeoutSeconds > 0 {
		cfg.MinerU.TimeoutSeconds = file.MinerU.TimeoutSeconds
	}
	if strings.TrimSpace(file.Xunfei.AppID) != "" {
		cfg.Xunfei.AppID = file.Xunfei.AppID
	}
	if strings.TrimSpace(file.Xunfei.APIKey) != "" {
		cfg.Xunfei.APIKey = file.Xunfei.APIKey
	}
	if strings.TrimSpace(file.Xunfei.APISecret) != "" {
		cfg.Xunfei.APISecret = file.Xunfei.APISecret
	}
	if file.Xunfei.ASREnabled {
		cfg.Xunfei.ASREnabled = true
	}
	if strings.TrimSpace(file.Xunfei.ASRWSURL) != "" {
		cfg.Xunfei.ASRWSURL = file.Xunfei.ASRWSURL
	}
	if file.Xunfei.TTSEnabled {
		cfg.Xunfei.TTSEnabled = true
	}
	if strings.TrimSpace(file.Xunfei.TTSWSURL) != "" {
		cfg.Xunfei.TTSWSURL = file.Xunfei.TTSWSURL
	}
	if file.EinoDev.Enabled {
		cfg.EinoDev.Enabled = true
	}
	if strings.TrimSpace(file.EinoDev.IP) != "" {
		cfg.EinoDev.IP = file.EinoDev.IP
	}
	if strings.TrimSpace(file.EinoDev.Port) != "" {
		cfg.EinoDev.Port = file.EinoDev.Port
	}
}

func applyAIEnvOverrides(cfg *AIConfig) {
	if value := env("AI_PROVIDER", ""); value != "" {
		cfg.Provider = value
	}
	if value := env("AI_BASE_URL", ""); value != "" {
		cfg.BaseURL = value
	}
	if value := env("AI_API_KEY", ""); value != "" {
		cfg.APIKey = value
	}
	if value := env("AI_MODEL", ""); value != "" {
		cfg.Model = value
	}
}

func applyBloomFileOverrides(cfg *BloomConfig, file BloomConfig) {
	if file.UserRegisterEnabled {
		cfg.UserRegisterEnabled = true
	}
	if strings.TrimSpace(file.UserRegisterKey) != "" {
		cfg.UserRegisterKey = file.UserRegisterKey
	}
	if file.UserRegisterExpectedItems > 0 {
		cfg.UserRegisterExpectedItems = file.UserRegisterExpectedItems
	}
	if file.UserRegisterFalsePositive > 0 {
		cfg.UserRegisterFalsePositive = file.UserRegisterFalsePositive
	}
	if file.UserRegisterBootstrapScan {
		cfg.UserRegisterBootstrapScan = true
	}
}

func applyBloomEnvOverrides(cfg *BloomConfig) {
	if value := env("BLOOM_USER_REGISTER_ENABLED", ""); value != "" {
		cfg.UserRegisterEnabled, _ = strconv.ParseBool(value)
	}
	if value := env("BLOOM_USER_REGISTER_KEY", ""); value != "" {
		cfg.UserRegisterKey = value
	}
	if value := env("BLOOM_USER_REGISTER_EXPECTED_ITEMS", ""); value != "" {
		cfg.UserRegisterExpectedItems = envUint64("BLOOM_USER_REGISTER_EXPECTED_ITEMS", cfg.UserRegisterExpectedItems)
	}
	if value := env("BLOOM_USER_REGISTER_FALSE_POSITIVE", ""); value != "" {
		cfg.UserRegisterFalsePositive = envFloat64("BLOOM_USER_REGISTER_FALSE_POSITIVE", cfg.UserRegisterFalsePositive)
	}
	if value := env("BLOOM_USER_REGISTER_BOOTSTRAP_SCAN", ""); value != "" {
		cfg.UserRegisterBootstrapScan, _ = strconv.ParseBool(value)
	}
}

func applySingleFlightFileOverrides(cfg *SingleFlightConfig, file SingleFlightConfig) {
	if file.Enabled {
		cfg.Enabled = true
	}
	if strings.TrimSpace(file.Mode) != "" {
		cfg.Mode = file.Mode
	}
	if file.DistributedEnabled {
		cfg.DistributedEnabled = true
	}
	if file.RunningTTLMillis > 0 {
		cfg.RunningTTLMillis = file.RunningTTLMillis
	}
	if file.TakeoverDetectMillis > 0 {
		cfg.TakeoverDetectMillis = file.TakeoverDetectMillis
	}
	if file.ResultTTLMillis > 0 {
		cfg.ResultTTLMillis = file.ResultTTLMillis
	}
	if file.FailedResultTTLMillis > 0 {
		cfg.FailedResultTTLMillis = file.FailedResultTTLMillis
	}
	if file.FollowerMaxWaitMillis > 0 {
		cfg.FollowerMaxWaitMillis = file.FollowerMaxWaitMillis
	}
	if file.PollFallbackIntervalMillis > 0 {
		cfg.PollFallbackIntervalMillis = file.PollFallbackIntervalMillis
	}
	if file.HeartbeatIntervalMillis > 0 {
		cfg.HeartbeatIntervalMillis = file.HeartbeatIntervalMillis
	}
	if file.L1CacheEnabled {
		cfg.L1CacheEnabled = true
	}
	if file.L1CacheMaxSize > 0 {
		cfg.L1CacheMaxSize = file.L1CacheMaxSize
	}
	if file.L1CacheTTLMillis > 0 {
		cfg.L1CacheTTLMillis = file.L1CacheTTLMillis
	}
	if strings.TrimSpace(file.CompressionCodec) != "" {
		cfg.CompressionCodec = file.CompressionCodec
	}
	if file.CompressionThresholdBytes > 0 {
		cfg.CompressionThresholdBytes = file.CompressionThresholdBytes
	}
}

func applySingleFlightEnvOverrides(cfg *SingleFlightConfig) {
	if value := env("AI_SINGLEFLIGHT_ENABLED", ""); value != "" {
		cfg.Enabled, _ = strconv.ParseBool(value)
	}
	if value := env("AI_SINGLEFLIGHT_MODE", ""); value != "" {
		cfg.Mode = value
	}
	if value := env("AI_SINGLEFLIGHT_DISTRIBUTED_ENABLED", ""); value != "" {
		cfg.DistributedEnabled, _ = strconv.ParseBool(value)
	}
	if value := env("AI_SINGLEFLIGHT_RUNNING_TTL_MILLIS", ""); value != "" {
		cfg.RunningTTLMillis = envInt("AI_SINGLEFLIGHT_RUNNING_TTL_MILLIS", cfg.RunningTTLMillis)
	}
	if value := env("AI_SINGLEFLIGHT_TAKEOVER_DETECT_MILLIS", ""); value != "" {
		cfg.TakeoverDetectMillis = envInt("AI_SINGLEFLIGHT_TAKEOVER_DETECT_MILLIS", cfg.TakeoverDetectMillis)
	}
	if value := env("AI_SINGLEFLIGHT_RESULT_TTL_MILLIS", ""); value != "" {
		cfg.ResultTTLMillis = envInt("AI_SINGLEFLIGHT_RESULT_TTL_MILLIS", cfg.ResultTTLMillis)
	}
	if value := env("AI_SINGLEFLIGHT_FAILED_RESULT_TTL_MILLIS", ""); value != "" {
		cfg.FailedResultTTLMillis = envInt("AI_SINGLEFLIGHT_FAILED_RESULT_TTL_MILLIS", cfg.FailedResultTTLMillis)
	}
	if value := env("AI_SINGLEFLIGHT_FOLLOWER_MAX_WAIT_MILLIS", ""); value != "" {
		cfg.FollowerMaxWaitMillis = envInt("AI_SINGLEFLIGHT_FOLLOWER_MAX_WAIT_MILLIS", cfg.FollowerMaxWaitMillis)
	}
	if value := env("AI_SINGLEFLIGHT_POLL_FALLBACK_INTERVAL_MILLIS", ""); value != "" {
		cfg.PollFallbackIntervalMillis = envInt("AI_SINGLEFLIGHT_POLL_FALLBACK_INTERVAL_MILLIS", cfg.PollFallbackIntervalMillis)
	}
	if value := env("AI_SINGLEFLIGHT_HEARTBEAT_INTERVAL_MILLIS", ""); value != "" {
		cfg.HeartbeatIntervalMillis = envInt("AI_SINGLEFLIGHT_HEARTBEAT_INTERVAL_MILLIS", cfg.HeartbeatIntervalMillis)
	}
	if value := env("AI_SINGLEFLIGHT_L1_CACHE_ENABLED", ""); value != "" {
		cfg.L1CacheEnabled, _ = strconv.ParseBool(value)
	}
	if value := env("AI_SINGLEFLIGHT_L1_CACHE_MAX_SIZE", ""); value != "" {
		cfg.L1CacheMaxSize = envInt("AI_SINGLEFLIGHT_L1_CACHE_MAX_SIZE", cfg.L1CacheMaxSize)
	}
	if value := env("AI_SINGLEFLIGHT_L1_CACHE_TTL_MILLIS", ""); value != "" {
		cfg.L1CacheTTLMillis = envInt("AI_SINGLEFLIGHT_L1_CACHE_TTL_MILLIS", cfg.L1CacheTTLMillis)
	}
	if value := env("AI_SINGLEFLIGHT_COMPRESSION_CODEC", ""); value != "" {
		cfg.CompressionCodec = value
	}
	if value := env("AI_SINGLEFLIGHT_COMPRESSION_THRESHOLD_BYTES", ""); value != "" {
		cfg.CompressionThresholdBytes = envInt("AI_SINGLEFLIGHT_COMPRESSION_THRESHOLD_BYTES", cfg.CompressionThresholdBytes)
	}
}

func applyMinerUEnvOverrides(cfg *MinerUConfig) {
	if value := env("MINERU_ENABLED", ""); value != "" {
		cfg.Enabled, _ = strconv.ParseBool(value)
	}
	if value := env("MINERU_BASE_URL", ""); value != "" {
		cfg.BaseURL = value
	}
	if value := env("MINERU_API_KEY", ""); value != "" {
		cfg.APIKey = value
	}
	if value := env("MINERU_PARSE_ENDPOINT", ""); value != "" {
		cfg.ParseEndpoint = value
	}
	if value := env("MINERU_TIMEOUT_SECONDS", ""); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.TimeoutSeconds = parsed
		}
	}
}

func applyXunfeiEnvOverrides(cfg *XunfeiConfig) {
	if value := env("XUNFEI_APP_ID", ""); value != "" {
		cfg.AppID = value
	}
	if value := env("XUNFEI_API_KEY", ""); value != "" {
		cfg.APIKey = value
	}
	if value := env("XUNFEI_API_SECRET", ""); value != "" {
		cfg.APISecret = value
	}
	if value := env("XUNFEI_ASR_ENABLED", ""); value != "" {
		cfg.ASREnabled, _ = strconv.ParseBool(value)
	}
	if value := env("XUNFEI_ASR_WS_URL", ""); value != "" {
		cfg.ASRWSURL = value
	}
	if value := env("XUNFEI_TTS_ENABLED", ""); value != "" {
		cfg.TTSEnabled, _ = strconv.ParseBool(value)
	}
	if value := env("XUNFEI_TTS_WS_URL", ""); value != "" {
		cfg.TTSWSURL = value
	}
}

func applyEinoDevEnvOverrides(cfg *EinoDevConfig) {
	if value := env("EINO_DEV_ENABLED", ""); value != "" {
		cfg.Enabled, _ = strconv.ParseBool(value)
	}
	if value := env("EINO_DEV_IP", ""); value != "" {
		cfg.IP = value
	}
	if value := env("EINO_DEV_PORT", ""); value != "" {
		cfg.Port = value
	}
}

func configFilePath() string {
	args := os.Args
	for index, arg := range args {
		if (arg == "-f" || arg == "--config") && index+1 < len(args) {
			return args[index+1]
		}
		if strings.HasPrefix(arg, "-f=") {
			return strings.TrimPrefix(arg, "-f=")
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}

	defaultPath := filepath.Join("etc", "application.toml")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	return ""
}

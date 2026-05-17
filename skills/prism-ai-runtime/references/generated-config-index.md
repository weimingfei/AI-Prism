<!-- GENERATED: run `python skills/prism-ai-runtime/scripts/extract_config_index.py` from repo root. Do not edit by hand. -->

# 配置索引

## TOML 配置段

- `[app]`
- `[http]`
- `[gin_webframework]`
- `[log]`
- `[trace]`
- `[ai]`
- `[ai_singleflight]`
- `[mineru]`
- `[xunfei]`
- `[eino_dev]`
- `[mysql]`
- `[mongo]`
- `[redis]`
- `[bloom]`
- `[vector_db]`

## Go 配置结构体字段

| 结构体 | 字段 | TOML key |
| --- | --- | --- |
| `Config` | `HTTP` | `-` |
| `Config` | `MySQL` | `-` |
| `Config` | `Mongo` | `-` |
| `Config` | `Redis` | `-` |
| `Config` | `Bloom` | `-` |
| `Config` | `VectorDB` | `-` |
| `Config` | `AI` | `-` |
| `Config` | `SingleFlight` | `-` |
| `Config` | `ASR` | `-` |
| `Config` | `MinerU` | `-` |
| `Config` | `Xunfei` | `-` |
| `Config` | `EinoDev` | `-` |
| `HTTPConfig` | `Addr` | `-` |
| `HTTPConfig` | `Mode` | `-` |
| `MySQLConfig` | `DSN` | `-` |
| `MongoConfig` | `URI` | `-` |
| `MongoConfig` | `Database` | `-` |
| `RedisConfig` | `Addr` | `-` |
| `RedisConfig` | `Password` | `-` |
| `RedisConfig` | `DB` | `-` |
| `BloomConfig` | `UserRegisterEnabled` | `user_register_enabled` |
| `BloomConfig` | `UserRegisterKey` | `user_register_key` |
| `BloomConfig` | `UserRegisterExpectedItems` | `user_register_expected_items` |
| `BloomConfig` | `UserRegisterFalsePositive` | `user_register_false_positive` |
| `BloomConfig` | `UserRegisterBootstrapScan` | `user_register_bootstrap_scan` |
| `VectorDBConfig` | `PostgresDSN` | `-` |
| `VectorDBConfig` | `Provider` | `-` |
| `AIConfig` | `Provider` | `provider` |
| `AIConfig` | `BaseURL` | `base_url` |
| `AIConfig` | `APIKey` | `api_key` |
| `AIConfig` | `Model` | `model` |
| `AIConfig` | `Temperature` | `temperature` |
| `SingleFlightConfig` | `Enabled` | `enabled` |
| `SingleFlightConfig` | `Mode` | `mode` |
| `SingleFlightConfig` | `DistributedEnabled` | `distributed_enabled` |
| `SingleFlightConfig` | `RunningTTLMillis` | `running_ttl_millis` |
| `SingleFlightConfig` | `TakeoverDetectMillis` | `takeover_detect_millis` |
| `SingleFlightConfig` | `ResultTTLMillis` | `result_ttl_millis` |
| `SingleFlightConfig` | `FailedResultTTLMillis` | `failed_result_ttl_millis` |
| `SingleFlightConfig` | `FollowerMaxWaitMillis` | `follower_max_wait_millis` |
| `SingleFlightConfig` | `PollFallbackIntervalMillis` | `poll_fallback_interval_millis` |
| `SingleFlightConfig` | `HeartbeatIntervalMillis` | `heartbeat_interval_millis` |
| `SingleFlightConfig` | `L1CacheEnabled` | `l1_cache_enabled` |
| `SingleFlightConfig` | `L1CacheMaxSize` | `l1_cache_max_size` |
| `SingleFlightConfig` | `L1CacheTTLMillis` | `l1_cache_ttl_millis` |
| `SingleFlightConfig` | `CompressionCodec` | `compression_codec` |
| `SingleFlightConfig` | `CompressionThresholdBytes` | `compression_threshold_bytes` |
| `ASRConfig` | `BaseURL` | `-` |
| `ASRConfig` | `APIKey` | `-` |
| `MinerUConfig` | `Enabled` | `enabled` |
| `MinerUConfig` | `BaseURL` | `base_url` |
| `MinerUConfig` | `APIKey` | `api_key` |
| `MinerUConfig` | `ParseEndpoint` | `parse_endpoint` |
| `MinerUConfig` | `TimeoutSeconds` | `timeout_seconds` |
| `XunfeiConfig` | `AppID` | `app_id` |
| `XunfeiConfig` | `APIKey` | `api_key` |
| `XunfeiConfig` | `APISecret` | `api_secret` |
| `XunfeiConfig` | `ASREnabled` | `asr_enabled` |
| `XunfeiConfig` | `ASRWSURL` | `asr_ws_url` |
| `XunfeiConfig` | `TTSEnabled` | `tts_enabled` |
| `XunfeiConfig` | `TTSWSURL` | `tts_ws_url` |
| `EinoDevConfig` | `Enabled` | `enabled` |
| `EinoDevConfig` | `IP` | `ip` |
| `EinoDevConfig` | `Port` | `port` |

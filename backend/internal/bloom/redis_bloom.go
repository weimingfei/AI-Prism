package bloom

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type RedisBloomFilter struct {
	redis     *redis.Client
	key       string
	metaKey   string
	bitSize   uint64
	hashCount uint64
}

type Config struct {
	Key                string
	ExpectedInsertions uint64
	FalsePositiveRate  float64
}

func NewRedisBloomFilter(redisClient *redis.Client, cfg Config) (*RedisBloomFilter, error) {
	if redisClient == nil {
		return nil, errors.New("redis bloom filter requires redis client")
	}
	key := strings.TrimSpace(cfg.Key)
	if key == "" {
		return nil, errors.New("redis bloom filter key is empty")
	}
	bitSize, hashCount := parameters(cfg.ExpectedInsertions, cfg.FalsePositiveRate)
	return &RedisBloomFilter{
		redis:     redisClient,
		key:       key,
		metaKey:   key + ":metadata",
		bitSize:   bitSize,
		hashCount: hashCount,
	}, nil
}

// TryInit 只初始化元信息，不清空 bitmap。线上重启时不能误删已经写入的用户名指纹。
func (f *RedisBloomFilter) TryInit(ctx context.Context) error {
	_, err := f.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSetNX(ctx, f.metaKey, "size", strconv.FormatUint(f.bitSize, 10))
		pipe.HSetNX(ctx, f.metaKey, "hashIterations", strconv.FormatUint(f.hashCount, 10))
		pipe.HSetNX(ctx, f.metaKey, "type", "go-redis-bloom")
		return nil
	})
	return err
}

func (f *RedisBloomFilter) Add(ctx context.Context, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	positions := f.positions(value)
	_, err := f.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, position := range positions {
			pipe.SetBit(ctx, f.key, int64(position), 1)
		}
		return nil
	})
	return err
}

func (f *RedisBloomFilter) Contains(ctx context.Context, value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	positions := f.positions(value)
	cmds := make([]*redis.IntCmd, 0, len(positions))
	_, err := f.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, position := range positions {
			cmds = append(cmds, pipe.GetBit(ctx, f.key, int64(position)))
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	for _, cmd := range cmds {
		if cmd.Val() == 0 {
			return false, nil
		}
	}
	return true, nil
}

func (f *RedisBloomFilter) positions(value string) []uint64 {
	first, second := hashPair(value)
	if second == 0 {
		second = 0x9e3779b97f4a7c15
	}
	positions := make([]uint64, 0, f.hashCount)
	for i := uint64(0); i < f.hashCount; i++ {
		positions = append(positions, (first+i*second+i*i)%f.bitSize)
	}
	return positions
}

func hashPair(value string) (uint64, uint64) {
	sum := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint64(sum[0:8]), binary.BigEndian.Uint64(sum[8:16])
}

func parameters(expectedInsertions uint64, falsePositiveRate float64) (uint64, uint64) {
	if expectedInsertions == 0 {
		expectedInsertions = 100000000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.001
	}
	n := float64(expectedInsertions)
	m := math.Ceil(-(n * math.Log(falsePositiveRate)) / (math.Ln2 * math.Ln2))
	k := math.Ceil((m / n) * math.Ln2)
	if m < 1 {
		m = 1
	}
	if k < 1 {
		k = 1
	}
	return uint64(m), uint64(k)
}

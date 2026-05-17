package bloom

import "testing"

func TestParametersMatchRedissonStyleDefaults(t *testing.T) {
	bitSize, hashCount := parameters(100000000, 0.001)
	if bitSize != 1437758757 {
		t.Fatalf("unexpected bit size: %d", bitSize)
	}
	if hashCount != 10 {
		t.Fatalf("unexpected hash count: %d", hashCount)
	}
}

func TestPositionsAreStableAndBounded(t *testing.T) {
	filter := &RedisBloomFilter{bitSize: 1024, hashCount: 7}
	first := filter.positions("fblossoms")
	second := filter.positions("fblossoms")
	if len(first) != 7 {
		t.Fatalf("unexpected positions length: %d", len(first))
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("positions are not stable")
		}
		if first[index] >= 1024 {
			t.Fatalf("position out of range: %d", first[index])
		}
	}
}

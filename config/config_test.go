package config

import "testing"

func TestRedisStatusSetsUseBridgeOpsNamespace(t *testing.T) {
	for status, key := range RedisStatusSets {
		if key == "" {
			t.Fatalf("status %q has an empty Redis key", status)
		}

		if wantPrefix := "bridgeops:"; len(key) < len(wantPrefix) || key[:len(wantPrefix)] != wantPrefix {
			t.Fatalf("status %q uses Redis key %q, want prefix %q", status, key, wantPrefix)
		}
	}
}

func TestReturnSuccessRedisStatusKey(t *testing.T) {
	if got, want := RedisStatusSets["returnsuccess"], "bridgeops:returnsuccess"; got != want {
		t.Fatalf("returnsuccess Redis key = %q, want %q", got, want)
	}
}

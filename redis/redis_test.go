package redis

import (
	"encoding/json"
	"testing"

	"gobglbridge/config"
	"gobglbridge/types"

	"github.com/alicebob/miniredis/v2"
	redigo "github.com/gomodule/redigo/redis"
)

func setupTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	server := miniredis.RunT(t)
	pool = &redigo.Pool{
		Dial: func() (redigo.Conn, error) {
			return redigo.Dial("tcp", server.Addr(), timeoutDialOptions()...)
		},
	}

	t.Cleanup(func() {
		_ = pool.Close()
		server.Close()
	})

	return server
}

func TestFindAllBridgeOperationsByStatusSkipsStaleSetMembers(t *testing.T) {
	setupTestRedis(t)

	conn := pool.Get()
	defer conn.Close()

	if _, err := conn.Do("SADD", config.RedisStatusSets["pending"], "bridgeop:pending:missing"); err != nil {
		t.Fatalf("seed stale set member: %v", err)
	}

	ops, err := FindAllBridgeOperationsByStatus("pending")
	if err != nil {
		t.Fatalf("FindAllBridgeOperationsByStatus returned error: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("expected no operations, got %d", len(ops))
	}
}

func TestFindBridgeOperationSourceTxHashSkipsStaleSetMembers(t *testing.T) {
	setupTestRedis(t)

	conn := pool.Get()
	defer conn.Close()

	statusSet := config.RedisStatusSets["pending"]
	if _, err := conn.Do("SADD", statusSet, "bridgeop:pending:missing"); err != nil {
		t.Fatalf("seed stale set member: %v", err)
	}

	op := &types.BridgeOperation{
		ID:           "op-1",
		Status:       "pending",
		SourceTxHash: "source-tx-1",
	}
	opJSON, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("marshal operation: %v", err)
	}
	if _, err := conn.Do("SET", "bridgeop:pending:op-1", opJSON); err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	if _, err := conn.Do("SADD", statusSet, "bridgeop:pending:op-1"); err != nil {
		t.Fatalf("seed operation set member: %v", err)
	}

	got, err := FindBridgeOperationSourceTxHash("source-tx-1")
	if err != nil {
		t.Fatalf("FindBridgeOperationSourceTxHash returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected operation, got nil")
	}
	if got.ID != op.ID {
		t.Fatalf("expected operation ID %q, got %q", op.ID, got.ID)
	}
}

func TestFindBridgeOperationByFieldRejectsUnknownStatus(t *testing.T) {
	setupTestRedis(t)

	_, err := FindBridgeOperationByFieldStringValue("Status", "pending", "unknown")
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}

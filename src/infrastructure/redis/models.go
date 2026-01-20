package redis

import (
	"fmt"

	"github.com/gomodule/redigo/redis"
)

// Model is a boot-time contract for a Redis "model" (key pattern / data structure).
//
// Redis doesn't have schemas; the goal here is:
// - validate a key doesn't exist with the WRONG type
// - create a minimal placeholder (optionally) so the key/type can be verified
//
// You can (and should) customize Ensure() to your domain needs.
type Model interface {
	// Name is used only for diagnostics.
	Name() string
	// Ensure makes sure the model is "present" and compatible in Redis.
	Ensure(conn redis.Conn) error
}

// EnsureModels runs Ensure() for every model in order.
func EnsureModels(conn redis.Conn, models []Model) error {
	for _, m := range models {
		if m == nil {
			continue
		}
		if err := m.Ensure(conn); err != nil {
			return fmt.Errorf("redis model %q: %w", m.Name(), err)
		}
	}
	return nil
}

// --- Helpers (used by templates) ---

func ensureKeyType(conn redis.Conn, key string, want string) error {
	// Redis returns:
	// - "none" when key doesn't exist
	// - one of: string, list, set, zset, hash, stream
	got, err := redis.String(conn.Do("TYPE", key))
	if err != nil {
		return fmt.Errorf("TYPE %s: %w", key, err)
	}
	if got == "none" {
		return nil
	}
	if got != want {
		return fmt.Errorf("type mismatch for key %q: want %q, got %q", key, want, got)
	}
	return nil
}

// EnsureStringKey ensures key is either absent or a Redis string, and seeds it if absent.
// Seed is optional; if empty, it will still create a placeholder ("__init__") so TYPE can be verified.
type EnsureStringKey struct {
	Key  string
	Seed string
}

func (m EnsureStringKey) Name() string { return "string:" + m.Key }
func (m EnsureStringKey) Ensure(conn redis.Conn) error {
	if err := ensureKeyType(conn, m.Key, "string"); err != nil {
		return err
	}
	exists, err := redis.Int(conn.Do("EXISTS", m.Key))
	if err != nil {
		return fmt.Errorf("EXISTS %s: %w", m.Key, err)
	}
	if exists == 1 {
		return nil
	}
	seed := m.Seed
	if seed == "" {
		seed = "__init__"
	}
	_, err = conn.Do("SET", m.Key, seed)
	if err != nil {
		return fmt.Errorf("SET %s: %w", m.Key, err)
	}
	return nil
}

// EnsureHashKey ensures key is either absent or a hash, and seeds one field.
// Note: empty hashes don't exist in Redis, so we seed a placeholder field.
type EnsureHashKey struct {
	Key        string
	SeedField  string
	SeedValue  string
	ExpireSecs int // optional; 0 => no expire
}

func (m EnsureHashKey) Name() string { return "hash:" + m.Key }
func (m EnsureHashKey) Ensure(conn redis.Conn) error {
	if err := ensureKeyType(conn, m.Key, "hash"); err != nil {
		return err
	}
	field := m.SeedField
	if field == "" {
		field = "__init__"
	}
	val := m.SeedValue
	if val == "" {
		val = "1"
	}
	if _, err := conn.Do("HSET", m.Key, field, val); err != nil {
		return fmt.Errorf("HSET %s %s: %w", m.Key, field, err)
	}
	if m.ExpireSecs > 0 {
		if _, err := conn.Do("EXPIRE", m.Key, m.ExpireSecs); err != nil {
			return fmt.Errorf("EXPIRE %s: %w", m.Key, err)
		}
	}
	return nil
}

// EnsureSetKey ensures key is either absent or a set, and seeds one member.
type EnsureSetKey struct {
	Key        string
	SeedMember string
}

func (m EnsureSetKey) Name() string { return "set:" + m.Key }
func (m EnsureSetKey) Ensure(conn redis.Conn) error {
	if err := ensureKeyType(conn, m.Key, "set"); err != nil {
		return err
	}
	member := m.SeedMember
	if member == "" {
		member = "__init__"
	}
	if _, err := conn.Do("SADD", m.Key, member); err != nil {
		return fmt.Errorf("SADD %s: %w", m.Key, err)
	}
	return nil
}

// EnsureZSetKey ensures key is either absent or a zset, and seeds one member.
type EnsureZSetKey struct {
	Key        string
	SeedScore  float64
	SeedMember string
}

func (m EnsureZSetKey) Name() string { return "zset:" + m.Key }
func (m EnsureZSetKey) Ensure(conn redis.Conn) error {
	if err := ensureKeyType(conn, m.Key, "zset"); err != nil {
		return err
	}
	member := m.SeedMember
	if member == "" {
		member = "__init__"
	}
	if _, err := conn.Do("ZADD", m.Key, m.SeedScore, member); err != nil {
		return fmt.Errorf("ZADD %s: %w", m.Key, err)
	}
	return nil
}

// EnsureListKey ensures key is either absent or a list, and seeds one element.
type EnsureListKey struct {
	Key         string
	SeedElement string
}

func (m EnsureListKey) Name() string { return "list:" + m.Key }
func (m EnsureListKey) Ensure(conn redis.Conn) error {
	if err := ensureKeyType(conn, m.Key, "list"); err != nil {
		return err
	}
	el := m.SeedElement
	if el == "" {
		el = "__init__"
	}
	if _, err := conn.Do("RPUSH", m.Key, el); err != nil {
		return fmt.Errorf("RPUSH %s: %w", m.Key, err)
	}
	return nil
}

// EnsureStreamKey ensures key is either absent or a stream, and seeds one entry.
// Note: stream also cannot exist empty.
type EnsureStreamKey struct {
	Key string
	// SeedKV is optional; if empty we use {"__init__": "1"}
	SeedKV map[string]string
}

func (m EnsureStreamKey) Name() string { return "stream:" + m.Key }
func (m EnsureStreamKey) Ensure(conn redis.Conn) error {
	if err := ensureKeyType(conn, m.Key, "stream"); err != nil {
		return err
	}
	kv := m.SeedKV
	if len(kv) == 0 {
		kv = map[string]string{"__init__": "1"}
	}

	// XADD key * field value [field value ...]
	args := redis.Args{}.Add(m.Key).Add("*")
	for k, v := range kv {
		args = args.Add(k).Add(v)
	}
	if _, err := conn.Do("XADD", args...); err != nil {
		return fmt.Errorf("XADD %s: %w", m.Key, err)
	}
	return nil
}

package env

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/antgroup/hugescm/modules/strengthen"
)

type Broker interface {
	ExpandEnv(s string) string
	LookupEnv(key string) (string, bool)
	Getenv(string) string
	Setenv(key, value string) error
	Unsetenv(key string) error
	Environ() []string
	Clearenv()
}

type broker struct {
}

func (b *broker) ExpandEnv(s string) string {
	return os.ExpandEnv(s)
}

func (b *broker) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (b *broker) Getenv(key string) string {
	return os.Getenv(key)
}

func (b *broker) Setenv(key, value string) error {
	return os.Setenv(key, value)
}

func (b *broker) Unsetenv(key string) error {
	return os.Unsetenv(key)
}

func (b *broker) Clearenv() {
	os.Clearenv()
}

func (b *broker) Environ() []string {
	return os.Environ()
}

type sanitizer struct {
	keys map[string]int
	env  []string
	mu   sync.RWMutex
}

func NewSanitizer() Broker {
	b := &sanitizer{
		keys: make(map[string]int),
		env:  slices.Clone(Environ()),
	}
	for i, e := range b.env {
		k, _, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		b.keys[k] = i
	}
	return b
}

func (b *sanitizer) ExpandEnv(s string) string {
	return os.Expand(s, b.Getenv)
}

func (b *sanitizer) LookupEnv(key string) (string, bool) {
	if len(key) == 0 {
		return "", false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	i, ok := b.keys[key]
	if !ok {
		return "", false
	}
	s := b.env[i]
	if len(s) != 0 {
		if _, v, ok := strings.Cut(s, "="); ok {
			return v, true
		}
	}
	return "", false
}

func (b *sanitizer) Getenv(key string) string {
	v, _ := b.LookupEnv(key)
	return v
}

func (b *sanitizer) Setenv(key, value string) error {
	if len(key) == 0 {
		return syscall.EINVAL
	}
	for i := range len(key) {
		if key[i] == '=' || key[i] == 0 {
			return syscall.EINVAL
		}
	}
	kv := key + "=" + value
	b.mu.Lock()
	defer b.mu.Unlock()
	i, ok := b.keys[key]
	if ok {
		b.env[i] = kv
		return nil
	}
	i = len(b.env)
	b.env = append(b.env, kv)
	b.keys[key] = i
	return nil
}

func (b *sanitizer) Unsetenv(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if i, ok := b.keys[key]; ok {
		b.env[i] = ""
		delete(b.keys, key)
	}
	return nil
}

func (b *sanitizer) Clearenv() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.keys = make(map[string]int)
	b.env = []string{}
}

func (b *sanitizer) Environ() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a := make([]string, 0, len(b.env)+16) // Reduce the number of memory allocations
	for _, env := range b.env {
		if env != "" {
			a = append(a, env)
		}
	}
	return a
}

func (b *sanitizer) Find(k K) string {
	return b.Getenv(string(k))
}

func (b *sanitizer) SimpleAtoi(k K, dv int64) int64 {
	v := b.Getenv(string(k))
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	return dv
}

func (b *sanitizer) SimpleAtou(k K, dv uint64) uint64 {
	v := b.Getenv(string(k))
	if i, err := strconv.ParseUint(v, 10, 64); err == nil {
		return i
	}
	return dv
}

func (b *sanitizer) SimpleAtob(k K, dv bool) bool {
	v := b.Getenv(string(k))
	return strengthen.SimpleAtob(v, dv)
}

func (b *sanitizer) Duration(k K, dv time.Duration) time.Duration {
	v := b.Getenv(string(k))
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return dv
}

func (b *sanitizer) Strings(k K) []string {
	s := b.Getenv(string(k))
	return strings.Split(s, StandardSeparator)
}

var (
	SystemBroker Broker = &broker{}
)

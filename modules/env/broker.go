package env

import (
	"os"
	"sync"
	"syscall"
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
	env     map[string]int
	envs    []string
	envLock sync.RWMutex
}

func NewSanitizer() Broker {
	sa := &sanitizer{
		env:  make(map[string]int),
		envs: make([]string, len(allowedEnv)),
	}
	for i, k := range allowedEnv {
		sa.env[k] = i
		if v, ok := LookupEnv(k); ok {
			sa.envs[i] = k + "=" + v
			continue
		}
		sa.envs[i] = ""
	}
	return sa
}

func (sa *sanitizer) ExpandEnv(s string) string {
	return os.Expand(s, sa.Getenv)
}

func (sa *sanitizer) LookupEnv(key string) (string, bool) {
	if len(key) == 0 {
		return "", false
	}
	sa.envLock.RLock()
	defer sa.envLock.RUnlock()
	i, ok := sa.env[key]
	if !ok {
		return "", false
	}
	s := sa.envs[i]
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[i+1:], true
		}
	}
	return "", false
}

func (sa *sanitizer) Getenv(key string) string {
	v, _ := sa.LookupEnv(key)
	return v
}

func (sa *sanitizer) Setenv(key, value string) error {
	if len(key) == 0 {
		return syscall.EINVAL
	}
	for i := 0; i < len(key); i++ {
		if key[i] == '=' || key[i] == 0 {
			return syscall.EINVAL
		}
	}
	kv := key + "=" + value
	sa.envLock.Lock()
	defer sa.envLock.Unlock()
	i, ok := sa.env[key]
	if ok {
		sa.envs[i] = kv
	} else {
		i = len(sa.envs)
		sa.envs = append(sa.envs, kv)
	}
	sa.env[key] = i
	return nil
}

func (sa *sanitizer) Unsetenv(key string) error {
	sa.envLock.Lock()
	defer sa.envLock.Unlock()

	if i, ok := sa.env[key]; ok {
		sa.envs[i] = ""
		delete(sa.env, key)
	}
	return nil
}

func (sa *sanitizer) Clearenv() {
	sa.envLock.Lock()
	defer sa.envLock.Unlock()
	sa.env = make(map[string]int)
	sa.envs = []string{}
}

func (sa *sanitizer) Environ() []string {
	sa.envLock.RLock()
	defer sa.envLock.RUnlock()
	a := make([]string, 0, len(sa.envs)+16) // Reduce the number of memory allocations
	for _, env := range sa.envs {
		if env != "" {
			a = append(a, env)
		}
	}
	return a
}

func (sa *sanitizer) copySanitizer() *sanitizer {
	sa.envLock.RLock()
	defer sa.envLock.RUnlock()
	nsa := &sanitizer{
		env:  make(map[string]int),
		envs: make([]string, len(sa.envs)),
	}
	for k, i := range sa.env {
		nsa.env[k] = i
		nsa.envs[i] = sa.envs[i]
	}
	return nsa
}

var (
	SystemBroker Broker = &broker{}
)

func DeriveSanitizer(b Broker) Broker {
	if b != nil {
		if sa, ok := b.(*sanitizer); ok {
			return sa.copySanitizer()
		}
	}
	return NewSanitizer()
}

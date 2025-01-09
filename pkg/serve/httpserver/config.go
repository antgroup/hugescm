// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/version"
)

const (
	DefaultReadTimeout  = 2 * time.Hour
	DefaultWriteTimeout = 2 * time.Hour
	DefaultIdleTimeout  = 5 * time.Minute
)

type ServerConfig struct {
	Listen        string          `toml:"listen"`
	Repositories  string          `toml:"repositories"`
	IdleTimeout   serve.Duration  `toml:"idle_timeout,omitempty"`
	ReadTimeout   serve.Duration  `toml:"read_timeout,omitempty"`
	WriteTimeout  serve.Duration  `toml:"write_timeout,omitempty"`
	BannerVersion string          `toml:"banner_version,omitempty"`
	DecryptedKey  string          `toml:"decrypted_key,omitempty"`
	Cache         *serve.Cache    `toml:"cache,omitempty"`
	DB            *serve.Database `toml:"database,omitempty"`
	PersistentOSS *serve.OSS      `toml:"oss,omitempty"` // Persistent storage
}

func NewServerConfig(file string, expandEnv bool) (*ServerConfig, error) {
	r, err := serve.NewExpandReader(file, expandEnv)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	sc := &ServerConfig{
		Listen: "127.0.0.1:21000",
		IdleTimeout: serve.Duration{
			Duration: DefaultIdleTimeout,
		},
		ReadTimeout: serve.Duration{
			Duration: DefaultReadTimeout,
		},
		WriteTimeout: serve.Duration{
			Duration: DefaultWriteTimeout,
		},
		BannerVersion: version.GetServerVersion(),
	}
	if _, err = toml.NewDecoder(r).Decode(sc); err != nil {
		return nil, err
	}
	sc.DB.Decrypt(sc.DecryptedKey)
	sc.PersistentOSS.Decrypt(sc.DecryptedKey)
	if sc.Cache == nil {
		sc.Cache = &serve.Cache{
			NumCounters: 1000000000,
			MaxCost:     20,
			BufferItems: 64,
		}
	}
	return sc, nil
}

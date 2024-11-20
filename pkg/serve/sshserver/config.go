// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/version"
)

const (
	DefaultMaxTimeout  = 2 * time.Hour
	DefaultIdleTimeout = 5 * time.Minute
)

type ServerConfig struct {
	Listen          string          `toml:"listen"`
	Repositories    string          `toml:"repositories"`
	Endpoint        string          `toml:"endpoint"`
	MaxTimeout      serve.Duration  `toml:"max_timeout,omitempty"`
	IdleTimeout     serve.Duration  `toml:"idle_timeout,omitempty"`
	BannerVersion   string          `toml:"banner_version,omitempty"`
	HostPrivateKeys []string        `toml:"host_private_keys"` // private keys
	DecryptedKey    string          `toml:"decrypted_key,omitempty"`
	Cache           *serve.Cache    `toml:"cache,omitempty"`
	DB              *serve.Database `toml:"database,omitempty"`
	ZetaOSS         *serve.OSS      `toml:"oss,omitempty"`
}

func NewServerConfig(file string, expandEnv bool) (*ServerConfig, error) {
	r, err := serve.NewExpandReader(file, expandEnv)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	sc := &ServerConfig{
		Listen: "127.0.0.1:22000",
		IdleTimeout: serve.Duration{
			Duration: DefaultIdleTimeout,
		},
		MaxTimeout: serve.Duration{
			Duration: DefaultMaxTimeout,
		},
		BannerVersion: "ZetaServe-" + version.GetVersion(),
	}
	if _, err = toml.NewDecoder(r).Decode(sc); err != nil {
		return nil, err
	}
	sc.DB.Decrypt(sc.DecryptedKey)
	sc.ZetaOSS.Decrypt(sc.DecryptedKey)
	if sc.Cache == nil {
		sc.Cache = &serve.Cache{
			NumCounters: 1000000000,
			MaxCost:     20,
			BufferItems: 64,
		}
	}
	if len(sc.Endpoint) == 0 {
		sc.Endpoint = "zeta.io"
	}
	return sc, nil
}

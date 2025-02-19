// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serve

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/go-sql-driver/mysql"
)

const (
	maxAllowedPacket = 16777216 // OB
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Database struct {
	Name    string   `toml:"name"`
	User    string   `toml:"user"`
	Host    string   `toml:"host"`
	Port    int      `toml:"port"`
	Passwd  string   `toml:"passwd"`
	Timeout Duration `toml:"timeout,omitempty"`
}

func (d *Database) Decrypt(decryptedKey string) {
	if d == nil || len(decryptedKey) == 0 {
		return
	}
	if passwd, err := Decrypt(d.Passwd, decryptedKey); err == nil {
		d.Passwd = passwd
	}
}

func (d *Database) MakeConfig() (*mysql.Config, error) {
	if d.Timeout.Duration == 0 {
		d.Timeout.Duration = 30 * time.Second
	}

	cfg := mysql.NewConfig()
	cfg.User = d.User
	cfg.Passwd = d.Passwd
	cfg.DBName = d.Name
	cfg.Net = "tcp"
	cfg.Addr = d.Host + ":" + strconv.Itoa(d.Port)
	cfg.Timeout = d.Timeout.Duration
	cfg.ReadTimeout = d.Timeout.Duration
	cfg.WriteTimeout = d.Timeout.Duration
	cfg.ParseTime = true
	cfg.InterpolateParams = true
	// http://iokde.com/post/go-mysql-max_allowed_packet.html
	// https://wangming1993.github.io/2019/02/25/go-mysql-disable-max-allowed-packet/
	cfg.MaxAllowedPacket = maxAllowedPacket

	return cfg, nil
}

type OSS struct {
	Endpoint        string `toml:"endpoint,omitempty"`
	SharedEndpoint  string `toml:"shared_endpoint,omitempty"`
	Bucket          string `toml:"bucket"`
	AccessKeyID     string `toml:"access_key_id"`
	AccessKeySecret string `toml:"access_key_secret"`
	Product         string `toml:"product,omitempty"`
	Region          string `toml:"region,omitempty"`
}

func (o *OSS) Decrypt(decryptedKey string) {
	if o == nil || len(decryptedKey) == 0 {
		return
	}
	if accessKeyID, err := Decrypt(o.AccessKeyID, decryptedKey); err == nil {
		o.AccessKeyID = accessKeyID
	}
	if accessKeySecret, err := Decrypt(o.AccessKeySecret, decryptedKey); err == nil {
		o.AccessKeySecret = accessKeySecret
	}
}

type Cache struct {
	NumCounters int64 `toml:"num_counters"`
	MaxCost     int64 `toml:"max_cost"`
	BufferItems int64 `toml:"buffer_items"`
}

const (
	MiByte = 1 << 20
)

func NewExpandReader(file string, expandEnv bool) (io.ReadCloser, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	if !expandEnv {
		return fd, err
	}
	defer fd.Close()
	buf, err := streamio.GrowReadMax(fd, 64*MiByte, 4096)
	if err != nil {
		return nil, err
	}
	b := strings.NewReader(os.ExpandEnv(string(buf)))
	return io.NopCloser(b), nil
}

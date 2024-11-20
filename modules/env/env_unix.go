//go:build !windows

package env

func InitializeEnv() error {
	return nil
}

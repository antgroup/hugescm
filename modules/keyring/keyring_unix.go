//go:build (dragonfly && cgo) || (freebsd && cgo) || linux || netbsd || openbsd

package keyring

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	ss "github.com/antgroup/hugescm/modules/keyring/secret_service"
	dbus "github.com/godbus/dbus/v5"
)

type secretServiceProvider struct{}

// Set stores user and pass in the keyring under the defined service
// name.
func (s secretServiceProvider) Set(ctx context.Context, service, user, pass string) error {
	svc, err := ss.NewSecretService()
	if err != nil {
		return err
	}

	// open a session
	session, err := svc.OpenSession()
	if err != nil {
		return err
	}
	defer svc.Close(session)

	attributes := map[string]string{
		"username": user,
		"service":  service,
	}

	secret := ss.NewSecret(session.Path(), pass)

	collection := svc.GetLoginCollection()

	err = svc.Unlock(collection.Path())
	if err != nil {
		return err
	}

	err = svc.CreateItem(collection,
		fmt.Sprintf("Password for '%s' on '%s'", user, service),
		attributes, secret)
	if err != nil {
		return err
	}

	return nil
}

// findItem lookup an item by service and user.
func (s secretServiceProvider) findItem(svc *ss.SecretService, service, user string) (dbus.ObjectPath, error) {
	collection := svc.GetLoginCollection()

	search := map[string]string{
		"username": user,
		"service":  service,
	}

	err := svc.Unlock(collection.Path())
	if err != nil {
		return "", err
	}

	results, err := svc.SearchItems(collection, search)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", ErrNotFound
	}

	return results[0], nil
}

// Get gets a secret from the keyring given a service name and a user.
func (s secretServiceProvider) Get(ctx context.Context, service, user string) (string, error) {
	svc, err := ss.NewSecretService()
	if err != nil {
		return "", err
	}

	item, err := s.findItem(svc, service, user)
	if err != nil {
		return "", err
	}

	// open a session
	session, err := svc.OpenSession()
	if err != nil {
		return "", err
	}
	defer svc.Close(session)

	// unlock if individual item is locked
	err = svc.Unlock(item)
	if err != nil {
		return "", err
	}

	secret, err := svc.GetSecret(item, session.Path())
	if err != nil {
		return "", err
	}

	return string(secret.Value), nil
}

// Delete deletes a secret, identified by service & user, from the keyring.
func (s secretServiceProvider) Delete(ctx context.Context, service, user string) error {
	svc, err := ss.NewSecretService()
	if err != nil {
		return err
	}

	item, err := s.findItem(svc, service, user)
	if err != nil {
		return err
	}

	return svc.Delete(item)
}

const (
	ZetaUserName = "ZetaUserName"
)

func unixTargetName(targetName string) string {
	return "zeta:" + targetName
}

func (s secretServiceProvider) Find(ctx context.Context, targetName string) (*Cred, error) {
	pass, err := s.Get(ctx, unixTargetName(targetName), ZetaUserName)
	if err != nil {
		return nil, err
	}
	body, err := base64.StdEncoding.DecodeString(pass)
	if err != nil {
		return nil, err
	}
	userName, password, ok := strings.Cut(string(body), "\x00")
	if !ok {
		return nil, errors.New("bad store data")
	}
	return &Cred{UserName: userName, Password: password}, nil
}

func (s secretServiceProvider) Store(ctx context.Context, targetName string, c *Cred) error {
	if strings.Contains(c.UserName, "\x00") {
		return errors.New("invalid username")
	}
	body := fmt.Sprintf("%s\x00%s", c.UserName, c.Password)
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	return s.Set(ctx, unixTargetName(targetName), ZetaUserName, encoded)
}

func (s secretServiceProvider) Discard(ctx context.Context, targetName string) error {
	return s.Delete(ctx, unixTargetName(targetName), ZetaUserName)
}

func init() {
	provider = secretServiceProvider{}
}

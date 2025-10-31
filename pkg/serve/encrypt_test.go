package serve

import (
	"crypto/ecdh"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/base58"
)

func TestECIS(t *testing.T) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GenKey error: %v\n", err)
		return
	}
	d := &Decrypter{
		PrivateKey: privateKey,
	}
	sss, err := d.encrypt([]byte("1234567890"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "E error: %v\n", err)
		return
	}
	text := base58.Encode(sss)
	fmt.Fprintf(os.Stderr, "ECIS@%s\n", text)
	raw, err := d.decrypt(base58.Decode(text))
	if err != nil {
		fmt.Fprintf(os.Stderr, "D error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", raw)
}

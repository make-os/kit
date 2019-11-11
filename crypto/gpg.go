package crypto

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

// GPGEntityFromPubKey returns an entity for the given public key
// pubKey: A valid GPG public key
func GPGEntityFromPubKey(pubKey string) (*openpgp.Entity, error) {
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(pubKey))
	if err != nil {
		return nil, err
	}
	return entities[0], nil
}

// GetGPGPublicKey finds the GPG public key on the machine
// keyID: The id of the key
// gpgProgram: The path to the gpg executeable
func GetGPGPublicKey(keyID string, gpgProgram string) (*openpgp.Entity, error) {

	// Run the command to fetch the public key
	cmd := exec.Command(gpgProgram, "--export", "-a", keyID)
	bz, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, fmt.
			Sprintf("failed to get public key (target id: %s)", keyID))
	}

	// If no output, then the public key does not exist
	if len(bz) == 0 {
		return nil, fmt.Errorf("gpg public key not found")
	}

	// Read the public key into an entity
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(bz))
	if err != nil {
		return nil, err
	}

	return entities[0], nil
}

// VerifyGPGSignature verifies a signature using the given public key entity
// pubKeyEntity: The public key as an entity
// sig: The signature to verify
// msg: The message that was signed
func VerifyGPGSignature(pubKeyEntity *openpgp.Entity, sig []byte, msg []byte) (bool, error) {

	signatureReader := bytes.NewReader(sig)
	block, err := armor.Decode(signatureReader)
	if err != nil {
		return false, errors.Wrap(err, "failed to decode signature")
	} else if block.Type != openpgp.SignatureType {
		return false, errors.New("invalid signature format")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	if err != nil {
		return false, errors.Wrap(err, "failed to read signature body")
	}

	packSig := pkt.(*packet.Signature)
	hash := packSig.Hash.New()
	messageReader := bytes.NewReader(msg)
	io.Copy(hash, messageReader)

	err = pubKeyEntity.PrimaryKey.VerifySignature(hash, packSig)
	if err != nil {
		return false, errors.Wrap(err, "failed to verify signature")
	}

	return true, nil
}

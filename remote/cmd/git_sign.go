package cmd

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/server"
)

// GitSignCmd mocks git signing interface allowing this program to
// be used by git for signing a commit or tag.
func GitSignCmd(cfg *config.AppConfig, data io.Reader) {

	log := cfg.G().Log

	// Get the data to be signed and the key id to use
	pushKeyID := os.Args[3]

	// Get the target repo
	repoDir, _ := os.Getwd()
	targetRepo, err := repo.GetRepo(repoDir)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "failed to get repo").Error())
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, pushKeyID, "", targetRepo)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get the push request token
	token := os.Getenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName))
	if token == "" {
		log.Fatal(errors.Wrap(err, "push request token not set").Error())
	}

	// Decode the push request token
	txDetail, err := server.DecodePushToken(token)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to decode token").Error())
	}

	// Construct the message
	// git sig message + msgpack(tx parameters)
	msg, _ := ioutil.ReadAll(data)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Sign the message
	sig, err := key.GetKey().PrivKey().Sign(msg)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to sign").Error())
	}

	// Write output
	w := bytes.NewBuffer(nil)
	pem.Encode(w, &pem.Block{Bytes: sig, Type: "PGP SIGNATURE", Headers: txDetail.ToMapForPEMHeader()})
	fmt.Fprintf(os.Stderr, "[GNUPG:] BEGIN_SIGNING\n")
	fmt.Fprintf(os.Stderr, "[GNUPG:] SIG_CREATED C\n")
	fmt.Fprintf(os.Stdout, "%s", w.Bytes())

	os.Exit(0)
}

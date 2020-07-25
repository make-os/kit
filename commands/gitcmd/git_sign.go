package gitcmd

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"gitlab.com/makeos/lobe/commands/common"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/remote/server"
	"gitlab.com/makeos/lobe/remote/types"
)

type GitSignArgs struct {
	// Args is the program arguments
	Args []string

	// RepoGetter is the function for getting a local repository
	RepoGetter func(path string) (types.LocalRepo, error)

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	PushKeyUnlocker common.KeyUnlocker

	// StdOut is the standard output
	StdOut io.Writer

	// StdErr is the standard error output
	StdErr io.Writer
}

// GitSignCmd mocks git signing interface allowing this program to
// be used by git for signing a commit or tag.
func GitSignCmd(cfg *config.AppConfig, data io.Reader, args *GitSignArgs) error {

	// Get the data to be signed and the key id to use
	pushKeyID := args.Args[3]

	// Get the target repo
	repoDir, _ := os.Getwd()
	targetRepo, err := args.RepoGetter(repoDir)
	if err != nil {
		return errors.Wrapf(err, "failed to get repo")
	}

	// Get and unlock the pusher key
	key, err := args.PushKeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: pushKeyID,
		Passphrase:   "",
		AskPass:      false,
		TargetRepo:   targetRepo,
	})
	if err != nil {
		return errors.Wrap(err, "failed to get push key")
	}

	// Get the push request token
	token := os.Getenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName))
	if token == "" {
		return fmt.Errorf("push request token not set")
	}

	// Decode the push request token
	txDetail, err := server.DecodePushToken(token)
	if err != nil {
		return errors.Wrap(err, "failed to decode token")
	}

	// Construct the message
	// git sig message + msgpack(tx parameters)
	msg, _ := ioutil.ReadAll(data)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Sign the message
	sig, err := key.GetKey().PrivKey().Sign(msg)
	if err != nil {
		return errors.Wrap(err, "failed to sign")
	}

	// Write output
	w := bytes.NewBuffer(nil)
	pem.Encode(w, &pem.Block{Bytes: sig, Type: "PGP SIGNATURE", Headers: txDetail.GetPEMHeader()})
	fmt.Fprintf(args.StdErr, "[GNUPG:] BEGIN_SIGNING\n")
	fmt.Fprintf(args.StdErr, "[GNUPG:] SIG_CREATED C\n")
	fmt.Fprintf(args.StdOut, "%s", w.Bytes())

	return nil
}

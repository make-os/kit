package gitcmd

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/remote/types"
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
	repo, err := args.RepoGetter(repoDir)
	if err != nil {
		return errors.Wrapf(err, "failed to get repo")
	}

	// Get and unlock the pusher key
	key, err := args.PushKeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: pushKeyID,
		Passphrase: "",
		NoPrompt:   false,
		TargetRepo: repo,
	})
	if err != nil {
		return errors.Wrap(err, "failed to get push key")
	}

	// Construct the message
	// git sig message
	msg, _ := ioutil.ReadAll(data)

	// Sign the message
	sig, err := key.GetKey().PrivKey().Sign(msg)
	if err != nil {
		return errors.Wrap(err, "failed to sign")
	}

	// Write output
	w := bytes.NewBuffer(nil)
	txDetail := &types.TxDetail{PushKeyID: key.GetPushKeyAddress()}
	pem.Encode(w, &pem.Block{Bytes: sig, Type: "PGP SIGNATURE", Headers: txDetail.GetGitSigPEMHeader()})
	fmt.Fprintf(args.StdErr, "[GNUPG:] BEGIN_SIGNING\n")
	fmt.Fprintf(args.StdErr, "[GNUPG:] SIG_CREATED C\n")
	fmt.Fprintf(args.StdOut, "%s", w.Bytes())

	return nil
}

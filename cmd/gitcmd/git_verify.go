package gitcmd

import (
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/types"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
)

type GitVerifyArgs struct {
	// Args is the program arguments
	Args []string

	// RepoGetter is the function for getting a local repository
	RepoGetter func(path string) (types.LocalRepo, error)

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	PushKeyUnlocker common.KeyUnlocker

	// PemDecoder is a function for decoding PEM data
	PemDecoder func(data []byte) (p *pem.Block, rest []byte)

	// StdOut is the standard output
	StdOut io.Writer

	// StdErr is the standard error output
	StdErr io.Writer

	// StdIn is the standard input
	StdIn io.ReadCloser
}

// GitVerifyCmd mocks git signing interface allowing this program to
// be used by git for verifying signatures.
func GitVerifyCmd(cfg *config.AppConfig, args *GitVerifyArgs) error {

	// Read the signature
	sig, err := ioutil.ReadFile(args.Args[4])
	if err != nil {
		err = errors.Wrap(err, "failed to read sig file")
		fmt.Fprintf(args.StdErr, err.Error())
		return err
	}

	// Attempt to decode from PEM
	decSig, _ := args.PemDecoder(sig)
	if decSig == nil {
		err = fmt.Errorf("malformed signature. Expected PEM encoded signature")
		fmt.Fprintf(args.StdErr, err.Error())
		return err
	}

	// Get tx parameters from the header
	txDetail, err := types.TxDetailFromGitSigPEMHeader(decSig.Headers)
	if err != nil {
		fmt.Fprintf(args.StdOut, "[GNUPG:] BADSIG 0\n")
		err = fmt.Errorf("invalid header: %s", err)
		fmt.Fprintf(args.StdErr, err.Error()+"\n")
		return err
	}

	// Ensure push key is set
	if txDetail.PushKeyID == "" {
		fmt.Fprintf(args.StdOut, "[GNUPG:] BADSIG 0\n")
		err = fmt.Errorf("invalid header: 'pkID' is required")
		fmt.Fprintf(args.StdErr, err.Error()+"\n")
		return err
	}

	// Get the target repo
	repoDir, _ := os.Getwd()
	targetRepo, err := args.RepoGetter(repoDir)
	if err != nil {
		err = fmt.Errorf("failed to get repo: %s", err.Error())
		fmt.Fprintf(args.StdErr, err.Error()+"\n")
		return err
	}

	// Get and unlock the pusher key
	key, err := args.PushKeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: txDetail.PushKeyID,
		Passphrase: "",
		NoPrompt:   true,
		TargetRepo: targetRepo,
	})
	if err != nil {
		fmt.Fprintf(args.StdErr, err.Error()+"\n")
		return err
	}

	// Construct the signature message
	msg, _ := ioutil.ReadAll(args.StdIn)

	// Verify the signature
	if ok, err := key.GetKey().PubKey().Verify(msg, decSig.Bytes); err != nil || !ok {
		err = fmt.Errorf("signature is not valid")
		fmt.Fprintf(args.StdErr, err.Error()+"\n")
		return err
	}

	// Write output
	cg := fmt2.GreenString
	fmt.Fprintf(args.StdOut, "[GNUPG:] NEWSIG\n")
	fmt.Fprintf(args.StdOut, "[GNUPG:] GOODSIG 0\n")
	fmt.Fprintf(args.StdOut, "[GNUPG:] TRUST_FULLY 0 shell\n")
	fmt.Fprintf(args.StdErr, "%s\n", cg("sig: signature is ok"))
	fmt.Fprintf(args.StdErr, "%s\n", cg("sig: signed by %s", key.GetPushKeyAddress()))

	return nil
}

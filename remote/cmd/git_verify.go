package cmd

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types"
)

// GitVerifyCmd mocks git signing interface allowing this program to
// be used by git for verifying signatures.
func GitVerifyCmd(cfg *config.AppConfig, args []string) {

	fp := fmt.Fprintf

	// Read the signature
	sig, err := ioutil.ReadFile(args[4])
	if err != nil {
		fp(os.Stderr, "failed to read sig file: %s", err)
		os.Exit(1)
	}

	// Attempt to decode from PEM
	decSig, _ := pem.Decode(sig)
	if decSig == nil {
		fp(os.Stderr, "malformed signatured. Expected pem encoded signature.", err)
		os.Exit(1)
	}

	// Get tx parameters from the header
	txDetail, err := types.TxDetailFromPEMHeader(decSig.Headers)
	if err != nil {
		fp(os.Stdout, "[GNUPG:] BADSIG 0\n")
		fp(os.Stderr, "invalid header: %s\n", err)
		os.Exit(1)
	}

	// Ensure push key is set
	if txDetail.PushKeyID == "" {
		fp(os.Stdout, "[GNUPG:] BADSIG 0\n")
		fp(os.Stderr, "invalid header: 'pkID' is required\n")
		os.Exit(1)
	}

	// Get the target repo
	repoDir, _ := os.Getwd()
	targetRepo, err := repo.GetRepo(repoDir)
	if err != nil {
		fp(os.Stderr, errors.Wrapf(err, "failed to get repo").Error()+"\n")
		os.Exit(1)
	}

	// Get and unlock the pusher key
	key, err := getAndUnlockPushKey(cfg, txDetail.PushKeyID, "", targetRepo)
	if err != nil {
		fp(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}

	// Construct the signature message
	msg, _ := ioutil.ReadAll(os.Stdin)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Verify the signature
	if ok, err := key.GetKey().PubKey().Verify(msg, decSig.Bytes); err != nil || !ok {
		fp(os.Stderr, "signature is not valid\n")
		os.Exit(1)
	}

	// Write output
	cg := color.GreenString
	fp(os.Stdout, "[GNUPG:] NEWSIG\n")
	fp(os.Stdout, "[GNUPG:] GOODSIG 0\n")
	fp(os.Stdout, "[GNUPG:] TRUST_FULLY 0 shell\n")
	fp(os.Stderr, "%s\n", cg("sig: signature is ok"))
	fp(os.Stderr, "%s\n", cg("sig: signed by %s (nonce: %d)", txDetail.PushKeyID, txDetail.Nonce))
	fp(os.Stderr, "%s\n", cg("sig: fee: %s", txDetail.Fee))
	if txDetail.MergeProposalID != "" {
		fp(os.Stderr, "%s\n", cg("sig: fulfilling merge proposal: %s", txDetail.MergeProposalID))
	}

	os.Exit(0)
}

package repocmd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/types"
	rpctypes "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/api"
	"github.com/spf13/cast"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
)

type Remote struct {
	Name string
	URL  string
}

// ConfigArgs contains arguments for ConfigCmd.
type ConfigArgs struct {

	// Value is the transaction value
	Value *float64

	// Fee is the transaction fee to be paid by the signing key
	Fee *float64

	// Nonce is the next nonce of the signing account
	Nonce *uint64

	// CommitAmend amends and sign the last commit instead of creating new one.
	AmendCommit *bool

	// SigningKey is the key that will be used to sign the transaction.
	SigningKey *string

	// SigningKeyPass is the passphrase for unlocking the signing key.
	SigningKeyPass *string

	// PushKey is the key that will be used sign push request
	PushKey *string

	// NoHook indicates that hooks should not be added
	NoHook bool

	// Remotes contain a list of git remotes to append to the repo.
	Remotes []Remote

	// RpcClient is the RPC client
	RPCClient rpctypes.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce api.NextNonceGetter

	// CreateRepo is a function for generating a transaction for creating a repository
	CreateRepo api.RepoCreator

	Stdout io.Writer
}

// ConfigCmd configures a repository
func ConfigCmd(cfg *config.AppConfig, repo types.LocalRepo, args *ConfigArgs) error {

	rcfg, err := repo.Config()
	if err != nil {
		return err
	}

	// Set fee at `user.fee`
	if args.Fee != nil {
		rcfg.Raw.Section("user").SetOption("fee", cast.ToString(*args.Fee))
	} else {
		rcfg.Raw.Section("user").SetOption("fee", "0")
	}

	// Set value at `user.value`
	if args.Value != nil {
		rcfg.Raw.Section("user").SetOption("value", cast.ToString(*args.Value))
	}

	// Set nonce at `user.nonce`
	if args.Nonce != nil {
		rcfg.Raw.Section("user").SetOption("nonce", cast.ToString(*args.Nonce))
	} else {
		rcfg.Raw.Section("user").SetOption("nonce", "0")
	}

	// Set signing key at `user.signingKey` only if args.PushKey is unset, otherwise use args.PushKey.
	if args.SigningKey != nil && (args.PushKey == nil || *args.PushKey == "") {
		rcfg.Raw.Section("user").SetOption("signingKey", *args.SigningKey)
	} else if args.PushKey != nil && *args.PushKey != "" {
		rcfg.Raw.Section("user").SetOption("signingKey", *args.PushKey)
	}

	// Set signing key passphrase at `user.passphrase`
	if args.SigningKeyPass != nil {
		rcfg.Raw.Section("user").SetOption("passphrase", *args.SigningKeyPass)
	}

	// Set commit amendment `commit.amend`
	if args.AmendCommit != nil {
		rcfg.Raw.Section("commit").SetOption("amend", cast.ToString(*args.AmendCommit))
	}

	// Lookup full path of app binary
	appBinPath, err := exec.LookPath(config.AppName)
	if err != nil {
		return fmt.Errorf("could not find '%s' executable in PATH", config.AppName)
	}

	// Set kit as `gpg.program`
	rcfg.Raw.Section("gpg").SetOption("program", appBinPath)

	// Add user-defined remotes
	for _, remote := range args.Remotes {
		rcfg.Remotes[remote.Name] = &gogitcfg.RemoteConfig{Name: remote.Name, URLs: strings.Split(remote.URL, ",")}
	}

	// If no remote was set, add default remote pointing to the local remote.
	if len(rcfg.Remotes) == 0 {
		defaultAddr := config.DefaultRemoteServerAddress
		if defaultAddr[:1] == ":" {
			defaultAddr = "127.0.0.1" + defaultAddr
		}
		url := fmt.Sprintf("http://%s/r/%s", defaultAddr, repo.GetName())
		rcfg.Remotes["origin"] = &gogitcfg.RemoteConfig{Name: "origin", URLs: []string{url}}
	}

	// Add hooks if allowed
	dotGitPath := filepath.Join(repo.GetPath(), ".git")
	if !args.NoHook {
		if err = addHooks(appBinPath, dotGitPath); err != nil {
			return err
		}
	}

	// Set credential helper
	rcfg.Raw.Section("credential").
		SetOption("helper", ""). // Used to clear other helpers from system/global config
		AddOption("helper", "store --file .git/.git-credentials")

	// Set the config
	if err = repo.SetConfig(rcfg); err != nil {
		return err
	}

	return nil
}

// addHook adds hooks to git repo at the given path.
// If not already added the hook command already exist, it will not be re-added.
// If the hook file does not exist, create it and make it an executable on non-windows system.
func addHooks(appAppName string, path string) error {
	for _, hook := range []string{"pre-push"} {
		cmd := fmt.Sprintf("%s repo hook $1", appAppName)

		os.Mkdir(filepath.Join(path, "hooks"), 0700)
		prePushFile := filepath.Join(path, "hooks", hook)
		if !util.IsFileOk(prePushFile) {
			err := ioutil.WriteFile(prePushFile, []byte(fmt.Sprintf("#!/bin/sh\n%s", cmd)), 0644)
			if err != nil {
				return err
			}
			if runtime.GOOS == "windows" {
				continue
			}
			err = exec.Command("bash", "-c", "chmod +x "+prePushFile).Run()
			if err != nil {
				return err
			}
			continue
		}

		f, err := os.OpenFile(prePushFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		// Check if the hook command already exist in the file.
		// If it does, do not append again.
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			// ignore comment lines
			if line := scanner.Text(); line != "" && line[:1] == "#" {
				continue
			}
			if strings.Contains(scanner.Text(), appAppName+" repo hook") {
				goto end
			}
			if strings.Contains(scanner.Text(), config.AppName+" repo hook") {
				goto end
			}
		}
		if scanner.Err() != nil {
			return err
		}
		_, err = f.WriteString("\n" + cmd)
		if err != nil {
			return err
		}
	end:
	}

	return nil
}

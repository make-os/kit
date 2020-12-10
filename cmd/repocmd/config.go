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
	"time"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/cmd/passcmd/agent"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/types"
	rpctypes "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/api"
	"github.com/pkg/errors"
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
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce api.NextNonceGetter

	// CommandCreator creates a wrapped exec.Cmd object
	CommandCreator util.CommandCreator

	// PassAgentPort determines the port where the passphrase agent will bind to
	PassAgentPort *string

	// PassCacheTTL is the number of seconds a cached passphrase will live for.
	PassCacheTTL string

	// PassAgentSet is a function for sending set request to the agent service
	PassAgentSet agent.SetFunc

	// PassAgentUp is a function that checks if the agent server is running
	PassAgentUp agent.IsUpFunc

	Stderr io.Writer
	Stdout io.Writer
}

// ConfigCmd configures a repository
func ConfigCmd(_ *config.AppConfig, repo types.LocalRepo, args *ConfigArgs) error {

	gitCfg, err := repo.Config()
	if err != nil {
		return err
	}

	// If signing key was not provided, get it from the environment
	if args.SigningKeyPass == nil {
		envPass := os.Getenv(common.MakePassEnvVar(config.AppName))
		args.SigningKeyPass = &envPass
	}

	// Set fee at `user.fee`
	if args.Fee != nil {
		gitCfg.Raw.Section("user").SetOption("fee", cast.ToString(*args.Fee))
	} else {
		gitCfg.Raw.Section("user").SetOption("fee", "0")
	}

	// Set value at `user.value`
	if args.Value != nil {
		gitCfg.Raw.Section("user").SetOption("value", cast.ToString(*args.Value))
	}

	// Set nonce at `user.nonce`
	if args.Nonce != nil {
		gitCfg.Raw.Section("user").SetOption("nonce", cast.ToString(*args.Nonce))
	} else {
		gitCfg.Raw.Section("user").SetOption("nonce", "0")
	}

	// Set signing key at `user.signingKey` only if args.PushKey is unset, otherwise use args.PushKey.
	if args.SigningKey != nil && (args.PushKey == nil || *args.PushKey == "") {
		gitCfg.Raw.Section("user").SetOption("signingKey", *args.SigningKey)
	} else if args.PushKey != nil && *args.PushKey != "" {
		gitCfg.Raw.Section("user").SetOption("signingKey", *args.PushKey)
	}

	// If signing key is set, start the passphrase agent and cache the passphrase.
	if args.SigningKeyPass != nil && *args.SigningKeyPass != "" {
		if !args.PassAgentUp(*args.PassAgentPort) {
			cmd := args.CommandCreator(config.AppName, "pass", "--start-agent", "--port", *args.PassAgentPort)
			cmd.SetStdout(args.Stdout)
			cmd.SetStderr(args.Stderr)
			if err := cmd.Start(); err != nil {
				return errors.Wrap(err, "failed to start passphrase agent")
			}
			time.Sleep(500 * time.Millisecond) // give the agent time to start
		}

		var ttl int
		if args.PassCacheTTL != "" {
			dur, err := time.ParseDuration(args.PassCacheTTL)
			if err != nil {
				return fmt.Errorf("passphrase cache duration is not valid")
			}
			ttl = int(dur.Seconds())
		}
		if err := args.PassAgentSet(*args.PassAgentPort, repo.GetName(), *args.SigningKeyPass, ttl); err != nil {
			return errors.Wrap(err, "failed to send set request to passphrase agent")
		}
	}

	// Lookup full path of app binary
	appBinPath, err := exec.LookPath(config.AppName)
	if err != nil {
		return fmt.Errorf("could not find '%s' executable in PATH", config.AppName)
	}

	// Add user-defined remotes
	for _, remote := range args.Remotes {
		gitCfg.Remotes[remote.Name] = &gogitcfg.RemoteConfig{Name: remote.Name, URLs: strings.Split(remote.URL, ",")}
	}

	// If no remote was set, add default remote pointing to the local remote.
	if len(gitCfg.Remotes) == 0 {
		defaultAddr := config.DefaultRemoteServerAddress
		if defaultAddr[:1] == ":" {
			defaultAddr = "127.0.0.1" + defaultAddr
		}
		url := fmt.Sprintf("http://%s/r/%s", defaultAddr, repo.GetName())
		gitCfg.Remotes["origin"] = &gogitcfg.RemoteConfig{Name: "origin", URLs: []string{url}}
	}

	// Add hooks if allowed
	dotGitPath := filepath.Join(repo.GetPath(), ".git")
	if !args.NoHook {
		if err = addHooks(appBinPath, dotGitPath); err != nil {
			return err
		}
	}

	// Set the config
	if err = repo.SetConfig(gitCfg); err != nil {
		return err
	}

	return nil
}

// addHook adds hooks to git repo at the given path.
// If the hook's command already exist, it will not be re-added.
// If the hook file does not exist, create it and make it an executable on non-windows system.
func addHooks(appAppName string, path string) error {
	for _, hook := range []string{"pre-push", "post-commit"} {
		var cmd string
		switch hook {
		case "pre-push":
			cmd = fmt.Sprintf("%s repo hook $1", appAppName)
		case "post-commit":
			cmd = fmt.Sprintf("%s repo hook -c", appAppName)
		}

		_ = os.Mkdir(filepath.Join(path, "hooks"), 0700)
		prePushFile := filepath.Join(path, "hooks", hook)
		if !util.IsFileOk(prePushFile) {
			err := ioutil.WriteFile(prePushFile, []byte(fmt.Sprintf("#!/bin/sh\n%s", cmd)), 0644)
			if err != nil {
				return err
			}
			if runtime.GOOS == "windows" {
				continue
			}
			err = exec.Command("sh", "-c", "chmod +x "+prePushFile).Run()
			if err != nil {
				return err
			}
			continue
		}

		f, err := os.OpenFile(prePushFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}

		// Check if the hook command already exist in the file.
		// If it does, do not append again.
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			// ignore comment lines
			if line := scanner.Text(); line != "" && line[:1] == "#" {
				continue
			}
			if strings.Contains(scanner.Text(), "repo hook") {
				_ = f.Close()
				goto end
			}
		}
		if scanner.Err() != nil {
			_ = f.Close()
			return err
		}
		_, err = f.WriteString("\n" + cmd)
		if err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
	end:
	}

	return nil
}

package keystore

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/keystore/types"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/olekukonko/tablewriter"
	"github.com/prometheus/common/log"
)

// List returns the accounts stored on disk.
func (ks *Keystore) List() (accounts []types.StoredKey, err error) {
	files, err := ioutil.ReadDir(ks.dir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		// [0-9]{10}: Is the creation unix timestamp
		// [up]: Indicates 'user' or 'push' key as primary
		// [_unprotected]: indicates encryption with default passphrase.
		m, _ := regexp.Match("^[0-9]{10}_([up])[a-zA-Z0-9]{51}(_unprotected)?$", []byte(f.Name()))
		if !m {
			continue
		}

		parts := strings.Split(f.Name(), "_")
		unixTime, _ := strconv.ParseInt(parts[0], 10, 64)
		timeCreated := time.Unix(unixTime, 0)
		cipher, _ := ioutil.ReadFile(filepath.Join(ks.dir, f.Name()))
		pubKey := parts[1][1:]

		// Decode the public key
		pk, err := ed25519.PubKeyFromBase58(pubKey)
		if err != nil {
			log.Infof("found an invalid key file: %s", f.Name())
			continue
		}

		keyType := types.KeyTypeUser
		if parts[1][:1] == "p" {
			keyType = types.KeyTypePush
		}

		accounts = append(accounts, &StoredKey{
			Type:        keyType,
			UserAddress: pk.Addr().String(),
			PushAddress: pk.PushAddr().String(),
			Cipher:      cipher,
			CreatedAt:   timeCreated,
			Filename:    f.Name(),
			Unprotected: strings.HasSuffix(f.Name(), "_unprotected"),
		})
	}

	return
}

// ListCmd fetches and lists all accounts
func (ks *Keystore) ListCmd(out io.Writer) error {

	accts, err := ks.List()
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{"", "Address", "Date Created", "Tag(s)"})
	table.SetBorder(false)
	table.SetAutoFormatHeaders(false)
	table.SetColumnSeparator("")
	table.SetHeaderLine(false)
	if config.NoColorFormatting {
		table.SetHeaderColor(nil, nil, nil, nil)
	}
	hc := tablewriter.Colors{tablewriter.Normal, tablewriter.FgHiBlackColor}
	table.SetHeaderColor(hc, hc, hc, hc)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for i, a := range accts {
		tagStr := ""
		if a.IsUnprotected() {
			tagStr = fmt2.RedString("unprotected")
		}

		displayAddr := a.GetUserAddress()
		if a.GetType() == types.KeyTypePush {
			displayAddr = a.GetPushKeyAddress()
		}

		table.Append([]string{
			fmt.Sprintf("[%d]", i),
			fmt2.CyanString(displayAddr),
			humanize.Time(a.GetCreatedAt()),
			tagStr,
		})
	}
	table.Render()

	return nil
}

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

	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types/core"
)

// List returns the accounts stored on disk.
func (ks *Keystore) List() (accounts []core.StoredKey, err error) {

	files, err := ioutil.ReadDir(ks.dir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		m, _ := regexp.Match("^[0-9]{10}_[a-zA-Z0-9]{43,}(_unprotected)?$", []byte(f.Name()))
		if !m {
			continue
		}

		nameParts := strings.Split(f.Name(), "_")
		unixTime, _ := strconv.ParseInt(nameParts[0], 10, 64)
		timeCreated := time.Unix(unixTime, 0)
		cipher, _ := ioutil.ReadFile(filepath.Join(ks.dir, f.Name()))
		address := nameParts[1]
		keyType := core.KeyTypeAccount
		if crypto.IsValidPushAddr(address) == nil {
			keyType = core.KeyTypePush
		}

		accounts = append(accounts, &StoredKey{
			Type:        keyType,
			Address:     address,
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
	hc := tablewriter.Colors{tablewriter.Normal, tablewriter.FgHiBlackColor}
	table.SetHeaderColor(hc, hc, hc, hc)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for i, a := range accts {
		tagStr := ""
		if a.IsUnprotected() {
			tagStr = color.RedString("unprotected")
		}
		table.Append([]string{
			fmt.Sprintf("[%d]", i),
			color.CyanString(a.GetAddress()),
			humanize.Time(a.GetCreatedAt()),
			tagStr,
		})
	}
	table.Render()

	return nil
}

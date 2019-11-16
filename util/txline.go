package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
)

// TxLine errors
var (
	ErrTxLineNotFound  = fmt.Errorf("txline was not set")
	ErrTxLineMalformed = fmt.Errorf("txline is malformed")
)

// RemoveTxLine removes all lines beginning with a 'Tx Line' prefix 'tx'.
// NOTE: It is case-sensitive.
func RemoveTxLine(msg string) string {
	lines := strings.Split(msg, "\n")
	newMsg := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "tx:") {
			newMsg += line
			if (i + 1) != len(lines) {
				newMsg += "\n"
			}
		}
	}
	return newMsg
}

// TxLine contains txline data
type TxLine struct {
	Fee      String
	Nonce    uint64
	PubKeyID string
}

// ParseTxLine parses the txline data in the message.
// Returns ErrTxLineNotFound if no txline in the message
// and
func ParseTxLine(msg string) (*TxLine, error) {
	lines := strings.Split(msg, "\n")
	txline := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "tx:") {
			txline = line
		}
	}

	if txline == "" {
		return nil, ErrTxLineNotFound
	}

	kvData := strings.Fields(strings.TrimSpace(txline[3:]))
	var txLine = new(TxLine)
	for _, kv := range kvData {
		kv = strings.TrimRight(strings.TrimSpace(kv), ",")
		kvParts := strings.Split(kv, "=")
		if len(kvParts) != 2 {
			return nil, ErrTxLineMalformed
		}

		if kvParts[0] == "fee" {
			if !govalidator.IsNumeric(kvParts[1]) {
				return nil, fmt.Errorf("field:fee, msg: fee must be numeric")
			}
			txLine.Fee = String(kvParts[1])
		}

		if kvParts[0] == "nonce" {
			nonce, err := strconv.ParseUint(kvParts[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("field:nonce, msg: nonce must be an unsigned integer")
			}
			txLine.Nonce = nonce
		}

		if kvParts[0] == "pkId" {
			if kvParts[1] == "" {
				return nil, fmt.Errorf("field:pkId, msg: public key id is required")
			}
			if len(kvParts[1]) != 42 {
				return nil, fmt.Errorf("field:pkId, msg: public key id is invalid")
			}
			txLine.PubKeyID = kvParts[1]
		}
	}

	return txLine, nil
}

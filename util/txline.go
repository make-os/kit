package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
)

const TxLinePrefix = "tx:"

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

// TxLine represents transaction information usually included in commits, notes
// and tag objects
type TxLine struct {
	Fee       String
	Nonce     uint64
	PubKeyID  string
	Signature string
}

// GetNonceString returns the nonce as a string
func (tl *TxLine) GetNonceString() string {
	return strconv.FormatUint(tl.Nonce, 10)
}

func (tl *TxLine) String() string {
	nonceStr := strconv.FormatUint(tl.Nonce, 10)
	return MakeTxLine(tl.Fee.String(), nonceStr, tl.PubKeyID, []byte(tl.Signature))
}

// MakeTxLine returns a well formatted txline string
func MakeTxLine(txFee, txNonce, pkID string, sig []byte) string {
	str := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", txFee, txNonce, pkID)
	if sig != nil {
		str = str + fmt.Sprintf(" sig=%s", ToHex(sig))
	}
	return str
}

// ParseTxLine parses the txline data in the message.
// Returns ErrTxLineNotFound if no txline in the message
// and
func ParseTxLine(msg string) (*TxLine, error) {
	lines := strings.Split(msg, "\n")
	txline := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, TxLinePrefix) {
			txline = line
		}
	}

	if txline == "" {
		return nil, ErrTxLineNotFound
	}

	kvData := strings.Fields(strings.TrimSpace(txline[3:]))
	sort.Strings(kvData)

	var txLine = new(TxLine)
	for _, kv := range kvData {
		kv = strings.TrimRight(strings.TrimSpace(kv), ",")
		kvParts := strings.Split(kv, "=")
		if len(kvParts) != 2 {
			return nil, ErrTxLineMalformed
		}

		if kvParts[0] == "fee" {
			if !govalidator.IsFloat(kvParts[1]) {
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

		if kvParts[0] == "sig" {
			if kvParts[1] == "" {
				return nil, fmt.Errorf("field:sig, msg: signature value is required")
			}
			if kvParts[1][:2] != "0x" {
				return nil, fmt.Errorf("field:sig, msg: signature format is not valid")
			}
			decSig, err := HexToStr(kvParts[1])
			if err != nil {
				return nil, fmt.Errorf("field:sig, msg: signature format is not valid")
			}
			txLine.Signature = decSig
		}
	}

	return txLine, nil
}

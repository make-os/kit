package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/modules/types"
	"github.com/themakeos/lobe/params"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/util"
)

// DevModule provides access to various development utility functions.
type DevModule struct {
	types.ModuleCommon
	vm *otto.Otto
}

// NewDevModule creates an instance of DevModule
func NewDevModule() *DevModule {
	return &DevModule{}
}

// methods are functions exposed in the special namespace of this module.
func (m *DevModule) methods() []*types.VMMember {
	return []*types.VMMember{
		{Name: "accountKey", Value: m.GetDevUserAccountKey(), Description: "Get the private key of the development user account"},
		{Name: "accountAddress", Value: m.GetDevUserAddress(), Description: "Get the address of the development user account"},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *DevModule) globals() []*types.VMMember {
	return []*types.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *DevModule) ConfigureVM(vm *otto.Otto) prompt.Completer {
	m.vm = vm

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, constants.NamespaceDev, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceDev, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// GetDevUserAccountKey returns the development account key
func (m *DevModule) GetDevUserAccountKey() string {
	box := packr.NewBox("../" + params.EmbeddableDataDir)
	devKey, err := box.FindString("dev_account_key")
	if err != nil {
		panic(errors.Wrap(err, "failed to read dev account key"))
	}
	return devKey
}

// GetDevUserAddress returns the development account address
func (m *DevModule) GetDevUserAddress() string {
	key := m.GetDevUserAccountKey()
	pk, err := crypto.PrivKeyFromBase58(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode dev account key"))
	}
	return crypto.NewKeyFromPrivKey(pk).Addr().String()
}

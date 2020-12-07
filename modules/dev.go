package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/data"
	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
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
	return data.DevAccountKey
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

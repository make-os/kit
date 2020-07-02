package modules

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/c-bata/go-prompt"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	"github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

// UtilModule provides access to various utility functions
type UtilModule struct {
	vm          *otto.Otto
	suggestions []prompt.Suggest
}

// NewUtilModule creates an instance of UtilModule
func NewUtilModule() *UtilModule {
	return &UtilModule{}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *UtilModule) ConsoleOnlyMode() bool {
	return true
}

// globals are functions exposed in the VM's global namespace
func (m *UtilModule) globals() []*types.ModuleFunc {
	return []*types.ModuleFunc{
		{Name: "pp", Value: m.prettyPrint, Description: "Pretty print an object"},
		{Name: "eval", Value: m.eval, Description: "Execute javascript code represented as a string"},
		{Name: "evalFile", Value: m.evalFile, Description: "Execute javascript code stored in a file"},
		{Name: "readFile", Value: m.readFile, Description: "Read a file"},
		{Name: "readTextFile", Value: m.readTextFile, Description: "Read a text file"},
		{Name: "treasuryAddress", Value: m.TreasuryAddress(), Description: "Get the treasury address"},
		{Name: "genKey", Value: m.GenKey, Description: "Generate an Ed25519 key"},
		{Name: "dump", Value: m.dump, Description: "Dump displays the passed parameters to standard out"},
		{Name: "diff", Value: m.diff, Description: "Diff returns a human-readable report of the differences between two values"},
	}
}

// methods are functions exposed in the special namespace of this module.
func (m *UtilModule) methods() []*types.ModuleFunc {
	return m.globals()
}

// completer returns suggestions for console input
func (m *UtilModule) completer(d prompt.Document) []prompt.Suggest {
	if words := d.GetWordBeforeCursor(); len(words) > 1 {
		return prompt.FilterHasPrefix(m.suggestions, words, true)
	}
	return nil
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *UtilModule) ConfigureVM(vm *otto.Otto) prompt.Completer {
	m.vm = vm

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, constants.NamespaceUtil, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceUtil, f.Name)
		m.suggestions = append(m.suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		m.suggestions = append(m.suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.completer
}

// prettyPrint pretty prints any object.
// Useful for debugging.
func (m *UtilModule) prettyPrint(values ...interface{}) {
	var v interface{} = values
	if len(values) == 1 {
		v = values[0]
	}
	f := prettyjson.NewFormatter()
	f.NewlineArray = ""
	bs, err := f.Marshal(v)
	if err != nil {
		panic(m.vm.MakeCustomError("PrettyPrintError", err.Error()))
	}
	fmt.Println(string(bs))
}

// Dump displays the passed parameters to standard out.
func (m *UtilModule) dump(objs ...interface{}) {
	spew.Dump(objs...)
}

// Diff returns a human-readable report of the differences between two values.
// It returns an empty string if and only if Equal returns true for the same
// input values and options.
func (m *UtilModule) diff(a, b interface{}) {
	fmt.Print(cmp.Diff(a, b))
}

// eval executes javascript represent as string
func (m *UtilModule) eval(src interface{}) interface{} {
	script, err := m.vm.Compile("", src)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	out, err := m.vm.Run(script)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	return out
}

func (m *UtilModule) evalFile(file string) interface{} {

	fullPath, err := filepath.Abs(file)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	return m.eval(content)
}

func (m *UtilModule) readFile(filename string) interface{} {

	if !filepath.IsAbs(filename) {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		filename = filepath.Join(dir, filename)
	}

	bz, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	return bz
}

func (m *UtilModule) readTextFile(filename string) string {
	bz := m.readFile(filename)
	return string(bz.([]byte))
}

func (m *UtilModule) TreasuryAddress() string {
	return params.TreasuryAddress
}

// genKey generates an Ed25519 key.
// seed: Specify an optional seed
func (m *UtilModule) GenKey(seed ...int64) interface{} {

	var s *int64 = nil
	if len(seed) > 0 {
		s = &seed[0]
	}

	key, err := crypto.NewKey(s)
	if err != nil {
		panic(err)
	}

	res := map[string]interface{}{}
	res["publicKey"] = key.PubKey().Base58()
	res["privateKey"] = key.PrivKey().Base58()
	res["address"] = key.Addr().String()
	return res
}

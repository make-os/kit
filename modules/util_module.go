package modules

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/c-bata/go-prompt"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	"github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/params"
	"gitlab.com/makeos/lobe/types/constants"
	"gitlab.com/makeos/lobe/util"
)

// ConsoleUtilModule provides access to various console utility functions.
type ConsoleUtilModule struct {
	types.ModuleCommon
	vm     *otto.Otto
	stdout io.Writer
}

// NewConsoleUtilModule creates an instance of ConsoleUtilModule
func NewConsoleUtilModule(stdout io.Writer) *ConsoleUtilModule {
	return &ConsoleUtilModule{stdout: stdout}
}

// globals are functions exposed in the VM's global namespace
func (m *ConsoleUtilModule) globals() []*types.VMMember {
	return []*types.VMMember{
		{Name: "pp", Value: m.PrettyPrint, Description: "Pretty print an object"},
		{Name: "eval", Value: m.Eval, Description: "Execute JavaScript code represented as a string"},
		{Name: "evalFile", Value: m.EvalFile, Description: "Execute JavaScript code stored in a file"},
		{Name: "readFile", Value: m.ReadFile, Description: "Read a file"},
		{Name: "readTextFile", Value: m.ReadTextFile, Description: "Read a text file"},
		{Name: "treasuryAddress", Value: m.TreasuryAddress(), Description: "Get the treasury address"},
		{Name: "genKey", Value: m.GenKey, Description: "Generate an Ed25519 key"},
		{Name: "dump", Value: m.Dump, Description: "Dump displays the passed parameters to standard out"},
		{Name: "diff", Value: m.Diff, Description: "Diff returns a human-readable report of the differences between two values"},
	}
}

// methods are functions exposed in the special namespace of this module.
func (m *ConsoleUtilModule) methods() []*types.VMMember {
	return m.globals()
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *ConsoleUtilModule) ConfigureVM(vm *otto.Otto) prompt.Completer {
	m.vm = vm

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, constants.NamespaceConsoleUtil, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceConsoleUtil, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// PrettyPrint pretty prints any object.
// Useful for debugging.
func (m *ConsoleUtilModule) PrettyPrint(values ...interface{}) {
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

	fmt.Fprintln(m.stdout, string(bs))
}

// Dump displays the passed parameters to standard out.
func (m *ConsoleUtilModule) Dump(objs ...interface{}) {
	spew.Fdump(m.stdout, objs...)
}

// Diff returns a human-readable report of the differences between two values.
// It returns an empty string if and only if Equal returns true for the same
// input values and options.
func (m *ConsoleUtilModule) Diff(a, b interface{}) {
	fmt.Fprint(m.stdout, cmp.Diff(a, b))
}

// Eval executes the given JavaScript source and returns the output
func (m *ConsoleUtilModule) Eval(src interface{}) otto.Value {
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

// EvalFile executes given JavaScript script file and returns the output
func (m *ConsoleUtilModule) EvalFile(file string) otto.Value {

	fullPath, _ := filepath.Abs(file)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	return m.Eval(content)
}

// ReadFile returns the content of the given file
func (m *ConsoleUtilModule) ReadFile(filename string) []byte {

	if !filepath.IsAbs(filename) {
		dir, _ := os.Getwd()
		filename = filepath.Join(dir, filename)
	}

	bz, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	return bz
}

// ReadTextFile is like ReadTextFile but casts the content to string type.
func (m *ConsoleUtilModule) ReadTextFile(filename string) string {
	return string(m.ReadFile(filename))
}

// TreasuryAddress returns the treasury address
func (m *ConsoleUtilModule) TreasuryAddress() string {
	return params.TreasuryAddress
}

// GenKey generates an Ed25519 key.
// seed: Specify an optional seed
func (m *ConsoleUtilModule) GenKey(seed ...int64) util.Map {

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

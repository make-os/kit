package modules

import (
	"fmt"
	"gitlab.com/makeos/mosdef/types"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/c-bata/go-prompt"
	prettyjson "github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/crypto"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"
)

// UtilModule provides access to various utility functions
type UtilModule struct {
	vm *otto.Otto
}

// NewUtilModule creates an instance of UtilModule
func NewUtilModule(vm *otto.Otto) *UtilModule {
	return &UtilModule{vm: vm}
}

func (m *UtilModule) globals() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		{
			Name:        "pp",
			Value:       m.prettyPrint,
			Description: "Pretty print an object",
		},
		{
			Name:        "eval",
			Value:       m.eval,
			Description: "Execute javascript code represented as a string",
		},
		{
			Name:        "evalFile",
			Value:       m.evalFile,
			Description: "Execute javascript code stored in a file",
		},
		{
			Name:        "readFile",
			Value:       m.readFile,
			Description: "Read a file",
		},
		{
			Name:        "readTextFile",
			Value:       m.readTextFile,
			Description: "Read a text file",
		},
		{
			Name:        "treasuryAddress",
			Value:       m.treasuryAddress(),
			Description: "Get the treasury address",
		},
		{
			Name:        "genKey",
			Value:       m.genKey,
			Description: "Generate an Ed25519 key",
		},
	}
}

// funcs exposed by the module
func (m *UtilModule) funcs() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		{
			Name:        "prettyPrint",
			Value:       m.prettyPrint,
			Description: "Pretty print an object",
		},
		{
			Name:        "eval",
			Value:       m.eval,
			Description: "Execute javascript code represented as a string",
		},
		{
			Name:        "evalFile",
			Value:       m.evalFile,
			Description: "Execute javascript code stored in a file",
		},
		{
			Name:        "readFile",
			Value:       m.readFile,
			Description: "Read a file",
		},
		{
			Name:        "readTextFile",
			Value:       m.readTextFile,
			Description: "Read a text file",
		},
		{
			Name:        "treasuryAddress",
			Value:       m.treasuryAddress(),
			Description: "Get the treasury address",
		},
		{
			Name:        "genKey",
			Value:       m.genKey,
			Description: "Generate an Ed25519 key",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *UtilModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceUtil, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceUtil, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// prettyPrint pretty prints any object.
// Useful for debugging.
func (m *UtilModule) prettyPrint(values ...interface{}) {
	var v interface{} = values
	if len(values) == 1 {
		v = values[0]
	}
	bs, err := prettyjson.Marshal(v)
	if err != nil {
		panic(m.vm.MakeCustomError("PrettyPrintError", err.Error()))
	}
	fmt.Println(string(bs))
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

func (m *UtilModule) treasuryAddress() string {
	return params.TreasuryAddress
}

// genKey generates an Ed25519 key.
// seed: Specify an optional seed
func (m *UtilModule) genKey(seed ...int64) interface{} {

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

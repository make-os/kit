package jsmodules

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	prettyjson "github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
)

// UtilModule provides access to various utility functions
type UtilModule struct {
	vm *otto.Otto
}

// NewUtilModule creates an instance of UtilModule
func NewUtilModule(vm *otto.Otto) *UtilModule {
	return &UtilModule{vm: vm}
}

func (m *UtilModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "pp",
			Value:       m.prettyPrint,
			Description: "Pretty print an object",
		},
		&types.JSModuleFunc{
			Name:        "eval",
			Value:       m.eval,
			Description: "Execute javascript code represented as a string",
		},
		&types.JSModuleFunc{
			Name:        "evalScript",
			Value:       m.evalScript,
			Description: "Execute javascript code stored in a file",
		},
	}
}

// funcs exposed by the module
func (m *UtilModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "prettyPrint",
			Value:       m.prettyPrint,
			Description: "Pretty print an object",
		},
		&types.JSModuleFunc{
			Name:        "eval",
			Value:       m.eval,
			Description: "Execute javascript code represented as a string",
		},
		&types.JSModuleFunc{
			Name:        "evalScript",
			Value:       m.evalScript,
			Description: "Execute javascript code stored in a file",
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
func (m *UtilModule) eval(src interface{}) {
	script, err := m.vm.Compile("", src)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	_, err = m.vm.Run(script)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}
}

func (m *UtilModule) evalScript(file string) {

	fullPath, err := filepath.Abs(file)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		panic(m.vm.MakeCustomError("ExecError", err.Error()))
	}

	m.eval(content)
}

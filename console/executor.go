package console

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gobuffalo/packr"

	prompt "github.com/c-bata/go-prompt"

	"github.com/fatih/color"
	"github.com/makeos/mosdef/util/logger"
	prettyjson "github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
)

// Executor is responsible for executing operations inside a
// javascript VM.
type Executor struct {

	// vm is an Otto instance for JS evaluation
	vm *otto.Otto

	// authToken is the token derived from the last login() invocation
	authToken string

	// log is a logger
	log logger.Logger

	// console is the console instance
	console *Console

	// scripts provides access to packed JS scripts
	scripts packr.Box
}

// NewExecutor creates a new executor
func newExecutor(l logger.Logger) *Executor {
	e := new(Executor)
	e.vm = otto.New()
	e.log = l
	e.scripts = packr.NewBox("./scripts")
	return e
}

// PrepareContext adds objects and functions into the VM's global
// contexts allowing users to have access to pre-defined values and objects
func (e *Executor) PrepareContext() ([]prompt.Suggest, error) {

	var suggestions = []prompt.Suggest{}

	// Add some methods to the global namespace
	e.vm.Set("pp", e.pp)
	e.vm.Set("exec", e.runRaw)
	e.vm.Set("runScript", e.runScript)
	e.vm.Set("rs", e.runScript)

	// nsObj is a namespace for storing
	// rpc methods and other categorized functions
	var nsObj = map[string]map[string]interface{}{
		"_system": {},
	}

	defer func() {
		for ns, objs := range nsObj {
			e.vm.Set(ns, objs)
		}

		// Add system scripts
		// e.runRaw(e.scripts.Bytes("transaction_builder.js"))
	}()

	return suggestions, nil
}

func (e *Executor) runScript(file string) {

	fullPath, err := filepath.Abs(file)
	if err != nil {
		panic(e.vm.MakeCustomError("ExecError", err.Error()))
	}

	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		panic(e.vm.MakeCustomError("ExecError", err.Error()))
	}

	e.runRaw(content)
}

func (e *Executor) runRaw(src interface{}) {
	script, err := e.vm.Compile("", src)
	if err != nil {
		panic(e.vm.MakeCustomError("ExecError", err.Error()))
	}

	_, err = e.vm.Run(script)
	if err != nil {
		panic(e.vm.MakeCustomError("ExecError", err.Error()))
	}
}

// pp pretty prints a slice of arbitrary objects
func (e *Executor) pp(values ...interface{}) {
	var v interface{} = values
	if len(values) == 1 {
		v = values[0]
	}
	bs, err := prettyjson.Marshal(v)
	if err != nil {
		panic(e.vm.MakeCustomError("PrettyPrintError", err.Error()))
	}
	fmt.Println(string(bs))
}

// OnInput receives inputs and executes
func (e *Executor) OnInput(in string) {
	switch in {
	case ".help":
		e.help()
	default:
		e.exec(in)
	}
}

func (e *Executor) exec(in string) {

	// RecoverFunc recovers from panics.
	defer func() {
		if r := recover(); r != nil {
			color.Red("Panic: %s", r)
		}
	}()

	v, err := e.vm.Run(in)
	if err != nil {
		color.Red("%s", err.Error())
		return
	}

	if v.IsNull() || v.IsUndefined() {
		color.Magenta("%s", v)
		return
	}

	vExp, _ := v.Export()
	if vExp != nil {
		bs, _ := prettyjson.Marshal(vExp)
		fmt.Println(string(bs))
	}
}

func (e *Executor) help() {
	for _, f := range commonFunc {
		fmt.Println(fmt.Sprintf("%s\t\t%s", f[0], f[1]))
	}
}

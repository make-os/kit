package console

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/pkgs/logger"
)

// Executor is responsible for executing operations inside a
// javascript VM.
type Executor struct {

	// vm is an Otto instance for JS evaluation
	vm *otto.Otto

	// log is a logger
	log logger.Logger

	// console is the console instance
	console *Console
}

// NewExecutor creates a new executor
func newExecutor(l logger.Logger) *Executor {
	e := new(Executor)
	e.vm = otto.New()
	e.log = l.Module("console/executor")
	return e
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
		format := prettyjson.NewFormatter()
		format.NewlineArray = ""
		bs, _ := format.Marshal(vExp)
		fmt.Println(string(bs))
	}
}

func (e *Executor) help() {
	for _, f := range commonFunc {
		fmt.Println(fmt.Sprintf("%s\t\t%s", f[0], f[1]))
	}
}

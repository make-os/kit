package console

import (
	"fmt"

	"github.com/ncodes/go-prettyjson"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/lobe/pkgs/logger"
	fmt2 "gitlab.com/makeos/lobe/util/colorfmt"
)

// Executor is responsible for executing operations inside a
// JavaScript VM.
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

func (e *Executor) exec(in interface{}) {

	// RecoverFunc recovers from panics.
	defer func() {
		if r := recover(); r != nil {
			fmt2.Red("Panic: %s\n", r)
		}
	}()

	v, err := e.vm.Run(in)
	if err != nil {
		fmt2.Red("%s\n", err.Error())
		return
	}

	if v.IsNull() || v.IsUndefined() {
		fmt2.Magenta("%s\n", v)
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

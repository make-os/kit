package console

import (
	"fmt"
	"io"
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
)

// AttachHandler configures VM to provide a context
// similar to a full console experience.
type AttachHandler struct {
	types.ConsoleSuggestions
	rpc     client.Client
	methods []rpc.MethodInfo
	Stdout  io.Writer
}

// NewAttachHandler creates an instance of AttachHandler
func NewAttachHandler(rpc client.Client, methods []rpc.MethodInfo) *AttachHandler {
	return &AttachHandler{rpc: rpc, methods: methods, Stdout: os.Stdout}
}

// ConfigureVM configures the vm
func (a *AttachHandler) ConfigureVM(vm *otto.Otto) (completers []prompt.Completer) {

	// Configure VM with utility module and add AttachHandler's completer
	completers = append(completers,
		modules.NewConsoleUtilModule(a.Stdout).ConfigureVM(vm),
		a.Completer)

	// Add global flag to let script determine mode
	vm.Set("attachMode", true)
	a.Suggestions = append(a.Suggestions, prompt.Suggest{
		Text: "attachMode", Description: "Check if console is in attach mode",
	})

	// Group all methods into their namespace
	nsGroup := make(map[string]map[string]interface{})
	for _, method := range a.methods {
		meth := method
		ns, ok := nsGroup[meth.Namespace]
		if !ok {
			ns = map[string]interface{}{}
			nsGroup[meth.Namespace] = ns
			vm.Set(meth.Namespace, ns)
		}

		// Set call method that calls <namespace_method>
		ns[meth.Name] = func(params ...interface{}) interface{} {
			var p interface{}
			if len(params) > 0 {
				p = params[0]
			}
			resp, _, err := a.rpc.Call(fmt.Sprintf("%s_%s", meth.Namespace, meth.Name), p)
			if err != nil {
				panic(err)
			}
			return resp
		}

		// Add suggestions for <namespace_method> and rpc.<namespace_method>.
		funcFullName := fmt.Sprintf("%s.%s", meth.Namespace, meth.Name)
		a.Suggestions = append(a.Suggestions, prompt.Suggest{Text: funcFullName, Description: meth.Description})
		rpcFullName := fmt.Sprintf("rpc.%s.%s", meth.Namespace, meth.Name)
		a.Suggestions = append(a.Suggestions, prompt.Suggest{Text: rpcFullName, Description: meth.Description})
	}

	// Add <rpc> as parent of the namespaces
	vm.Set("rpc", nsGroup)
	a.Suggestions = append(a.Suggestions, prompt.Suggest{
		Text: "rpc", Description: "Access all RPC namespaces and their methods",
	})

	return
}

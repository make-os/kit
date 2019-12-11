package jsmodules

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/makeos/mosdef/config"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/util"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// ExtensionModule provides extension management capabilities
type ExtensionModule struct {
	cfg  *config.AppConfig
	vm   *otto.Otto
	main types.JSModule
}

// NewExtentionModule creates an instance of ExtensionModule
func NewExtentionModule(
	cfg *config.AppConfig,
	vm *otto.Otto, main types.JSModule) *ExtensionModule {
	return &ExtensionModule{
		cfg:  cfg,
		vm:   vm,
		main: main,
	}
}

func (m *ExtensionModule) namespacedFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "run",
			Value:       m.Run,
			Description: "Load and run an extension",
		},
		&types.JSModuleFunc{
			Name:        "load",
			Value:       m.Load,
			Description: "Load an extension",
		},
		&types.JSModuleFunc{
			Name:        "isInstalled",
			Value:       m.Exist,
			Description: "Check whether an extension is installed",
		},
		&types.JSModuleFunc{
			Name:        "installed",
			Value:       m.Installed,
			Description: "Fetch all installed extensions",
		},
	}
}

func (m *ExtensionModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *ExtensionModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// Set the namespace object
	util.VMSet(m.vm, types.NamespaceExtension, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceExtension, f.Name)
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

func (m *ExtensionModule) prepare(name string, args ...map[string]interface{}) *ExtensionControl {

	var extPath = filepath.Join(m.cfg.GetExtensionDir(), name)
	if filepath.Ext(name) == "" {
		extPath = filepath.Join(m.cfg.GetExtensionDir(), name+".js")
	}

	extBz, err := ioutil.ReadFile(extPath)
	if err != nil {
		panic(fmt.Errorf("failed to open extension, ensure the " +
			"extension exist in the extension directory"))
	}

	var argsMap map[string]interface{}
	if len(args) > 0 {
		argsMap = args[0]
	}

	vm := otto.New()
	m.main.Configure(vm)
	vm.Set("args", argsMap)

	return &ExtensionControl{
		vm:             vm,
		timerInterrupt: make(chan bool),
		script:         extBz,
		args:           argsMap,
	}
}

// Exist checks whether an extension exists
func (m *ExtensionModule) Exist(name string) bool {
	var extPath = filepath.Join(m.cfg.GetExtensionDir(), name)
	if filepath.Ext(name) == "" {
		extPath = filepath.Join(m.cfg.GetExtensionDir(), name+".js")
	}
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		return false
	}
	return true
}

// Installed returns all installed extensions
func (m *ExtensionModule) Installed() (extensions []string) {
	filepath.Walk(m.cfg.GetExtensionDir(), func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		extensions = append(extensions, strings.Split(info.Name(), ".")[0])
		return nil
	})
	return
}

// Load loads an extension
func (m *ExtensionModule) Load(name string, args ...map[string]interface{}) map[string]interface{} {
	ec := m.prepare(name, args...)
	return map[string]interface{}{
		"stop":      ec.stop,
		"isRunning": ec.hasStopped,
		"run": func() {
			if ec.closed {
				panic(fmt.Errorf("stopped extension cannot be restarted"))
			}
			if !ec.running {
				ec.run()
			}
		},
	}
}

// Run loads and starts an extension
func (m *ExtensionModule) Run(name string, args ...map[string]interface{}) map[string]interface{} {
	ec := m.prepare(name, args...)
	ec.run()
	return map[string]interface{}{
		"stop":      ec.stop,
		"isRunning": ec.hasStopped,
	}
}

// ExtensionControl provides functionalities for controlling a loaded extension
type ExtensionControl struct {
	vm             *otto.Otto
	timerInterrupt chan bool
	closed         bool
	running        bool
	script         []byte
	args           map[string]interface{}
}

// stop stops the extension's runtime
func (e *ExtensionControl) stop() {
	if e.closed {
		return
	}

	if !util.IsFuncChanClosed(e.vm.Interrupt) && e.vm.Interrupt != nil {
		close(e.vm.Interrupt)
	}

	if !util.IsBoolChanClosed(e.timerInterrupt) {
		close(e.timerInterrupt)
	}

	e.running = false
	e.closed = true
}

// hasStopped checks if the extension has stopped running
func (e *ExtensionControl) hasStopped() bool {
	return e.running
}

// run the extension
func (e *ExtensionControl) run() {
	err := runExtension(e)
	if err != nil {
		panic(errors.Wrap(err, "failed to create extension vm"))
	}
	e.running = true
}

type _timer struct {
	timer    *time.Timer
	duration time.Duration
	interval bool
	call     otto.FunctionCall
}

// runExtension runs an extension.
// It adds setTimeout and setInterval support.
// See https://github.com/robertkrimen/natto/blob/master/natto.go
func runExtension(ec *ExtensionControl) error {
	registry := map[*_timer]*_timer{}
	ready := make(chan *_timer)

	newTimer := func(call otto.FunctionCall, interval bool) (*_timer, otto.Value) {
		delay, _ := call.Argument(1).ToInteger()
		if 0 >= delay {
			delay = 1
		}

		timer := &_timer{
			duration: time.Duration(delay) * time.Millisecond,
			call:     call,
			interval: interval,
		}
		registry[timer] = timer

		timer.timer = time.AfterFunc(timer.duration, func() {
			ready <- timer
		})

		value, err := call.Otto.ToValue(timer)
		if err != nil {
			panic(err)
		}

		return timer, value
	}

	setTimeout := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, false)
		return value
	}
	ec.vm.Set("setTimeout", setTimeout)

	setInterval := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, true)
		return value
	}
	ec.vm.Set("setInterval", setInterval)

	clearTimeout := func(call otto.FunctionCall) otto.Value {
		timer, _ := call.Argument(0).Export()
		if timer, ok := timer.(*_timer); ok {
			timer.timer.Stop()
			delete(registry, timer)
		}
		return otto.UndefinedValue()
	}
	ec.vm.Set("clearTimeout", clearTimeout)
	ec.vm.Set("clearInterval", clearTimeout)

	go func() {
		defer func() {
			r := recover()
			if r != nil {
				fmt.Println(color.RedString(r.(*otto.Error).String()))
				ec.stop()
			}
		}()

		_, err := ec.vm.Eval(ec.script)
		if err != nil {
			panic(errors.Wrap(err, "failed to execute extension script"))
		}

		for {
			select {
			case <-ec.vm.Interrupt:
				return
			case <-ec.timerInterrupt:
				return
			case timer := <-ready:
				var arguments []interface{}
				if len(timer.call.ArgumentList) > 2 {
					tmp := timer.call.ArgumentList[2:]
					arguments = make([]interface{}, 2+len(tmp))
					for i, value := range tmp {
						arguments[i+2] = value
					}
				} else {
					arguments = make([]interface{}, 1)
				}
				arguments[0] = timer.call.ArgumentList[0]
				_, err := ec.vm.Call(`Function.call.call`, nil, arguments...)
				if err != nil {
					for _, timer := range registry {
						timer.timer.Stop()
						delete(registry, timer)
						panic(err)
					}
				}
				if timer.interval {
					timer.timer.Reset(timer.duration)
				} else {
					delete(registry, timer)
				}
			default:
				// Escape valve!
				// If this isn't here, we deadlock...
			}
			if len(registry) == 0 {
				break
			}
		}
	}()

	return nil
}

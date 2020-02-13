package extensions

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
	"github.com/thoas/go-funk"

	"github.com/makeos/mosdef/util"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// Manager provides extension management capabilities
type Manager struct {
	cfg        *config.AppConfig
	vm         *otto.Otto
	main       types.ModulesAggregator
	runningExt map[string]*ExtensionControl
}

// NewManager creates an instance of Manager
func NewManager(
	cfg *config.AppConfig,
	vm *otto.Otto) *Manager {
	return &Manager{
		cfg:        cfg,
		vm:         vm,
		runningExt: make(map[string]*ExtensionControl),
	}
}

// SetVM sets the main vm where module JS functions are written to
func (m *Manager) SetVM(vm *otto.Otto) *Manager {
	m.vm = vm
	return m
}

// SetMainModule configures the main JS module
func (m *Manager) SetMainModule(main types.ModulesAggregator) {
	m.main = main
}

func (m *Manager) namespacedFuncs() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{
		&types.ModulesAggregatorFunc{
			Name:        "run",
			Value:       m.Run,
			Description: "Load and run an extension",
		},
		&types.ModulesAggregatorFunc{
			Name:        "load",
			Value:       m.Load,
			Description: "Load an extension",
		},
		&types.ModulesAggregatorFunc{
			Name:        "isInstalled",
			Value:       m.Exist,
			Description: "Check whether an extension is installed",
		},
		&types.ModulesAggregatorFunc{
			Name:        "installed",
			Value:       m.Installed,
			Description: "Fetch all installed extensions",
		},
		&types.ModulesAggregatorFunc{
			Name:        "running",
			Value:       m.Running,
			Description: "Fetch a list of running extensions",
		},
		&types.ModulesAggregatorFunc{
			Name:        "isRunning",
			Value:       m.IsRunning,
			Description: "Check whether an extension is currently running",
		},
		&types.ModulesAggregatorFunc{
			Name:        "stop",
			Value:       m.Stop,
			Description: "Stop a running extension",
		},
	}
}

func (m *Manager) globals() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{}
}

// Configure implements types.ModulesAggregator. It configures the JS
// context and return any number of console prompt suggestions
func (m *Manager) Configure() []prompt.Suggest {
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

func (m *Manager) prepare(name string, args ...map[string]string) *ExtensionControl {

	var extPath = filepath.Join(m.cfg.GetExtensionDir(), name)
	if filepath.Ext(name) == "" {
		extPath = filepath.Join(m.cfg.GetExtensionDir(), name+".js")
	}

	extBz, err := ioutil.ReadFile(extPath)
	if err != nil {
		panic(fmt.Errorf("failed to read extension ('%s'), ensure the extension exists", name))
	}

	var argsMap map[string]string
	if len(args) > 0 {
		argsMap = args[0]
	}

	vm := otto.New()
	m.main.ConfigureVM(vm)
	vm.Set("args", argsMap)

	return &ExtensionControl{
		vm:             vm,
		timerInterrupt: make(chan bool),
		script:         extBz,
		args:           argsMap,
	}
}

// Exist checks whether an extension exists
func (m *Manager) Exist(name string) bool {
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
func (m *Manager) Installed() (extensions []string) {
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
func (m *Manager) Load(name string, args ...map[string]string) map[string]interface{} {
	ec := m.prepare(name, args...)
	return map[string]interface{}{
		"isRunning": ec.hasStopped,
		"stop": func() {
			ec.stop()
			delete(m.runningExt, name)
		},
		"run": func() {
			if ec.closed {
				panic(fmt.Errorf("stopped extension cannot be restarted"))
			}
			if m.IsRunning(name) {
				panic(fmt.Errorf("an instance of the extension is currently running"))
			}
			if !ec.running {
				ec.run()
				m.runningExt[name] = ec
			}
		},
	}
}

// Run loads and starts an extension
func (m *Manager) Run(name string, args ...map[string]string) map[string]interface{} {
	ec := m.prepare(name, args...)

	if m.IsRunning(name) {
		panic(fmt.Errorf("an instance of the extension is currently running"))
	}

	ec.run()
	m.runningExt[name] = ec
	return map[string]interface{}{
		"isRunning": ec.hasStopped,
		"stop": func() {
			ec.stop()
			delete(m.runningExt, name)
		},
	}
}

// Stop a running extension
func (m *Manager) Stop(name string) {
	if !m.IsRunning(name) {
		panic(fmt.Errorf("no running extension named '%s'", name))
	}
	ec, _ := m.runningExt[name]
	ec.stop()
	delete(m.runningExt, name)
}

// Running returns a list of running extension
func (m *Manager) Running() []string {
	return funk.Keys(m.runningExt).([]string)
}

// IsRunning checks whether an extension is running
func (m *Manager) IsRunning(name string) bool {
	return m.runningExt[name] != nil
}

// ExtensionControl provides functionalities for controlling a loaded extension
type ExtensionControl struct {
	vm             *otto.Otto
	timerInterrupt chan bool
	closed         bool
	running        bool
	script         []byte
	args           map[string]string
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

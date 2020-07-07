// Package console provides javascript-enabled console
// environment for interacting with the client. It includes
// pre-loaded methods that access the node's RPC interface
// allowing access to the state and condition of the client.
package console

import (
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/modules/types"
	fmt2 "gitlab.com/makeos/mosdef/util/colorfmt"

	"github.com/pkg/errors"

	"gitlab.com/makeos/mosdef/util"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"

	"github.com/c-bata/go-prompt"
)

// Console defines functionalities for create and using
// an interactive Javascript console to perform and query
// the system.
type Console struct {
	sync.RWMutex

	// prompt is the prompt mechanism
	// we are building the console on
	prompt *prompt.Prompt

	// executor is the javascript executor
	executor *Executor

	// suggestMgr managers prompt suggestions
	completerMgr *CompleterManager

	// attached indicates whether the console
	// is in attach mode.
	attached bool

	// historyFile is the path to the file
	// where the file is stored.
	historyFile string

	// history contains the commands
	// collected during this console session.
	history []string

	// cfg is the client config
	cfg *config.AppConfig

	confirmedStop bool

	// onStopFunc is called when the
	// console exists. Console caller
	// use this to perform clean up etc
	onStopFunc func()

	// modules provides access to the system's module APIs
	modules types.ModulesHub

	// Versions
	protocol uint64
	client   string
	runtime  string
	commit   string
}

// New creates a new Console instance.
// signatory is the address
func New(historyPath string, cfg *config.AppConfig, log logger.Logger) *Console {
	c := new(Console)
	c.historyFile = historyPath
	c.executor = newExecutor(log)
	c.completerMgr = newCompleterManager()
	c.executor.console = c
	c.cfg = cfg

	// retrieve the history
	var history []string
	data, _ := ioutil.ReadFile(historyPath)
	if len(data) > 0 {
		_ = util.ToObject(data, &history)
	}

	c.history = append(c.history, history...)

	return c
}

// NewAttached is like New but enables attach mode

// Prepare sets up the console's prompt
// colors, suggestions etc.
func (c *Console) Prepare() error {

	// Set some options
	options := []prompt.Option{
		prompt.OptionPrefixTextColor(prompt.White),
		prompt.OptionAddKeyBind(prompt.KeyBind{
			Key: prompt.ControlC,
			Fn: func(b *prompt.Buffer) {
				c.Stop(false)
			},
		}),
		prompt.OptionDescriptionBGColor(prompt.Black),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionSuggestionTextColor(prompt.Turquoise),
		prompt.OptionSuggestionBGColor(prompt.Black),
		prompt.OptionHistory(c.history),
	}

	// Pass the VM to the system modules for context configuration
	if c.modules != nil {
		c.completerMgr.add(c.modules.ConfigureVM(c.executor.vm)...)
	}

	// create new prompt and configure it
	// with the options create above
	p := prompt.New(func(in string) {
		c.history = append(c.history, in)
		switch in {

		// handle exit command
		case ".exit":
			c.Stop(true)

		// pass other expressions
		// to the JS executor
		default:
			c.confirmedStop = false
			c.executor.OnInput(in)
		}
	}, c.completerMgr.completer, options...)

	c.Lock()
	c.prompt = p
	c.Unlock()

	return nil
}

// SetModulesHub sets the system modules hub
func (c *Console) SetModulesHub(hub types.ModulesHub) {
	c.modules = hub
}

// OnStop sets a function that is called
// when the console is stopped
func (c *Console) OnStop(f func()) {
	c.Lock()
	defer c.Unlock()
	c.onStopFunc = f
}

// Run the console
func (c *Console) Run() error {

	if err := c.Prepare(); err != nil {
		return errors.Wrap(err, "failed to prepare console")
	}

	if !c.cfg.ConsoleOnly() {
		fmt.Println("")
	}
	c.about()
	c.prompt.Run()

	return nil
}

// Stop stops console. It saves command
// history and calls the stop callback
func (c *Console) Stop(immediately bool) {
	c.Lock()
	defer c.Unlock()

	if c.confirmedStop || immediately {
		c.saveHistory()
		if c.onStopFunc != nil {
			c.onStopFunc()
		}
	}

	fmt.Println("(To exit, press ^C again or type .exit)")
	c.confirmedStop = true
}

// SetVersions sets the versions of components
func (c *Console) SetVersions(protocol uint64, client, runtime, commit string) {
	c.Lock()
	defer c.Unlock()
	c.protocol = protocol
	c.client = client
	c.runtime = runtime
	c.commit = commit
}

// about prints some information about
// the version of the client and some
// of its components.
func (c *Console) about() {
	c.RLock()
	defer c.RUnlock()
	fmt.Println(fmt2.CyanString("Welcome to the Javascript Console!"))
	fmt.Println(fmt.Sprintf("Client:%s, Protocol:%d, Commit:%s, Go:%s", c.client, c.protocol, util.String(c.commit).SS(), c.runtime))
	fmt.Println(" type '.exit' to exit console")
	fmt.Println("")
}

// saveHistory stores the commands
// that have been cached in this session.
// Note: Read lock must be called by the caller
func (c *Console) saveHistory() {
	c.history = funk.UniqString(c.history)
	if len(c.history) == 0 {
		return
	}

	bs := util.ToBytes(c.history)
	err := ioutil.WriteFile(c.historyFile, bs, 0644)
	if err != nil {
		panic(err)
	}
}

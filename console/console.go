// Package console provides JavaScript-enabled console
// environment for interacting with the client. It includes
// pre-loaded methods that access the node's RPC interface
// allowing access to the state and condition of the client.
package console

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"sync"

	"github.com/make-os/lobe/api/rpc/client"
	apitypes "github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/modules/types"
	fmt2 "github.com/make-os/lobe/util/colorfmt"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"

	"github.com/pkg/errors"

	"github.com/make-os/lobe/util"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/lobe/config"
)

// Console defines functionalities for create and using
// an interactive JavaScript console to perform and query
// the system.
type Console struct {
	sync.RWMutex

	// prompt is the prompt mechanism
	// we are building the console on
	prompt *prompt.Prompt

	// executor is the JavaScript executor
	executor *Executor

	// suggestMgr managers prompt suggestions
	completerMgr *CompleterManager

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
}

// New creates a new Console instance.
// signatory is the address
func New(cfg *config.AppConfig) *Console {
	c := new(Console)
	c.historyFile = cfg.GetConsoleHistoryPath()
	c.executor = newExecutor(cfg.G().Log.Module("console"))
	c.completerMgr = newCompleterManager()
	c.executor.console = c
	c.cfg = cfg

	// retrieve the history
	var history []string
	data, _ := ioutil.ReadFile(c.historyFile)
	if len(data) > 0 {
		_ = util.ToObject(data, &history)
	}

	c.history = append(c.history, history...)

	return c
}

// Prepare prepares the console and VM
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

	// Create new prompt
	p := prompt.New(func(in string) {
		c.history = append(c.history, in)
		switch in {

		// handle exit command
		case ".exit":
			c.Stop(true)

		// pass other expressions to the JS executor
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

// connectToNode creates an RPC client to configured remote server.
// It will test the connection by getting the RPC methods supported
// by the server. Returns both client and RPC methods on success.
func (c *Console) connectToNote() (client.Client, *apitypes.GetMethodResponse, error) {
	host, port, err := net.SplitHostPort(c.cfg.RPC.Address)
	if err != nil {
		return nil, nil, err
	}

	cl := client.NewClient(&client.Options{
		Host:     host,
		Port:     cast.ToInt(port),
		HTTPS:    c.cfg.RPC.HTTPS,
		User:     c.cfg.RPC.User,
		Password: c.cfg.RPC.Password,
	})

	methods, err := cl.RPC().GetMethods()
	if err != nil {
		return nil, nil, err
	}

	return cl, methods, nil
}

// Run the console.
// If execCode is set, it is executed after context is prepared.
func (c *Console) Run(code ...string) error {

	var err error
	if err = c.Prepare(); err != nil {
		return errors.Wrap(err, "failed to prepare console")
	}

	if !c.cfg.IsAttachMode() {
		fmt.Println("")
	}

	// Execute 'code' if set and stop the console when finished.
	// If code is a file path, read and execute the file content.
	if len(code) > 0 && code[0] != "" {
		var src interface{} = code[0]
		if util.IsPathOk(code[0]) {
			fullPath, _ := filepath.Abs(code[0])
			src, err = ioutil.ReadFile(fullPath)
			if err != nil {
				return errors.Wrap(err, "exec failed; failed to read file")
			}
		}
		c.executor.exec(src)
		c.Stop(true)
		return nil
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
			return
		}
	}

	fmt.Println("(To exit, press ^C again or type .exit)")
	c.confirmedStop = true
}

// about prints some information about
// the version of the client and some
// of its components.
func (c *Console) about() {
	c.RLock()
	defer c.RUnlock()
	fmt.Println(fmt2.CyanString("Welcome to the JavaScript Console!"))
	fmt.Println(fmt.Sprintf("Client:%s, Protocol:%d, Commit:%s, Go:%s",
		c.cfg.VersionInfo.BuildVersion,
		config.GetNetVersion(),
		util.String(c.cfg.VersionInfo.BuildCommit).SS(),
		c.cfg.VersionInfo.GoVersion))
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

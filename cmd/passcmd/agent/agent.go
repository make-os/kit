package agent

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/make-os/kit/pkgs/cache"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

var mem *cache.Cache

func init() {
	mem = cache.NewCacheWithExpiringEntry(100, 5*time.Second)
}

// setHandler handles /set request
func setHandler(w http.ResponseWriter, req *http.Request) {
	body, err := objx.FromURLQuery(req.URL.Query().Encode())
	if err != nil {
		w.WriteHeader(400)
		return
	}

	var exp []time.Time
	if body.Has("ttl") {
		ttl := cast.ToInt(body.Get("ttl").Str())
		exp = append(exp, time.Now().Add(time.Duration(ttl)*time.Second))
	}

	mem.Add(strings.ToLower(body.Get("key").Str()), body.Get("pass").Str(), exp...)
}

// getHandler handles /get request
func getHandler(w http.ResponseWriter, req *http.Request) {
	body, err := objx.FromURLQuery(req.URL.Query().Encode())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !mem.Has(strings.ToLower(body.Get("key").Str())) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	fmt.Fprint(w, mem.Get(strings.ToLower(body.Get("key").Str())))
}

// stopHandler handles /stop request
func stopHandler(_ http.ResponseWriter, _ *http.Request) {
	go os.Exit(0)
}

// statusHandler handles /status request.
func statusHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// AgentStatusChecker describes a function for checking the status of the checker
type AgentStatusChecker func(port string) bool

// IsAgentUp checks whether the agent is running
func IsAgentUp(port string) bool {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/status", port))
	if err != nil {
		return false
	}
	return resp.StatusCode == http.StatusOK
}

// SetRequestSender describes a function for sending set request
type SetRequestSender func(port, key, pass string, ttl int) error

// SendSetRequest sends a set request to store a key/passphrase mapping
func SendSetRequest(port, key, pass string, ttl int) error {
	_, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/set?key=%s&pass=%s&ttl=%d", port, key, pass, ttl))
	if err != nil {
		return err
	}
	return nil
}

// SendGetRequest sends a get request from a passphrase corresponding to the given key
func SendGetRequest(port, key string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/get?key=%s", port, key))
	if err != nil {
		return "", err
	}
	bz, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bz), nil
}

// SendStopRequest sends a stop request to the agent
func SendStopRequest(port string) error {
	_, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/stop", port))
	if err != nil {
		return err
	}
	return nil
}

func getMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/set", setHandler)
	mux.HandleFunc("/get", getHandler)
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/stop", stopHandler)
	return mux
}

// ServerStarterStopperFunc represents a function for starting and stopping a server
type ServerStarterStopperFunc func(port string) error

// RunAgentServer starts up
func RunAgentServer(port string) error {
	return http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", port), getMux())
}

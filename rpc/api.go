package rpc

import (
	"fmt"

	"gitlab.com/makeos/lobe/util"
)

// Params represent JSON API parameters
type Params map[string]interface{}

// Scan attempts to convert the params to a struct or map type
func (p *Params) Scan(dest interface{}) error {
	return util.DecodeMap(p, &dest)
}

// APIInfo defines a standard API function type
// and other parameters.
type APIInfo struct {

	// Func is the API function to be executed.
	Func func(params interface{}) *Response

	// Namespace is the namespace where the method is under
	Namespace string

	// Name is the name of the method
	Name string

	// Private indicates a requirement for a private, authenticated
	// user session before this API function is executed.
	Private bool

	// Description describes the API
	Description string
}

func (a *APIInfo) FullName() string {
	return fmt.Sprintf("%s_%s", a.Namespace, a.Name)
}

// APISet defines a collection of APIs
type APISet []APIInfo

// Get gets an API function by name
// and namespace
func (a *APISet) Get(name string) *APIInfo {
	for _, v := range *a {
		if name == v.FullName() {
			return &v
		}
	}
	return nil
}

// Get gets an API function by name
// and namespace
func (a *APISet) Add(api APIInfo) {
	*a = append(*a, api)
}

// API defines an interface for providing and
// accessing API functions. Packages that offer
// services accessed via RPC or any service-oriented
// interface must implement it.
type API interface {
	APIs() APISet
}

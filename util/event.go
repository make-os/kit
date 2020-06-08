package util

// CheckEvtArgs checks whether the arguments of an event conforms
// to the expected standard where only 2 arguments are expected;
// Argument 0: WaitForResult
// Argument 1: Error
// Panics if standard is not met
func CheckEvtArgs(args []interface{}) error {
	if len(args) != 2 {
		panic("invalid number of arguments")
	}

	if args[0] == nil {
		return nil
	}

	err, ok := args[0].(error)
	if !ok {
		panic("invalid type at evt.Arg[0]")
	}

	return err
}

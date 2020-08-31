package common

type NoopLogger struct{}

func (*NoopLogger) Errorf(string, ...interface{})   {}
func (*NoopLogger) Warningf(string, ...interface{}) {}
func (*NoopLogger) Infof(string, ...interface{})    {}
func (*NoopLogger) Debugf(string, ...interface{})   {}

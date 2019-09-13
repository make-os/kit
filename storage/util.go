package storage

type noLogger struct{}

func (*noLogger) Errorf(string, ...interface{})   {}
func (*noLogger) Warningf(string, ...interface{}) {}
func (*noLogger) Infof(string, ...interface{})    {}
func (*noLogger) Debugf(string, ...interface{})   {}

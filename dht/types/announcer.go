package types

type Announcer interface {
	// Announce queues an object to be announced.
	// objType is the type of the object.
	// key is the unique identifier of the object.
	// doneCB is called after successful announcement
	// Returns true if object has been successfully queued
	Announce(objType int, repo string, key []byte, doneCB func(error)) bool

	// Start starts the announcer.
	// Panics if reference announcer is already started.
	Start()

	// IsRunning checks if the announcer is running.
	IsRunning() bool

	// HasTask checks whether there are one or more unprocessed tasks.
	HasTask() bool

	// NewSession creates an instance of Session
	NewSession() Session

	// Stop stops the announcer and releases resources
	Stop()

	// RegisterChecker allows external caller to register existence checker
	// for a given object type. Only one checker per object type.
	RegisterChecker(objType int, checker CheckFunc)
}

// AnnouncerService is like Announcer but exposes limited methods
type AnnouncerService interface {

	// Announce queues an object to be announced.
	// objType is the type of the object.
	// key is the unique identifier of the object.
	// doneCB is called after successful announcement
	Announce(objType int, repo string, key []byte, doneCB func(error)) bool
}

type Session interface {
	Announce(objType int, repo string, key []byte) bool
	OnDone(cb func(errCount int))
}

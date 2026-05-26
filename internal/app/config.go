package app

// Version is the current CLI version. It is usually set during the build process.
var Version = "0.1.0-dev"

// Config controls runtime behavior.
type Config struct {
	InitialQuery   string
	ContextCommand string
	ForceOffline   bool
}

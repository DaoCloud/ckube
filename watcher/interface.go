package watcher

type Watcher interface {
	Start() error
	Stop() error
}

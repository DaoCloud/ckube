package fake

import "k8s.io/client-go/rest"

type EventAction int

const (
	EventActionError  EventAction = 0
	EventActionAdd    EventAction = 1
	EventActionUpdate EventAction = 2
	EventActionDelete EventAction = 2
)

type Event struct {
	EventAction
	Group     string
	Version   string
	Resource  string
	Namespace string
	Name      string
}

type CkubeServer interface {
	Events() <-chan Event
	GetKubeConfig() *rest.Config
	Stop()
}

package utils

import (
	"github.com/DaoCloud/ckube/log"
	"github.com/fsnotify/fsnotify"
	"io"
	"os"
	"sync"
	"time"
)

type FixedFileWatcher interface {
	io.Closer
	Start() error
	Events() <-chan Event
}

type Event struct {
	Name string
	Type EventType
}

type EventType int

const (
	EventTypeChanged = iota
	EventTypeError
)

type fixedFileWatcher struct {
	files     []string
	mux       sync.RWMutex
	fswatcher *fsnotify.Watcher
	events    chan Event
}

func NewFixedFileWatcher(files []string) (FixedFileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return nil, err
		}
		if err := w.Add(f); err != nil {
			return nil, err
		}
	}
	return &fixedFileWatcher{
		files:  files,
		events: make(chan Event),
	}, nil
}

func (w *fixedFileWatcher) Start() error {
	ww, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fswatcher = ww
	for _, f := range w.files {
		if err := w.fswatcher.Add(f); err != nil {
			return err
		}
	}
	go func() {
		for {
			select {
			case e, open := <-w.fswatcher.Events:
				if !open {
					log.Info("fs watcher closed")
					return
				}
				log.Infof("get file watcher event: %v", e)
				switch e.Op {
				case fsnotify.Write:
					// do reload
				case fsnotify.Remove:
					// 在 Kubernetes 里面，当挂载 ConfigMap 的时候，如果发生文件重新，Kubernetes 会首先删除这个文件
					// 再重新创建，所以我们应该在删除之后重新建立 watcher。
					w.fswatcher.Remove(e.Name)
					time.Sleep(time.Second * 2)
					// 等待一定时间之后重新加入 watcher 队列
					err := w.fswatcher.Add(e.Name)
					if err != nil {
						log.Errorf("add watcher for %s error: %v", e.Name, err)
						w.events <- Event{
							Name: e.Name,
							Type: EventTypeError,
						}
						continue
					}
					// do reload
				default:
					// do not reload
					continue
				}
				w.events <- Event{
					Name: e.Name,
					Type: EventTypeChanged,
				}
			}
		}
	}()
	return nil
}

func (w *fixedFileWatcher) Close() error {
	if w.fswatcher != nil {
		w.fswatcher.Close()
		w.fswatcher = nil
		close(w.events)
		w.events = nil
	}
	return nil
}

func (w *fixedFileWatcher) Events() <-chan Event {
	return w.events
}

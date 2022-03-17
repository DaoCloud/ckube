package utils

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestFixedFileWatcher_Events(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		files      []string
		init       func()
		events     func(w FixedFileWatcher)
		wantEvents []Event
	}{
		{
			name:  "normal",
			files: []string{"/tmp/dsm-watcher-test-1"},
			init: func() {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test"), 0777)
			},
			events: func(w FixedFileWatcher) {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test-1"), 0777)
				time.Sleep(time.Second)
				w.Close()
			},
			wantEvents: []Event{
				{
					Name: "/tmp/dsm-watcher-test-1",
					Type: EventTypeChanged,
				},
			},
		},
		{
			name:  "mutiple files",
			files: []string{"/tmp/dsm-watcher-test-1", "/tmp/dsm-watcher-test-2"},
			init: func() {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test"), 0777)
				ioutil.WriteFile("/tmp/dsm-watcher-test-2", []byte("test"), 0777)
			},
			events: func(w FixedFileWatcher) {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test-1"), 0777)
				time.Sleep(time.Second)
				ioutil.WriteFile("/tmp/dsm-watcher-test-2", []byte("test-2"), 0777)
				time.Sleep(time.Second)
				w.Close()
			},
			wantEvents: []Event{
				{
					Name: "/tmp/dsm-watcher-test-1",
					Type: EventTypeChanged,
				},
				{
					Name: "/tmp/dsm-watcher-test-2",
					Type: EventTypeChanged,
				},
			},
		},
		{
			name:  "remove and recreate",
			files: []string{"/tmp/dsm-watcher-test-1"},
			init: func() {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test"), 0777)
			},
			events: func(w FixedFileWatcher) {
				os.Remove("/tmp/dsm-watcher-test-1")
				time.Sleep(time.Second)
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test-1"), 0777)
				time.Sleep(time.Second * 3) // wait for recheck
				w.Close()
			},
			wantEvents: []Event{
				{
					Name: "/tmp/dsm-watcher-test-1",
					Type: EventTypeChanged,
				},
			},
		},
		{
			name:  "remove",
			files: []string{"/tmp/dsm-watcher-test-1"},
			init: func() {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test"), 0777)
			},
			events: func(w FixedFileWatcher) {
				os.Remove("/tmp/dsm-watcher-test-1")
				time.Sleep(time.Second * 3) // wait for recheck
				w.Close()
			},
			wantEvents: []Event{
				{
					Name: "/tmp/dsm-watcher-test-1",
					Type: EventTypeError,
				},
			},
		},
		{
			name:  "symlink",
			files: []string{"/tmp/dsm-watcher-test-link"},
			init: func() {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test"), 0777)
				os.Symlink("/tmp/dsm-watcher-test-1", "/tmp/dsm-watcher-test-link")
			},
			events: func(w FixedFileWatcher) {
				ioutil.WriteFile("/tmp/dsm-watcher-test-1", []byte("test-2"), 0777)
				time.Sleep(time.Second) // wait for recheck
				w.Close()
				os.Remove("/tmp/dsm-watcher-test-1")
			},
			wantEvents: []Event{
				{
					Name: func() string {
						if runtime.GOOS == "darwin" {
							return "/private/tmp/dsm-watcher-test-1"
						}
						// linux will return the symlink file name
						// but macos return the target file name
						return "/tmp/dsm-watcher-test-link"
					}(),
					Type: EventTypeChanged,
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.init()
			w, err := NewFixedFileWatcher(c.files)
			assert.NoError(t, err)
			defer func() {
				for _, f := range c.files {
					os.Remove(f)
				}
			}()
			es := []Event{}
			w.Start()
			stop := make(chan struct{})
			go func() {
				for {
					e, open := <-w.Events()
					if !open {
						close(stop)
						break
					}
					es = append(es, e)
				}
			}()
			c.events(w)
			<-stop
			assert.Equal(t, c.wantEvents, es)
		})
	}
}

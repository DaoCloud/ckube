package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	Debug     = log.Debug
	Debugf    = log.Debugf
	Info      = log.Info
	Infof     = log.Infof
	Warn      = log.Warn
	Warning   = log.Warn
	Warnf     = log.Warnf
	Error     = log.Error
	Errorf    = log.Errorf
	WithField = log.WithField
	AccessLog = log.New()
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp:       false,
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		DisableColors:          true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			fs := strings.Split(f.File, "/")
			filename := fs[len(fs)-1]
			ff := strings.Split(f.Function, "/")
			_f := ff[len(ff)-1]
			return fmt.Sprintf("%s()", _f), fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(true)
	AccessLog.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
}

func SetDebug() {
	log.SetLevel(log.DebugLevel)
}

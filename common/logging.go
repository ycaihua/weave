package common

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
)

type textFormatter struct {
}

// Based off logrus.TextFormatter, which behaves completely
// differently when you don't want colored output
func (f *textFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	levelText := strings.ToUpper(entry.Level.String())[0:4]
	timeStamp := entry.Time.Format("2006/01/02 15:04:05.999999")
	if len(entry.Data) > 0 {
		fmt.Fprintf(b, "%s: %s %-44s ", levelText, timeStamp, entry.Message)
		for k, v := range entry.Data {
			fmt.Fprintf(b, " %s=%v", k, v)
		}
	} else {
		// No padding when there's no fields
		fmt.Fprintf(b, "%s: %s %s", levelText, timeStamp, entry.Message)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

var (
	standardTextFormatter = &textFormatter{}
)

var (
	Log  *logrus.Logger
	Info *logrus.Logger
)

func InitLogging(level logrus.Level) {
	if Info == nil {
		Info = &logrus.Logger{
			Out:       os.Stderr,
			Formatter: standardTextFormatter,
			Hooks:     make(logrus.LevelHooks),
			Level:     level,
		}
		Log = Info
	}
	Info.Level = level
}

func InitDefaultLogging(debug bool) {
	level := logrus.InfoLevel
	if debug {
		level = logrus.DebugLevel
	}
	InitLogging(level)
}

func SetLogLevel(levelname string) error {
	level, err := logrus.ParseLevel(levelname)
	if err != nil {
		return err
	}
	InitLogging(level)
	return nil
}

// Combination of InitDefaultLogging and SetLogLevel, for convenience
// of existing programs that support both options
func InitDefaultLoggingOrDie(debug bool, loglevel string) {
	if loglevel != "" {
		if err := SetLogLevel(loglevel); err != nil {
			Log.Fatal(err)
		}
	} else {
		InitDefaultLogging(debug)
	}
}

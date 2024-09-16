package redislock

import (
	"errors"
	"io"
	"log"
	"os"
	"time"
)

// ErrRecordNotFound record not found error
var ErrRecordNotFound = errors.New("record not found")

// Colors
const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BlueBold    = "\033[34;1m"
	MagentaBold = "\033[35;1m"
	RedBold     = "\033[31;1m"
	YellowBold  = "\033[33;1m"
)

// LogLevel log level
type LogLevel int

const (
	// Silent silent log level
	Silent LogLevel = iota + 1
	// Error error log level
	Error
	// Warn warn log level
	Warn
	// Info info log level
	Info
)

var (
	// Discard logger will print any log to io.Discard
	Discard = New(log.New(io.Discard, "", log.LstdFlags), Config{})
	// Default Default logger
	Default = New(log.New(os.Stdout, "\r\n", log.LstdFlags), Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
	})
)

// Logger logger interface
type Logger interface {
	LogMode(LogLevel) Logger
	Info(string, ...interface{})
	Warn(string, ...interface{})
	Error(string, ...interface{})
}

// New initialize logger
func New(writer Writer, config Config) Logger {
	var (
		infoStr      = "%s\n[info] "
		warnStr      = "%s\n[warn] "
		errStr       = "%s\n[error] "
		traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	if config.Colorful {
		infoStr = Green + "%s\n" + Reset + Green + "[info] " + Reset
		warnStr = BlueBold + "%s\n" + Reset + Magenta + "[warn] " + Reset
		errStr = Magenta + "%s\n" + Reset + Red + "[error] " + Reset
		traceStr = Green + "%s\n" + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
		traceWarnStr = Green + "%s " + Yellow + "%s\n" + Reset + RedBold + "[%.3fms] " + Yellow + "[rows:%v]" + Magenta + " %s" + Reset
		traceErrStr = RedBold + "%s " + MagentaBold + "%s\n" + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
	}

	return &logger{
		Writer:       writer,
		Config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}
}

// Writer log writer interface
type Writer interface {
	Printf(string, ...interface{})
}

// Config logger config
type Config struct {
	SlowThreshold             time.Duration
	Colorful                  bool
	IgnoreRecordNotFoundError bool
	ParameterizedQueries      bool
	LogLevel                  LogLevel
}

type logger struct {
	Writer
	Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

// LogMode log mode
func (l *logger) LogMode(level LogLevel) Logger {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info print info
func (l *logger) Info(msg string, data ...interface{}) {
	if l.LogLevel >= Info {
		l.Printf(l.infoStr+msg, data)
	}
}

// Warn print warn messages
func (l *logger) Warn(msg string, data ...interface{}) {
	if l.LogLevel >= Warn {
		l.Printf(l.warnStr+msg, data...)
	}
}

// Error print error messages
func (l *logger) Error(msg string, data ...interface{}) {
	if l.LogLevel >= Error {
		l.Printf(l.errStr+msg, data)
	}
}

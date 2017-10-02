package log

import (
	"fmt"
	"io"
	"log"
)

const (
	LL_CRIT = iota
	LL_ERR
	LL_WARN
	LL_INFO
	LL_DEBUG
)

var (
	Metalog *ParseLogger
	llStr   = []string{"CRIT", "ERR", "WARN", "INFO", "DEBUG"}
)

type ParseLogger struct {
	logger    *log.Logger
	strict    bool
	dieLog    func(v ...interface{})
	normalLog func(v ...interface{})
}

func New(w io.Writer, prefix string) *ParseLogger {
	parselog := &ParseLogger{}
	parselog.logger = log.New(w, prefix, log.Llongfile)
	parselog.dieLog = log.Fatal
	parselog.normalLog = log.Print
	return parselog
}

func (self *ParseLogger) Log(level int, object string, value string) {
	logline := fmt.Sprintf("%s [%s] %s", llStr[level], object, value)
	if level == LL_CRIT {
		self.dieLog(logline)
	}
	self.normalLog(logline)
}

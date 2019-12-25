package nano

import (
	"log"
	"os"

	"github.com/uber/jaeger-client-go"
)

//Logger represents  the log interface
type Logger interface {
	Println(v ...interface{})
	Fatal(v ...interface{})
}

// Default logger
var logger Logger = log.New(os.Stderr, "", log.LstdFlags|log.Llongfile)

// SetLogger rewrites the default logger
func SetLogger(l Logger) {
	if l != nil {
		logger = l
	}
}

var opentracingLogger jaeger.Logger

// SetLogger rewrites the default logger
func SetJaegerLogger(l jaeger.Logger) {
	if l != nil {
		opentracingLogger = l
	}
}

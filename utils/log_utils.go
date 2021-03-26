package utils

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

var debug bool

func init() {
	debug = os.Getenv("DEBUG") != ""
}

// LogInfo example:
//
// LogInfo("timezone %s", timezone)
//
func LogInfo(msg string, vars ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	fileAsPaths := strings.Split(file, "/")
	log.Printf(strings.Join([]string{"[INFO]", fmt.Sprintf("[%s:%d]", fileAsPaths[len(fileAsPaths)-1], line), msg}, " "), vars...)
}

// Debug example:
//
// Debug("timezone %s", timezone)
//
func LogDebug(msg string, vars ...interface{}) {
	if debug {
		_, file, line, _ := runtime.Caller(1)
		fileAsPaths := strings.Split(file, "/")
		log.Printf(strings.Join([]string{"[DEBUG]", fmt.Sprintf("[%s:%d]", fileAsPaths[len(fileAsPaths)-1], line), msg}, " "), vars...)
	}
}

// Fatal example:
//
// Fatal(errors.New("db timezone must be UTC"))
//
func LogFatal(err error) {
	pc, fn, line, _ := runtime.Caller(1)
	// Include function name if debugging
	if debug {
		log.Fatalf("[FATAL] %s [%s:%s:%d]", err, runtime.FuncForPC(pc).Name(), fn, line)
	} else {
		log.Fatalf("[FATAL] %s [%s:%d]", err, fn, line)
	}
}

// Error example:
//
// Error(errors.Errorf("Invalid timezone %s", timezone))
//
func LogError(err error) {
	pc, fn, line, _ := runtime.Caller(1)
	// Include function name if debugging
	if debug {
		log.Printf("[ERROR] [%s:%s:%d] %s", runtime.FuncForPC(pc).Name(), fn, line, err)
	} else {
		log.Printf("[ERROR] [%s:%d] %s", fn, line, err)
	}
}

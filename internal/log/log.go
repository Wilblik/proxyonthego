package log

import (
	"log"
	"os"
	"io"
)

var (
	infoLog = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime)
	errLog  = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime)
)

func LogInfo(format string, v ...any) {
	infoLog.Printf(format, v...)
}

func LogErr(format string, v ...any) {
	errLog.Printf(format, v...)
}

func LogFatalf(format string, v ...any) {
	errLog.Fatalf(format, v...)
}

func LogFatal(v ...any) {
	errLog.Fatal(v...)
}

func DisableInfo() {
	infoLog.SetOutput(io.Discard)
}

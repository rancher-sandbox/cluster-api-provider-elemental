package log

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log logr.Logger
var config zap.Config

const (
	DebugLevel = 1
	InfoLevel  = 0
)

func init() {
	config = zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.Level(0))
	// Make the logging human readable
	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	// Build the logger
	zap, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("initializing logger (%v)?", err))
	}
	log = zapr.NewLogger(zap)
}

func EnableDebug() {
	config.Level = zap.NewAtomicLevelAt(zapcore.Level(-1))
	zap, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("enabling debug on logger (%v)?", err))
	}
	log = zapr.NewLogger(zap)
	Debug("Debug logging enabled")
}

func Info(msg string, keysAndValues ...any) {
	log.WithCallDepth(1).V(InfoLevel).Info(msg, keysAndValues...)
}

func Infof(format string, values ...any) {
	log.WithCallDepth(1).V(InfoLevel).Info(fmt.Sprintf(format, values...))
}

func Debug(msg string, keysAndValues ...any) {
	log.WithCallDepth(1).V(DebugLevel).Info(msg, keysAndValues...)
}

func Debugf(format string, values ...any) {
	log.WithCallDepth(1).V(DebugLevel).Info(fmt.Sprintf(format, values...))
}

func Error(err error, msg string, keysAndValues ...any) {
	log.WithCallDepth(1).Error(err, msg, keysAndValues...)
}

func Errorf(err error, format string, values ...any) {
	log.WithCallDepth(1).Error(err, fmt.Sprintf(format, values...))
}

func Fatal(err error, msg string, keysAndValues ...any) {
	log.WithCallDepth(1).Error(err, msg, keysAndValues...)
	os.Exit(1)
}

func Fatalf(err error, format string, values ...any) {
	log.WithCallDepth(1).Error(err, fmt.Sprintf(format, values...))
	os.Exit(1)
}

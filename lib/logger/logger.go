package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log *zap.Logger
var atomicLevel zap.AtomicLevel

func Debug(msg string, fields ...zap.Field) {
	log.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	log.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	log.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	log.Error(msg, fields...)
}

func DPanic(msg string, fields ...zap.Field) {
	log.DPanic(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	log.Panic(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	log.Fatal(msg, fields...)
}

func Sync() {
	log.Sync()
}

func SetLogLevel(level int) {
	atomicLevel.SetLevel(zapcore.Level(level))
}

func init() {
	hook := lumberjack.Logger{
		Filename:   filepath.Join(os.TempDir(), "nps.log"),
		MaxSize:    128,
		MaxBackups: 30,
		MaxAge:     7,
		Compress:   true,
	}

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.TimeKey = "log_time"
	encoderConfig.LevelKey = "level"
	encoderConfig.NameKey = "logger"
	encoderConfig.CallerKey = "caller"
	encoderConfig.MessageKey = "msg"
	encoderConfig.StacktraceKey = "StacktraceKey"
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05"))
	}

	consoleConfig := encoderConfig
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	atomicLevel = zap.NewAtomicLevel()
	// init info level
	atomicLevel.SetLevel(zapcore.Level(-1))

	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.NewMultiWriteSyncer(zapcore.AddSync(&hook)), atomicLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(consoleConfig), zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), atomicLevel),
	)

	caller := zap.AddCaller()
	development := zap.Development()

	log = zap.New(core, caller, development, zap.AddCallerSkip(1), zap.AddStacktrace(zap.ErrorLevel))
	defer log.Sync()
	undo := zap.ReplaceGlobals(log)
	defer undo()
	logLevelSignal()

}

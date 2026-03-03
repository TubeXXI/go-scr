package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"tubexxi/scraper/pkg/utils"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger         *zap.Logger
	loggerOnce     sync.Once
	fallbackLogger = zap.NewExample()
	loggerMu       sync.Mutex
)

func InitJSONLogger() (*zap.Logger, error) {
	var err error
	Logger, err = initLogger()
	if err != nil {
		fallbackLogger.Error("Failed to initialize custom logger",
			zap.Error(err),
			zap.String("fallback", "using example logger"),
		)
		Logger = fallbackLogger
	}
	return Logger, err
}

func initLogger() (*zap.Logger, error) {
	resultDir := "./logs"

	err := utils.EnsureDir(resultDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	jsonFile := "logs.json"
	jsonPath := filepath.Join(resultDir, jsonFile)

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	jsonEncoder := zapcore.NewJSONEncoder(encoderConfig)

	consoleEncoderConfig := encoderConfig
	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

	fileWriter, err := createFileWriter(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file writer: %w", err)
	}

	consoleWriter := zapcore.AddSync(os.Stdout)

	logLevel := zapcore.DebugLevel

	core := zapcore.NewTee(
		zapcore.NewCore(jsonEncoder, fileWriter, logLevel),
		zapcore.NewCore(consoleEncoder, consoleWriter, logLevel),
	)

	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.AddCallerSkip(1),
		zap.Fields(
			zap.String("app", "tubexxi-scraper"),
			zap.String("env", "development"),
		),
		zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(
				core,
				time.Second,
				500, // initial
				50,  // thereafter
			)
		}),
	)

	zap.RedirectStdLog(logger)

	return logger, nil
}
func createFileWriter(path string) (zapcore.WriteSyncer, error) {
	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}
	if _, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err != nil {
		return nil, err
	}

	return zapcore.AddSync(writer), nil
}
func GetLogger() *zap.Logger {
	if Logger == nil {
		var err error
		Logger, err = initLogger()
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			Logger = fallbackLogger
		}
	}
	return Logger
}
func CloseLogger() {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if Logger != nil {
		err := Logger.Sync()
		if err != nil && !isHarmlessSyncError(err) {
			fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
		}
	}
}
func isHarmlessSyncError(err error) bool {
	if runtime.GOOS == "linux" &&
		strings.Contains(err.Error(), "sync /dev/stdout: invalid argument") {
		return true
	}

	if runtime.GOOS == "windows" &&
		strings.Contains(err.Error(), "The handle is invalid") {
		return true
	}

	return false
}

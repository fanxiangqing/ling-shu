package log

import (
	"fmt"
	"os"
	"strings"

	"ling-shu/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(cfg config.LogConfig) (*zap.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	encoding := strings.ToLower(strings.TrimSpace(cfg.Encoding))
	if encoding == "" {
		encoding = "json"
	}
	if encoding != "json" && encoding != "console" {
		return nil, fmt.Errorf("unsupported log encoding: %s", cfg.Encoding)
	}

	encoderCfg := newEncoderConfig()
	if encoding == "console" {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoder := newEncoder(encoding, encoderCfg)
	enabler := zap.NewAtomicLevelAt(level)

	cores := make([]zapcore.Core, 0, 2)
	if cfg.ConsoleEnabled {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), enabler))
	}
	if cfg.FileEnabled {
		writer, err := NewDailyFileWriter(cfg.FileDir, cfg.FileName)
		if err != nil {
			return nil, err
		}
		cores = append(cores, zapcore.NewCore(newEncoder("json", newEncoderConfig()), zapcore.AddSync(writer), enabler))
	}
	if len(cores) == 0 {
		return zap.NewNop(), nil
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

func newEncoderConfig() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.StringDurationEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	return encoderCfg
}

func newEncoder(encoding string, cfg zapcore.EncoderConfig) zapcore.Encoder {
	if encoding == "console" {
		return zapcore.NewConsoleEncoder(cfg)
	}
	return zapcore.NewJSONEncoder(cfg)
}

func parseLevel(value string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info", "":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unsupported log level: %s", value)
	}
}

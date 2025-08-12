package log

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"slgGame/pkg/pool"
	"time"
)

type BaseLogger struct {
	*zap.Logger
	ch chan func()
}

func (a *BaseLogger) start() {
	for fn := range a.ch {
		fn()
	}
}

func (a *BaseLogger) addLog(f func()) {
	t := time.After(time.Second)
	select {
	case <-t:
	case a.ch <- f:
	}
}

var logger *BaseLogger

func init() {
	zapLevel := initZapLogLev("debug") //TODO: 暂时这样写，后续服务器搭建完成 通过配置获取

	encoderCfg := initJsonCfg()
	// 同时输出到文件和控制台
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), initSyncer("log/debug.log"), zapLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(os.Stdout), zapLevel),
	)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	logger = &BaseLogger{
		Logger: zapLogger,
		ch:     make(chan func(), 1024),
	}

	pool.Go(func(ctx context.Context) (interface{}, error) {
		logger.start()
		return nil, nil
	})
}

func initZapLogLev(level string) zapcore.Level {
	// 日志等级
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}
	return zapLevel
}

func initSyncer(logPath string) zapcore.WriteSyncer {
	// 日志文件滚动配置
	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100,  // MB
		MaxBackups: 30,   // 最多保留30个备份
		MaxAge:     7,    // 7天
		Compress:   true, // 启用压缩
	})
	return writeSyncer
}

func initJsonCfg() zapcore.EncoderConfig {
	// 编码器（JSON 格式）
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05.000000000"))
		},
		EncodeCaller: zapcore.ShortCallerEncoder,
	}
	return encoderConfig
}

// 快捷调用
func Debug(msg string, fields ...zap.Field) {
	logger.addLog(func() {
		logger.Debug(msg, fields...)
	})
}
func Info(msg string, fields ...zap.Field) {
	logger.addLog(func() { logger.Info(msg, fields...) })
}

func Warn(msg string, fields ...zap.Field) {
	logger.addLog(func() { logger.Warn(msg, fields...) })
}

func Error(msg string, fields ...zap.Field) {
	logger.addLog(func() {
		logger.Error(msg, fields...)
	})
}

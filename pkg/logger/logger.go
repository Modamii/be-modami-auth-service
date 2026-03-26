package logger

import (
	"context"

	logging "gitlab.com/lifegoeson-libs/pkg-logging"
	pkglogger "gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

func Init(cfg logging.Config) error {
	return pkglogger.Init(cfg)
}

func Shutdown(ctx context.Context) error {
	return pkglogger.Shutdown(ctx)
}

func L() logging.Logger {
	return pkglogger.L()
}

func FromContext(ctx context.Context) logging.Logger {
	return pkglogger.FromContext(ctx)
}

func Info(ctx context.Context, msg string, attrs ...logging.Attr) {
	pkglogger.Info(ctx, msg, attrs...)
}

func Warn(ctx context.Context, msg string, attrs ...logging.Attr) {
	pkglogger.Warn(ctx, msg, attrs...)
}

func Error(ctx context.Context, msg string, err error, attrs ...logging.Attr) {
	pkglogger.Error(ctx, msg, err, attrs...)
}

func Debug(ctx context.Context, msg string, attrs ...logging.Attr) {
	pkglogger.Debug(ctx, msg, attrs...)
}

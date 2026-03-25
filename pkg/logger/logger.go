package logger

import "go.uber.org/zap"

func New(level, format string) (*zap.Logger, error) {
	var cfg zap.Config
	if format == "console" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}

	return cfg.Build()
}

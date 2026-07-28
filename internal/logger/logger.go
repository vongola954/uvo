package logger

import (
	"github.com/sirupsen/logrus"
	"os"
)

type Config struct {
	Level      string
	OutputPath string
}

func Init(cfg *Config) {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)

	if cfg.OutputPath != "" {
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logrus.SetOutput(file)
		}
	}

	level, err := logrus.ParseLevel(cfg.Level)
	if err == nil {
		logrus.SetLevel(level)
	}
}
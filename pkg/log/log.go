package log

import "github.com/sirupsen/logrus"

var loggerInstance *logrus.Logger

func ConfigLogger(logLevel logrus.Level) *logrus.Logger {
	if loggerInstance == nil {
		logger := logrus.New()
		logger.SetLevel(logLevel)
		loggerInstance = logger
	}

	return loggerInstance
}

func Log() *logrus.Logger {
	return loggerInstance
}

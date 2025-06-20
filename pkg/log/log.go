package log

import "github.com/sirupsen/logrus"

var loggerInstance *logrus.Logger

func Configure(logLevel logrus.Level) {
	logrus.SetLevel(logLevel)
}

package logger

import (
	"go.uber.org/zap"
)

var Log *zap.SugaredLogger

func Init() error {
	var err error
	l, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	Log = l.Sugar()
	return nil
}

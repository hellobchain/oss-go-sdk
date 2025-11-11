package ossclient

import "fmt"

type Logger interface {
	Print(v ...interface{})
}

type DefaultLogger struct{}

func (d *DefaultLogger) Print(v ...interface{}) {
	fmt.Println(v...)
}

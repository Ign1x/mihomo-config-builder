package logging

import (
	"fmt"
	"io"
)

type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

type Logger struct {
	out io.Writer
	err io.Writer
}

func New(out io.Writer, err io.Writer) *Logger {
	return &Logger{out: out, err: err}
}

func (l *Logger) log(level string, w io.Writer, msg string, args ...any) {
	if len(args) > 0 {
		fmt.Fprintf(w, "[%s] %s\n", level, fmt.Sprintf(msg, args...))
		return
	}
	fmt.Fprintf(w, "[%s] %s\n", level, msg)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log("info", l.out, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log("warn", l.err, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.log("error", l.err, msg, args...)
}

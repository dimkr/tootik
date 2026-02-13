package main

import (
	"context"
	"fmt"
)

type t struct {
	ctx     context.Context
	tempDir string
}

func (t) Parallel() {}

func (t) Name() string {
	return "demo"
}

func (t t) Context() context.Context {
	return t.ctx
}

func (t t) TempDir() string {
	return t.tempDir
}

func (t) Fatal(args ...any) {
	panic(fmt.Sprint(args...))
}

func (t) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

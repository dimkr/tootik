/*
Copyright 2023 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logger

import (
	"github.com/dimkr/tootik/cfg"
	log "github.com/sirupsen/logrus"
	"os"
)

type formatter struct {
	formatter log.Formatter
	fields    log.Fields
}

func New(fields log.Fields) *log.Logger {
	l := log.Logger{
		Out:       os.Stderr,
		Formatter: &formatter{formatter: &log.JSONFormatter{}, fields: fields},
		Hooks:     log.LevelHooks{},
		Level:     log.Level(cfg.LogLevel),
		ExitFunc:  os.Exit,
	}

	if l.Level == log.DebugLevel {
		l.ReportCaller = true
	}

	return &l
}

func (f *formatter) Format(e *log.Entry) ([]byte, error) {
	for k, v := range f.fields {
		e.Data[k] = v
	}
	return f.formatter.Format(e)
}

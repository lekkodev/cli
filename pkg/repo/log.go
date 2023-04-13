// Copyright 2022 Lekko Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repo

import (
	"fmt"
	"io"

	"github.com/lekkodev/cli/pkg/logging"
)

// Allows customizing how the we log print statements and where we log them to.
type Logger interface {
	Logf(format string, a ...any)
	// Allows configuring the logger by temporarily overriding the
	// current logging configuration. Prior configuration is restored
	// by calling clear().
	ConfigureLogger(c *LoggingConfiguration) (clear func())
	// If colors are supported (e.g. in Bash), the methods below will customize
	// variables to be printed with the specified formatting applied.
	Bold(v interface{}) string
	Red(v interface{}) string
	Green(v interface{}) string
	Yellow(v interface{}) string
}

type LoggingConfiguration struct {
	Writer         io.Writer
	ColorsDisabled bool
}

func (r *repository) Logf(format string, a ...any) {
	if r.log == nil {
		return
	}
	fmt.Fprintf(r.log.Writer, format, a...)
}

func (r *repository) ConfigureLogger(c *LoggingConfiguration) (clear func()) {
	currentConfig := r.log
	r.log = c
	return func() { r.log = currentConfig }
}

func (r *repository) Bold(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return logging.Bold(v)
}

func (r *repository) Red(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return logging.Red(v)
}

func (r *repository) Green(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return logging.Green(v)
}

func (r *repository) Yellow(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return logging.Yellow(v)
}

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
)

type Logger interface {
	Logf(format string, a ...any)
	ConfigureLogger(c *LoggingConfiguration) (clear func())
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
	return r.Bold(v)
}

func (r *repository) Red(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return r.Red(v)
}

func (r *repository) Green(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return r.Green(v)
}

func (r *repository) Yellow(v interface{}) string {
	if r.log == nil || r.log.ColorsDisabled {
		return fmt.Sprintf("%s", v)
	}
	return r.Yellow(v)
}

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

package logging

import (
	"fmt"
	"runtime"
)

// Source: https://twin.sh/articles/35/how-to-add-colors-to-your-console-terminal-output-in-go

var reset = "\033[0m"
var red = "\033[31m"
var green = "\033[32m"
var bold = "\033[1m"

func InitColors() {
	if runtime.GOOS == "windows" {
		reset = ""
		red = ""
		green = ""
		bold = ""
	}
}

func Bold(v interface{}) string {
	return fmt.Sprintf("%s%v%s", bold, v, reset)
}

func Red(v interface{}) string {
	return fmt.Sprintf("%s%s%s", red, v, reset)
}

func Green(v interface{}) string {
	return fmt.Sprintf("%s%s%s", green, v, reset)
}

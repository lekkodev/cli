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

import "runtime"

// Source: https://twin.sh/articles/35/how-to-add-colors-to-your-console-terminal-output-in-go

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Bold = "\033[1m"

func InitColors() {
	if runtime.GOOS == "windows" {
		Reset = ""
		Red = ""
		Green = ""
		Bold = ""
	}
}
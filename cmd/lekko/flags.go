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

package main

const (
	QuietModeFlag      = "quiet-mode"
	QuietModeFlagShort = "q"
	QuietModeFlagDVal  = false

	DryRunFlag      = "dry-run"
	DryRunFlagShort = "d"
	DryRunFlagDVal  = false

	ForceFlag      = "force"
	ForceFlagShort = "f"
	ForceFlagDVal  = false

	InteractiveModeFlag      = "interactive"
	InteractiveModeFlagShort = "i"

	VerboseFlag      = "verbose"
	VerboseFlagShort = "v"

	VerboseFlagDVal = false

	LekkoBackendFlag            = "backend-url"
	LekkoBackendFlagDescription = "Lekko backend url"
	LekkoBackendFlagURL         = "https://prod.api.lekko.dev"
	IsLekkoBackendFlagHidden    = true
)
const (
	FlagOptions = "[FLAGS]"
)

var InteractiveModeFlagDVal = true
var IsInteractive = true
var QuietModeFlagDescription string = "prints without formatting or extra information"
var DryRunFlagDescription string = "does not persist results of the executed command"
var ForceFlagDescription string = "forces all user confirmation evaluations to true"
var InteractiveModeFlagDescription string = "supports functionality for interactive terminals"
var VerboseFlagDescription = "verbose output"

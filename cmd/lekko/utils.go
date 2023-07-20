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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"

	"github.com/lekkodev/cli/pkg/logging"
	"github.com/spf13/cobra"
)

// Helpful method to ask the user to enter a piece of text before
// doing something irreversible, like deleting something.
func confirmInput(text string) error {
	var inputText string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("Enter '%s' to continue:", text),
	}, &inputText); err != nil {
		return errors.Wrap(err, "prompt")
	}
	if text != inputText {
		return errors.New("incorrect input")
	}
	return nil
}

// printLinef prints according to provided format, removing leading and trailing
// white space characters in quiet mode. It is intended to print single line for print-outs
// with formatting done only in the format string
func printLinef(cmd *cobra.Command, format string, params ...interface{}) {
	if isDryRun, _ := cmd.Flags().GetBool(DryRunFlag); isDryRun {
		format = "[Dry Run]: " + format
	}
	if isQuiet, _ := cmd.Flags().GetBool(QuietModeFlag); isQuiet {
		format = strings.TrimSpace(format)
	}
	fmt.Printf(format, params...)
}

// printErr prints error message on stderr, using in bold and red (on Mac OS).
// In non-quiet mode, newline is added.
func printErr(cmd *cobra.Command, err error) {
	if isQuiet, _ := cmd.Flags().GetBool(QuietModeFlag); !isQuiet {
		_, _ = fmt.Fprintln(os.Stderr, logging.Bold(logging.Red("Error: "+err.Error())))
	} else {
		_, _ = fmt.Fprint(os.Stderr, logging.Bold(logging.Red("Error: "+err.Error())))
	}
}

func formCmdUse(cmd string, args ...string) string {
	return fmt.Sprintf("%s %s %s", cmd, FlagOptions, strings.Join(args, " "))
}
func printErrExit(cmd *cobra.Command, err error) {
	printErr(cmd, err)
	os.Exit(1)
}

func getNArgs(nArgs int, args []string) (retArgs []string, n int) {
	maxN := len(args)
	if maxN < nArgs {
		maxN = nArgs
	}
	retArgs = make([]string, maxN)
	n = len(args)
	for i := 0; i < n; i++ {
		retArgs[i] = args[i]
	}
	//fmt.Printf("\n*%v*\n", retArgs)
	return retArgs, n
}

// splits string into a slice of desired capacity
func splitStrIntoFixedSlice(str string, tDel string, num int) (rs []string, err error) {
	if !strings.Contains(str, tDel) {
		return rs, fmt.Errorf("required token delimiter '%s' not found in %s", tDel, str)
	}
	rs = make([]string, num)
	ss := strings.Split(str, tDel)
	for i, j := 0, 0; i < len(rs) && i < len(ss); i, j = i+1, j+1 {
		rs[i] = ss[j]
	}
	return rs, err
}

func errIfMoreArgs(eArgs, args []string) error {
	if len(eArgs) < len(args) {
		if len(eArgs) == 0 {
			return errors.New(fmt.Sprintf("wrong number of arguments - no args expected, received %d (%s)", len(args), strings.Join(args, " ")))
		} else {
			return errors.New(fmt.Sprintf("wrong number of arguments - expected %d (%s), received %d (%s)", len(eArgs), strings.Join(eArgs, " "), len(args), strings.Join(args, " ")))
		}
	}
	return nil
}

func errIfLessArgs(eArgs, args []string) error {
	if len(eArgs) > len(args) {
		argsStr := ""
		if len(args) > 0 {
			argsStr = strings.Join(args, " ")
		}
		if len(argsStr) > 0 {
			argsStr = "(" + argsStr + ")"
		}
		return errors.New(fmt.Sprintf("wrong number of arguments - required %d (%s), received %d args %s", len(eArgs), strings.Join(eArgs, " "), len(args), argsStr))
	}
	return nil
}

type UseCmdParams struct {
	App             string
	Name            string
	Cmds            []string
	PosArgs         []string
	PosOptionalArgs []string
	FlagOptions     string
}

func getUseCmdParams(useLine, cmdName string) *UseCmdParams {
	var useCmdParams UseCmdParams
	useCmdParams.Cmds = []string{}
	useCmdParams.PosArgs = []string{}
	useCmdParams.PosOptionalArgs = []string{}

	useLineArr := strings.Split(useLine, " ")
	useCmdParams.App = useLineArr[0]
	useCmdParams.Name = cmdName
	index := 1
	useLineArr = useLineArr[index:]

	var p string
	for _, p = range useLineArr {
		useCmdParams.Cmds = append(useCmdParams.Cmds, p)
		index++
		if p == cmdName {
			break
		}
	}

	useLineArr = useLineArr[index:]
	for _, p = range useLineArr {
		if p == FlagOptions {
			useCmdParams.FlagOptions = p
		} else if strings.Contains(p, "[") && strings.Contains(p, "]") {
			useCmdParams.PosOptionalArgs = append(useCmdParams.PosOptionalArgs, p)
		} else if len(p) > 0 {
			useCmdParams.PosArgs = append(useCmdParams.PosArgs, p)
		}
	}
	return &useCmdParams
}

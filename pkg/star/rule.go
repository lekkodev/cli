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

package star

import (
	"github.com/nikunjy/rules/parser"
)

// Validates that a rules lang string is valid by parsing and traversing it.
//
// For now, this validates the string as a rule created in the lekkodev/rules fork
// of nikunjy/rules lib. This will be our own custom rules in the future,
// and we will need to add versioning somehow.
//
// Returns raw errors from the rules library, which may contain panics.
// TODO: cleanup error representation.
func validateRulesLang(rulesStr string) error {
	evaluator, err := parser.NewEvaluator(rulesStr)
	if err != nil {
		return err
	}

	// Process with an empty context to make sure we can traverse the
	// tree properly.
	if _, err := evaluator.Process(map[string]interface{}{}); err != nil {
		return err
	}

	return nil
}

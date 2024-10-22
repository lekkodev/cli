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

package sync

import "fmt"

type SyncError struct {
	Inner error
}

func NewSyncError(inner error) *SyncError {
	return &SyncError{
		Inner: inner,
	}
}

func (e *SyncError) Error() string {
	return fmt.Sprintf("sync: %v", e.Inner)
}

func (e *SyncError) Unwrap() error {
	return e.Inner
}

// Error that can be directly attributed to incorrect/unsupported code
type SyncPosError struct {
	Inner     error
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

func NewSyncPosError(inner error, filename string, startLine, startCol, endLine, endCol int) *SyncPosError {
	return &SyncPosError{
		Inner:     inner,
		Filename:  filename,
		StartLine: startLine,
		StartCol:  startCol,
		EndLine:   endLine,
		EndCol:    endCol,
	}
}

func (e *SyncPosError) Error() string {
	return fmt.Sprintf("sync %s:%d:%d:%d:%d: %v", e.Filename, e.StartLine, e.StartCol, e.EndLine, e.EndCol, e.Inner)
}

func (e *SyncPosError) Unwrap() error {
	return e.Inner
}

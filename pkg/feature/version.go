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

package feature

import (
	"errors"
)

type NamespaceVersion string

const (
	NamespaceVersionV1Beta1 NamespaceVersion = "v1beta1"
	NamespaceVersionV1Beta2 NamespaceVersion = "v1beta2"
	NamespaceVersionV1Beta3 NamespaceVersion = "v1beta3"
	// Supports generating n-ary rules AST
	NamespaceVersionV1Beta4 NamespaceVersion = "v1beta4"
	// Supports != operator
	NamespaceVersionV1Beta5 NamespaceVersion = "v1beta5"
	// Supports using "overrides" instead of "rules" and "export(Config(...))" instead of "result = feature(...)" (dual support)
	NamespaceVersionV1Beta6 NamespaceVersion = "v1beta6"
)

var (
	ErrDeprecatedVersion error = errors.New("namespace version is deprecated, please upgrade")
	ErrUnknownVersion    error = errors.New("namespace version is unknown")
)

func NewNamespaceVersion(v string) NamespaceVersion {
	return NamespaceVersion(v)
}

// Returns all namespace versions in the order that they were released
func AllNamespaceVersions() []NamespaceVersion {
	return []NamespaceVersion{
		NamespaceVersionV1Beta1,
		NamespaceVersionV1Beta2,
		NamespaceVersionV1Beta3,
		NamespaceVersionV1Beta4,
		NamespaceVersionV1Beta5,
		NamespaceVersionV1Beta6,
	}
}

// Returns the list of namespace versions that are supported.
func SupportedNamespaceVersions() []NamespaceVersion {
	all := AllNamespaceVersions()
	start := 0
	for i, v := range all {
		if v == NamespaceVersionV1Beta3 { // First supported version
			start = i
			break
		}
	}
	return all[start:]
}

// Returns the latest namespace version.
func LatestNamespaceVersion() NamespaceVersion {
	all := SupportedNamespaceVersions()
	return all[len(all)-1]
}

func (nv NamespaceVersion) IsLatest() bool {
	return nv == LatestNamespaceVersion()
}

func (nv NamespaceVersion) Supported() error {
	all := SupportedNamespaceVersions()
	for _, v := range all {
		if v == nv {
			return nil
		}
	}
	if nv.Before(all[0]) {
		return ErrDeprecatedVersion
	}
	return ErrUnknownVersion
}

func (nv NamespaceVersion) Before(cmp NamespaceVersion) bool {
	myIdx, cmpIdx := -1, -1
	for i, v := range AllNamespaceVersions() {
		if v == nv {
			myIdx = i
		}
		if v == cmp {
			cmpIdx = i
		}
	}
	return myIdx >= 0 && myIdx < cmpIdx
}

func (nv NamespaceVersion) String() string {
	return string(nv)
}

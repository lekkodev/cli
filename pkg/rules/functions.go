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

package rules

import (
	"bytes"
	"encoding/binary"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/OneOfOne/xxhash"
	"github.com/pkg/errors"
)

// If the hashed feature value % 100 <= threshold, it fits in the "bucket".
// In reality, we internally store the threshold as an integer in [0,100000]
// to account for up to 3 decimal places.
// The feature value is salted using the namespace, feature name, and context key.
func (v1b3 *v1beta3) evaluateBucket(bucketF *rulesv1beta3.CallExpression_Bucket, featureCtx map[string]interface{}) (bool, error) {
	ctxKey := bucketF.ContextKey
	value, present := featureCtx[ctxKey]
	// If key is missing in context map, evaluate to false - move to next rule
	if !present {
		return false, nil
	}
	var valueBytes []byte
	var err error
	bytesBuffer := new(bytes.Buffer)
	switch typedValue := value.(type) {
	case int:
		err = binary.Write(bytesBuffer, binary.BigEndian, int64(typedValue))
	case int8:
		err = binary.Write(bytesBuffer, binary.BigEndian, int64(typedValue))
	case int16:
		err = binary.Write(bytesBuffer, binary.BigEndian, int64(typedValue))
	case int32:
		err = binary.Write(bytesBuffer, binary.BigEndian, int64(typedValue))
	case int64:
		err = binary.Write(bytesBuffer, binary.BigEndian, typedValue)
	case float32:
		err = binary.Write(bytesBuffer, binary.BigEndian, float64(typedValue))
	case float64:
		err = binary.Write(bytesBuffer, binary.BigEndian, typedValue)
	case string:
		valueBytes = []byte(typedValue)
	default:
		return false, errors.Errorf("unsupported value type for bucket: %T", value)
	}
	if err != nil {
		return false, err
	}

	if len(valueBytes) == 0 {
		valueBytes = bytesBuffer.Bytes()
	}

	bytesFrags := [][]byte{
		[]byte(v1b3.evalContext.Namespace),
		[]byte(v1b3.evalContext.FeatureName),
		[]byte(ctxKey),
		valueBytes,
	}

	// Hash namespace -> feature name -> context key -> value
	hash := xxhash.NewS32(0)
	for _, bytesFrag := range bytesFrags {
		_, err := hash.Write(bytesFrag)
		if err != nil {
			return false, err
		}
	}

	return hash.Sum32()%100000 <= bucketF.Threshold, nil
}

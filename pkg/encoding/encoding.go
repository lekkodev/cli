package encoding

import (
	"fmt"

	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"

	"google.golang.org/protobuf/encoding/protojson"
)

// Takes a version number and parses file contents into the corresponding
// type.
// TODO have this based on some sort of common internal representation.
func ParseFeature(contents []byte, version string) (*rulesv1beta1.Feature, error) {
	switch version {
	case "v1beta1":
		var feature rulesv1beta1.Feature
		err := protojson.Unmarshal(contents, &feature)
		if err != nil {
			return nil, err
		}
		return &feature, nil
	default:
		return nil, fmt.Errorf("unknown version when parsing feature: %s", version)
	}
}

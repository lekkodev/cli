package repo

import (
	"context"
	"fmt"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

func (r *Repo) GetFeatureFiles(ctx context.Context, namespace string) ([]feature.FeatureFile, error) {
	_, nsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, "", r)
	if err != nil {
		return nil, errors.Wrap(err, "parse root md")
	}
	nsMD, ok := nsMDs[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found in root metadata", namespace)
	}
	ffs, err := feature.GroupFeatureFiles(ctx, namespace, nsMD, r, true)
	if err != nil {
		return nil, errors.Wrap(err, "group feature files")
	}
	return ffs, nil
}

func (r *Repo) GetFeatureFile(ctx context.Context, namespace, featureName string) (*feature.FeatureFile, error) {
	ffs, err := r.GetFeatureFiles(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "get feature files")
	}
	var ff *feature.FeatureFile
	for _, file := range ffs {
		if file.Name == featureName {
			ff = &file
			break
		}
	}
	if ff == nil {
		return nil, fmt.Errorf("feature '%s' not found in namespace '%s'", featureName, namespace)
	}
	return ff, nil
}

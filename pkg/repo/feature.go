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

package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/lekkodev/cli/pkg/encoding"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

// func (r *Repo) Compile(ctx context.Context, registry *protoregistry.Types, force bool) error {
// 	var err error
// 	if registry == nil {
// 		rootMD, _, err := r.ParseMetadata(ctx)
// 		if err != nil {
// 			return errors.Wrap(err, "parse metadata")
// 		}
// 		registry, err = r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
// 		if err != nil {
// 			return errors.Wrap(err, "build dynamic type registry")
// 		}
// 	}
// 	contents, err := r.GetContents(ctx)
// 	if err != nil {
// 		return errors.Wrap(err, "get contents")
// 	}
// 	for nsMD, ffs := range contents {
// 		for _, ff := range ffs {
// 			if _, err := r.CompileFeature(ctx, registry, nsMD.Name, ff.Name, true, force); err != nil {
// 				return errors.Wrap(err, "compile feature")
// 			}
// 		}
// 	}
// 	return nil
// }

// func (r *Repo) CompileNamespace(ctx context.Context, registry *protoregistry.Types, namespace string, force bool) error {
// 	ffs, err := r.GetFeatureFiles(ctx, namespace)
// 	if err != nil {
// 		return errors.Wrap(err, "get feature files")
// 	}
// 	if registry == nil {
// 		rootMD, _, err := r.ParseMetadata(ctx)
// 		if err != nil {
// 			return errors.Wrap(err, "parse metadata")
// 		}
// 		registry, err = r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
// 		if err != nil {
// 			return errors.Wrap(err, "build dynamic type registry")
// 		}
// 	}
// 	for _, ff := range ffs {
// 		if _, err := r.CompileFeature(ctx, registry, namespace, ff.Name, true, force); err != nil {
// 			return errors.Wrap(err, "compile feature")
// 		}
// 	}
// 	return nil
// }

func (r *Repo) CompileFeature(ctx context.Context, registry *protoregistry.Types, namespace, featureName string) (*feature.CompiledFeature, error) {
	ff := feature.NewFeatureFile(namespace, featureName)
	registry, err := r.registry(ctx, registry)
	if err != nil {
		return nil, errors.Wrap(err, "registry")
	}
	compiler := star.NewCompiler(registry, &ff, r)
	f, err := compiler.Compile(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "compile")
	}
	return f, nil
}

func (r *Repo) PersistFeature(ctx context.Context, registry *protoregistry.Types, namespace string, f *feature.Feature, force bool) error {
	registry, err := r.registry(ctx, registry)
	if err != nil {
		return errors.Wrap(err, "registry")
	}
	ff := feature.NewFeatureFile(namespace, f.Key)
	compiler := star.NewCompiler(registry, &ff, r)
	persisted, err := compiler.Persist(ctx, f, force)
	if err != nil {
		return errors.Wrap(err, "persist")
	}
	if persisted {
		r.Logf("Generated diff for %s/%s", namespace, f.Key)
	}
	if err := r.FormatFeature(ctx, ff); err != nil {
		return errors.Wrap(err, "format")
	}
	return nil
}

type CompileRequest struct {
	// Registry of protobuf types. This field is optional, if it does not exist, it will be instantiated.
	Registry *protoregistry.Types
	// Optional fields to filter features by, so as to not compile the entire world.
	NamespaceFilter, FeatureFilter string
	// Whether or not to persist the successfully compiled features
	Persist bool
	// If true, any generated compilation changes will overwrite previous features
	// even if there are type mismatches
	IgnoreBackwardsCompatibility bool
}

type FeatureCompilationResult struct {
	NamespaceName    string
	FeatureName      string
	CompiledFeature  *feature.CompiledFeature
	CompilationError error
}

func (fcr *FeatureCompilationResult) Debug() (ret string) {
	lines := []string{fmt.Sprintf("%s[%s/%s]%s %s", bold, fcr.NamespaceName, fcr.FeatureName, reset, fcr.SummaryString())}
	defer func() {
		ret = strings.Join(lines, "\n")
	}()
	if fcr.CompilationError != nil {
		lines = append(lines, fmt.Sprintf("%v", fcr.CompilationError))
		return
	}
	for _, vr := range fcr.CompiledFeature.ValidatorResults {
		if !vr.Passed() {
			lines = append(lines, fmt.Sprintf("\t%s: %v", vr.Key, vr.Error))
		}
	}
	for _, tr := range fcr.CompiledFeature.TestResults {
		if !tr.Passed() {
			lines = append(lines, fmt.Sprintf("\t%s: %v", tr.Key, tr.Error))
		}
	}
	return
}

func (fcr *FeatureCompilationResult) SummaryString() string {
	stylizeStr := func(s string, pass bool) string {
		if pass {
			return fmt.Sprintf("%s%s %s%s", green, s, "✔", reset)
		}
		return fmt.Sprintf("%s%s %s%s", red, s, "✖", reset)
	}
	if fcr.CompilationError != nil {
		return stylizeStr("Compile", false)
	}
	subs := []string{stylizeStr("Compile", true)}
	if len(fcr.CompiledFeature.ValidatorResults) > 0 {
		var numPassed int
		for _, vr := range fcr.CompiledFeature.ValidatorResults {
			if vr.Passed() {
				numPassed++
			}
		}
		subs = append(subs, stylizeStr(fmt.Sprintf("Validate %d/%d", numPassed, len(fcr.CompiledFeature.ValidatorResults)), numPassed == len(fcr.CompiledFeature.ValidatorResults)))
	}
	if len(fcr.CompiledFeature.TestResults) > 0 {
		var numPassed int
		for _, tr := range fcr.CompiledFeature.TestResults {
			if tr.Passed() {
				numPassed++
			}
		}
		subs = append(subs, stylizeStr(fmt.Sprintf("Test %d/%d", numPassed, len(fcr.CompiledFeature.TestResults)), numPassed == len(fcr.CompiledFeature.TestResults)))
	}
	return strings.Join(subs, " | ")
}

func (fcr *FeatureCompilationResult) Err() error {
	if fcr.CompilationError != nil {
		return fcr.CompilationError
	}
	for _, r := range fcr.CompiledFeature.ValidatorResults {
		if !r.Passed() {
			return r.Error
		}
	}
	for _, r := range fcr.CompiledFeature.TestResults {
		if !r.Passed() {
			return r.Error
		}
	}
	return nil
}

func (r *Repo) Compile(ctx context.Context, req *CompileRequest) ([]*FeatureCompilationResult, error) {
	// Step 1: collect. Find all features
	ffs, numNamespaces, err := r.FindFeatureFiles(ctx, req.NamespaceFilter, req.FeatureFilter, true)
	if err != nil {
		return nil, errors.Wrap(err, "find features")
	}
	var results []*FeatureCompilationResult
	for _, ff := range ffs {
		results = append(results, &FeatureCompilationResult{NamespaceName: ff.NamespaceName, FeatureName: ff.Name})
	}
	r.Logf("Found %d features across %d namespaces\n", len(ffs), numNamespaces)
	r.Logf("Compiling...\n")
	registry, err := r.registry(ctx, req.Registry)
	if err != nil {
		return nil, errors.Wrap(err, "registry")
	}
	concurrency := 50
	if len(results) < 50 {
		concurrency = len(results)
	}
	var wg sync.WaitGroup
	featureChan := make(chan *FeatureCompilationResult)
	// start compile workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case fcr, ok := <-featureChan:
					if !ok {
						return
					}
					cf, err := r.CompileFeature(ctx, registry, fcr.NamespaceName, fcr.FeatureName)
					fcr.CompiledFeature = cf
					fcr.CompilationError = err
				}
			}
		}()
	}

	for _, fcr := range results {
		featureChan <- fcr
	}
	close(featureChan)
	wg.Wait()

	// print results
	sort.Slice(results, func(i, j int) bool {
		if results[i].NamespaceName < results[j].NamespaceName {
			return true
		} else if results[i].NamespaceName > results[j].NamespaceName {
			return false
		} else {
			return results[i].FeatureName < results[j].FeatureName
		}
	})
	var compileErr error
	for _, fcr := range results {
		r.Logf("%v\n", fcr.Debug())
		if fcr.Err() != nil {
			compileErr = fcr.Err()
		}
	}
	if compileErr != nil {
		r.Logf("Failed compilation, exiting...")
		return results, compileErr
	}

	if req.Persist {
		r.Logf("\n")
		for _, fcr := range results {
			if err := r.PersistFeature(ctx, registry, fcr.NamespaceName, fcr.CompiledFeature.Feature, req.IgnoreBackwardsCompatibility); err != nil {
				return nil, errors.Wrapf(err, "persist feature %s/%s", fcr.NamespaceName, fcr.FeatureName)
			}
		}
	}

	return results, nil
}

func (r *Repo) FindFeatureFiles(ctx context.Context, namespaceFilter, featureFilter string, verify bool) ([]*feature.FeatureFile, int, error) {
	contents, err := r.GetContents(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "get contents")
	}
	var numNamespaces int
	var results []*feature.FeatureFile
	for nsMD, ffs := range contents {
		if len(namespaceFilter) > 0 && nsMD.Name != namespaceFilter {
			continue
		}
		numNamespaces++
		for _, ff := range ffs {
			ff := ff
			if len(featureFilter) > 0 && ff.Name != featureFilter {
				continue
			}
			if verify {
				if err := ff.Verify(); err != nil {
					return nil, 0, errors.Wrapf(err, "feature %s/%s verify", ff.NamespaceName, ff.Name)
				}
				if err := feature.ComplianceCheck(ff, &nsMD); err != nil {
					return nil, 0, errors.Wrapf(err, "feature %s/%s compliance check", ff.NamespaceName, ff.Name)
				}
			}
			results = append(results, &ff)
		}
	}
	return results, numNamespaces, nil
}

func (r *Repo) registry(ctx context.Context, registry *protoregistry.Types) (*protoregistry.Types, error) {
	if registry != nil {
		return registry, nil
	}
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse metadata")
	}
	registry, err = r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return nil, errors.Wrap(err, "build dynamic type registry")
	}
	return registry, nil
}

func (r *Repo) BuildDynamicTypeRegistry(ctx context.Context, protoDirPath string) (*protoregistry.Types, error) {
	return star.BuildDynamicTypeRegistry(ctx, protoDirPath, r)
}

// Actually regenerates the buf image, and writes it to the file system.
// Note: we don't have a way yet to run this from an ephemeral repo,
// because we need to first ensure that buf cmd line can be executed in the
// ephemeral env.
func (r *Repo) ReBuildDynamicTypeRegistry(ctx context.Context, protoDirPath string) (*protoregistry.Types, error) {
	if !r.bufEnabled {
		return nil, errors.New("buf cmd line not enabled")
	}
	return star.ReBuildDynamicTypeRegistry(ctx, protoDirPath, r)
}

func (r *Repo) Format(ctx context.Context) error {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "parse metadata")
	}
	for ns, nsMD := range nsMDs {
		if nsMD.Version != metadata.LatestNamespaceVersion {
			r.Logf("Skipping namespace %s since version %s doesn't conform to compilation\n", ns, nsMD.Version)
			continue
		}
		ffs, err := r.GetFeatureFiles(ctx, ns)
		if err != nil {
			return errors.Wrap(err, "get feature files")
		}

		for _, ff := range ffs {
			if err := r.FormatFeature(ctx, ff); err != nil {
				return errors.Wrapf(err, "format feature '%s/%s", ff.NamespaceName, ff.Name)
			}
		}
	}
	return nil
}

func (r *Repo) FormatFeature(ctx context.Context, ff feature.FeatureFile) error {
	formatter := star.NewStarFormatter(ff.RootPath(ff.StarlarkFileName), ff.Name, r)
	ok, err := formatter.Format(ctx)
	if err != nil {
		return errors.Wrap(err, "star format")
	}
	if ok {
		r.Logf("Formatted and rewrote %s/%s\n", ff.NamespaceName, ff.Name)
	}
	return nil
}

func (r *Repo) Add(ctx context.Context, ns, featureName string, fType feature.FeatureType) error {
	nsMD, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := metadata.CreateNamespaceMetadata(ctx, "", ns, r); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	if featureName == "" {
		r.Logf("Your new namespace has been created: %s\n", ns)
		return nil
	}
	ffs, err := r.GetFeatureFiles(ctx, ns)
	if err != nil {
		return fmt.Errorf("failed to get feature files: %v", err)
	}
	for _, ff := range ffs {
		if err := feature.ComplianceCheck(ff, nsMD); err != nil {
			return fmt.Errorf("compliance check for feature %s: %w", ff.Name, err)
		}
		if ff.Name == featureName {
			return fmt.Errorf("feature named %s already exists", featureName)
		}
	}

	featurePath := filepath.Join(ns, fmt.Sprintf("%s.star", featureName))
	template, err := star.GetTemplate(fType)
	if err != nil {
		return errors.Wrap(err, "get template")
	}
	if err := r.WriteFile(featurePath, template, 0600); err != nil {
		return fmt.Errorf("failed to add feature: %v", err)
	}
	r.Logf("Your new feature has been written to %s\n", featurePath)
	r.Logf("Make your changes, and run 'lekko compile'.\n")
	return nil
}

func (r *Repo) RemoveFeature(ctx context.Context, ns, featureName string) error {
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	var removed bool
	for _, file := range []string{
		fmt.Sprintf("%s.star", featureName),
		filepath.Join(metadata.GenFolderPathJSON, fmt.Sprintf("%s.json", featureName)),
		filepath.Join(metadata.GenFolderPathProto, fmt.Sprintf("%s.proto.bin", featureName)),
	} {
		ok, err := r.RemoveIfExists(filepath.Join(ns, file))
		if err != nil {
			return fmt.Errorf("remove if exists failed to remove %s: %v", file, err)
		}
		if ok {
			removed = true
		}
	}
	if removed {
		r.Logf("Feature %s has been removed\n", featureName)
	} else {
		r.Logf("Feature %s does not exist\n", featureName)
	}
	return nil
}

func (r *Repo) RemoveNamespace(ctx context.Context, ns string) error {
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	ok, err := r.RemoveIfExists(ns)
	if err != nil {
		return fmt.Errorf("failed to remove namespace %s: %v", ns, err)
	}
	if !ok {
		r.Logf("Namespace %s does not exist\n", ns)
	}
	if err := metadata.UpdateRootConfigRepoMetadata(ctx, "", r, func(rcrm *metadata.RootConfigRepoMetadata) {
		var updatedNamespaces []string
		for _, n := range rcrm.Namespaces {
			if n != ns {
				updatedNamespaces = append(updatedNamespaces, n)
			}
		}
		rcrm.Namespaces = updatedNamespaces
	}); err != nil {
		return fmt.Errorf("failed to update root config md: %v", err)
	}
	r.Logf("Namespace %s has been removed\n", ns)
	return nil
}

func (r *Repo) Eval(ctx context.Context, ns, featureName string, iCtx map[string]interface{}) (*anypb.Any, error) {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse metadata")
	}
	nsMD, ok := nsMDs[ns]
	if !ok {
		return nil, fmt.Errorf("invalid namespace: %s", ns)
	}

	ff, err := r.GetFeatureFile(ctx, ns, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get feature file")
	}

	if err := feature.ComplianceCheck(*ff, nsMD); err != nil {
		return nil, errors.Wrap(err, "compliance check")
	}

	evalF, err := encoding.ParseFeature(ctx, "", *ff, nsMD, r)
	if err != nil {
		return nil, err
	}

	ret, _, err := evalF.Evaluate(iCtx)
	return ret, err
}

func (r *Repo) Parse(ctx context.Context, ns, featureName string) error {
	fc, err := r.GetFeatureContents(ctx, ns, featureName)
	if err != nil {
		return errors.Wrap(err, "get feature contents")
	}
	filename := fc.File.RootPath(fc.File.StarlarkFileName)
	w := static.NewWalker(filename, fc.Star)
	f, err := w.Build()
	if err != nil {
		return errors.Wrap(err, "build")
	}
	f.Key = fc.File.Name

	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "parse metadata")
	}
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return errors.Wrap(err, "build dynamic type registry")
	}
	// Print json to stdout
	f.PrintJSON(registry)
	// Rewrite the bytes to the starfile path, based on the parse AST.
	// This is just an illustration, but in the future we could modify
	// the feature and use the following code to write it out.
	bytes, err := w.Mutate(f)
	if err != nil {
		return errors.Wrap(err, "mutate")
	}
	if err := r.WriteFile(filename, bytes, 0600); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	return nil
}

func (r *Repo) GetContents(ctx context.Context) (map[metadata.NamespaceConfigRepoMetadata][]feature.FeatureFile, error) {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse root md")
	}
	ret := make(map[metadata.NamespaceConfigRepoMetadata][]feature.FeatureFile, len(nsMDs))
	for namespace, nsMD := range nsMDs {
		ffs, err := r.GetFeatureFiles(ctx, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "get feature files")
		}
		ret[*nsMD] = ffs
	}
	return ret, nil
}

func (r *Repo) GetFeatureFiles(ctx context.Context, namespace string) ([]feature.FeatureFile, error) {
	ffs, err := feature.GroupFeatureFiles(ctx, namespace, r)
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
		file := file
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

func (r *Repo) GetFeatureContents(ctx context.Context, namespace, featureName string) (*feature.FeatureContents, error) {
	ff, err := r.GetFeatureFile(ctx, namespace, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get feature file")
	}
	star, err := r.Read(filepath.Join(namespace, ff.StarlarkFileName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read star bytes")
	}
	json, err := r.Read(filepath.Join(namespace, ff.CompiledJSONFileName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json bytes")
	}
	proto, err := r.Read(filepath.Join(namespace, ff.CompiledProtoBinFileName))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read proto bytes")
	}
	computedHash := plumbing.ComputeHash(plumbing.BlobObject, proto)
	return &feature.FeatureContents{
		File:  ff,
		Star:  star,
		JSON:  json,
		Proto: proto,
		SHA:   computedHash.String(),
	}, nil
}

// Returns the hash of the proto bin file as indexed by the git tree. If there are
// uncommitted changes those won't be accounted for in the hash.
// The method is a reference, so I don't forget how to do this in the future (shubhit)
func (r *Repo) GetFeatureHash(ctx context.Context, namespace, featureName string) (*plumbing.Hash, error) {
	ff, err := r.GetFeatureFile(ctx, namespace, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get feature file")
	}
	hash, err := r.WorkingDirectoryHash()
	if err != nil {
		return nil, errors.Wrap(err, "working directory hash")
	}
	co, err := r.Repo.CommitObject(*hash)
	if err != nil {
		return nil, errors.Wrapf(err, "commit object of hash %s", hash.String())
	}
	protoPath := filepath.Join(namespace, ff.CompiledProtoBinFileName)
	fi, err := co.File(protoPath)
	if err != nil {
		return nil, errors.Wrapf(err, "commit object file '%s'", protoPath)
	}
	return &fi.Hash, nil
}

func (r *Repo) ParseMetadata(ctx context.Context) (*metadata.RootConfigRepoMetadata, map[string]*metadata.NamespaceConfigRepoMetadata, error) {
	return metadata.ParseFullConfigRepoMetadataStrict(ctx, "", r)
}

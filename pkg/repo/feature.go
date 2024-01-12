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
	"regexp"
	"sort"
	"strings"
	"sync"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/lekkodev/cli/pkg/encoding"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/prototypes"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Provides functionality needed for accessing and making changes to Lekko configuration.
// This interface should make no assumptions about where the configuration is stored.
type ConfigurationStore interface {
	Compile(ctx context.Context, req *CompileRequest) ([]*FeatureCompilationResult, error)
	Verify(ctx context.Context, req *VerifyRequest) error
	BuildDynamicTypeRegistry(ctx context.Context, protoDirPath string) (*protoregistry.Types, error)
	ReBuildDynamicTypeRegistry(ctx context.Context, protoDirPath string, useExternalTypes bool) (*protoregistry.Types, error)
	GetFileDescriptorSet(ctx context.Context, protoDirPath string) (*descriptorpb.FileDescriptorSet, error)
	Format(ctx context.Context, verbose bool) error
	AddFeature(ctx context.Context, ns, featureName string, fType eval.ConfigType, protoMessageName string) (string, error)
	RemoveFeature(ctx context.Context, ns, featureName string) error
	AddNamespace(ctx context.Context, name string) error
	RemoveNamespace(ctx context.Context, ns string) error
	Eval(ctx context.Context, ns, featureName string, featureCtx map[string]interface{}) (*anypb.Any, eval.ConfigType, eval.ResultPath, error)
	Parse(ctx context.Context, ns, featureName string, registry *protoregistry.Types) (*featurev1beta1.StaticFeature, error)
	GetContents(ctx context.Context) (map[metadata.NamespaceConfigRepoMetadata][]feature.FeatureFile, error)
	ListNamespaces(ctx context.Context) ([]*metadata.NamespaceConfigRepoMetadata, error)
	GetFeatureFiles(ctx context.Context, namespace string) ([]feature.FeatureFile, error)
	GetFeatureFile(ctx context.Context, namespace, featureName string) (*feature.FeatureFile, error)
	GetFeatureContents(ctx context.Context, namespace, featureName string) (*feature.FeatureContents, error)
	GetFeatureHash(ctx context.Context, namespace, featureName string) (*plumbing.Hash, error)
	GetProtoMessages(ctx context.Context) ([]string, error)
	ParseMetadata(ctx context.Context) (*metadata.RootConfigRepoMetadata, map[string]*metadata.NamespaceConfigRepoMetadata, error)
	RestoreWorkingDirectory(hash string) error
}

func checkRuleFitsContextType(rule *rulesv1beta3.Rule, contextType protoreflect.MessageType) error {
	var err error
	switch r := rule.Rule.(type) {
	case *rulesv1beta3.Rule_BoolConst:
		return nil
	case *rulesv1beta3.Rule_Not:
		return checkRuleFitsContextType(r.Not, contextType)
	case *rulesv1beta3.Rule_LogicalExpression:
		for _, lr := range r.LogicalExpression.GetRules() {
			err = checkRuleFitsContextType(lr, contextType)
			if err != nil {
				return err
			}
		}
	case *rulesv1beta3.Rule_Atom:
		field := contextType.Descriptor().Fields().ByName(protoreflect.Name(r.Atom.GetContextKey()))
		if field == nil {
			return errors.Errorf("`%s` field not found in context type", r.Atom.GetContextKey())
		}
		fieldType := field.Kind().String()

		switch r.Atom.ComparisonOperator {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_PRESENT:
			return nil
		case
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS:
			switch r.Atom.GetComparisonValue().Kind.(type) {
			case *structpb.Value_BoolValue:
				if fieldType != "string" {
					return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
				}

			case *structpb.Value_NumberValue:
				if fieldType != "int64" && fieldType != "double" {
					return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
				}
			case *structpb.Value_StringValue:
				if fieldType != "string" {
					return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
				}
			default:
				panic("This should never happen")
			}
		case
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN,
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS,
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			if fieldType != "int64" && fieldType != "double" {
				return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
			}
			return nil
		case
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH,
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH,
			rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			if fieldType != "string" {
				return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
			}
			return nil
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			listRuleVal, ok := r.Atom.GetComparisonValue().Kind.(*structpb.Value_ListValue)
			if !ok {
				panic("This should never happen")
			}
			for _, elem := range listRuleVal.ListValue.Values {
				switch elem.Kind.(type) {
				case *structpb.Value_BoolValue:
					if fieldType != "string" {
						return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
					}

				case *structpb.Value_NumberValue:
					if fieldType != "int64" && fieldType != "double" {
						return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
					}
				case *structpb.Value_StringValue:
					if fieldType != "string" {
						return errors.Errorf("%s field has invalid type", r.Atom.GetContextKey())
					}
				default:
					panic("This should never happen")
				}
			}
			return nil
		}
		return nil
	case *rulesv1beta3.Rule_CallExpression:
		switch f := r.CallExpression.Function.(type) {
		case *rulesv1beta3.CallExpression_Bucket_:
			contextKey := f.Bucket.ContextKey
			if contextType.Descriptor().Fields().ByName(protoreflect.Name(contextKey)) == nil {
				return errors.Errorf("%s field not found in context type", contextKey)
			}
			return nil
		}
	default:
		panic("This should never happen")
	}
	return nil
}

func (r *repository) CompileFeature(ctx context.Context, registry *protoregistry.Types, nsContextTypes map[string]protoreflect.MessageType, namespace, featureName string, nv feature.NamespaceVersion) (*feature.CompiledFeature, error) {
	if !isValidName(namespace) {
		return nil, errors.Errorf("invalid name '%s'", namespace)
	}
	if !isValidName(featureName) {
		return nil, errors.Errorf("invalid name '%s'", featureName)
	}
	ff := feature.NewFeatureFile(namespace, featureName)
	err := nv.Supported()
	if errors.Is(err, feature.ErrUnknownVersion) {
		r.Logf("Ignoring %s/%s with version %s: %v\n", ff.NamespaceName, ff.Name, nv.String(), err)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	registry, err = r.registry(ctx, registry)
	if err != nil {
		return nil, errors.Wrap(err, "registry")
	}
	compiler := star.NewCompiler(registry, &ff, r)
	f, err := compiler.Compile(ctx, nv)
	if err != nil {
		return nil, errors.Wrap(err, "compile")
	}
	if nsContextTypes[namespace] != nil {
		for _, override := range f.Feature.Overrides {
			err = checkRuleFitsContextType(override.RuleASTV3, nsContextTypes[namespace])
			if err != nil {
				return nil, err
			}
		}
	}
	return f, nil
}

type VerifyRequest struct {
	// Registry of protobuf types. This field is optional, if it does not exist, it will be instantiated.
	Registry *protoregistry.Types
	// Optional fields to filter features by, so as to not verify the entire world.
	NamespaceFilter, FeatureFilter string
}

type CompileRequest struct {
	// Registry of protobuf types. This field is optional, if it does not exist, it will be instantiated.
	Registry *protoregistry.Types
	// Optional fields to filter features by, so as to not compile the entire world.
	NamespaceFilter, FeatureFilter string
	// Whether or not to persist the successfully compiled features
	DryRun bool
	// If true, any generated compilation changes will overwrite previous features
	// even if there are type mismatches
	IgnoreBackwardsCompatibility bool
	// If true, we will verify the structure of all feature files, ensuring that each
	// .star file has the relevant generated json and proto files, and all relevant compliance
	// checks are run. This should be false if we've just added a new .star file and are
	// compiling it for the first time.
	Verify bool
	// If true, and if a version later than the existing namespace version exists,
	// we will compile the requested feature(s) to the latest version.
	// Note: if upgrading, no feature filter is allowed to be specified.
	// That is because you must upgrade an entire namespace at a time.
	Upgrade bool
	// Enable verbose logging output
	Verbose bool
}

func (cr CompileRequest) Validate() error {
	if cr.Upgrade && len(cr.FeatureFilter) > 0 {
		return errors.New("cannot provide a config filter if upgrading an entire namespace")
	}
	return nil
}

type FeatureCompilationResult struct {
	NamespaceName         string
	FeatureName           string
	NamespaceVersion      feature.NamespaceVersion
	CompiledFeature       *feature.CompiledFeature
	CompilationError      error
	CompilationDiffExists bool
	FormattingDiffExists  bool
	PersistenceError      error
}

func (fcr *FeatureCompilationResult) CompilationErrorString(r Logger) {
	if fcr.Err() == nil {
		return
	}
	r.Logf(r.Bold(fmt.Sprintf("[%s/%s]\n", fcr.NamespaceName, fcr.FeatureName)))
	if fcr.CompilationError != nil {
		r.Logf(fmt.Sprintf("\t%s %v\n", r.Bold("→"), fcr.CompilationError))
	}
	if fcr.CompiledFeature == nil {
		return
	}
	for _, res := range fcr.CompiledFeature.ValidatorResults {
		if !res.Passed() {
			r.Logf(fmt.Sprintf("\t%s %s\n", r.Bold("→"), res.DebugString()))
		}
	}
	for _, res := range fcr.CompiledFeature.TestResults {
		if !res.Passed() {
			r.Logf(fmt.Sprintf("\t%s %s\n", r.Bold("→"), res.DebugString()))
		}
	}
	if fcr.PersistenceError != nil {
		r.Logf(fmt.Sprintf("\t[persistence] %s %v\n", r.Bold("→"), fcr.PersistenceError))
	}
}

func (fcr *FeatureCompilationResult) Summary(r Logger) []string {
	featureType := "-"
	compile := r.Red("✖")
	test := "-"
	validate := "-"
	persisted := "-"
	if fcr.CompilationError == nil {
		compile = r.Green("✔")
		if fcr.CompiledFeature != nil {
			featureType = string(fcr.CompiledFeature.Feature.FeatureType)
			if len(fcr.CompiledFeature.TestResults) > 0 {
				var numPassed int
				for _, tr := range fcr.CompiledFeature.TestResults {
					if tr.Passed() {
						numPassed++
					}
				}
				test = fmt.Sprintf("%d/%d", numPassed, len(fcr.CompiledFeature.TestResults))
				if numPassed == len(fcr.CompiledFeature.TestResults) {
					test = r.Green(test)
				} else {
					test = r.Red(test)
				}
			}

			if len(fcr.CompiledFeature.ValidatorResults) > 0 {
				var numPassed int
				for _, vr := range fcr.CompiledFeature.ValidatorResults {
					if vr.Passed() {
						numPassed++
					}
				}
				validate = fmt.Sprintf("%d/%d", numPassed, len(fcr.CompiledFeature.ValidatorResults))
				if numPassed == len(fcr.CompiledFeature.ValidatorResults) {
					validate = r.Green(validate)
				} else {
					validate = r.Red(validate)
				}
			}
			if fcr.PersistenceError == nil {
				persisted = r.Green("✔")
			} else {
				persisted = r.Red("✖")
			}
		}
	}
	return []string{fcr.NamespaceName, fcr.FeatureName, compile, featureType, test, validate, persisted}
}

func (fcr *FeatureCompilationResult) Err() error {
	if fcr.CompilationError != nil {
		return fcr.CompilationError
	}
	if fcr.CompiledFeature == nil {
		return nil
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
	if fcr.PersistenceError != nil {
		return fcr.PersistenceError
	}
	return nil
}

type FeatureCompilationResults []*FeatureCompilationResult

func (fcrs FeatureCompilationResults) RenderSummary(r Logger) {
	table := tablewriter.NewWriter(r.Writer())
	table.SetHeader([]string{"Namespace", "Config", "Compiled", "Type", "Tests", "Validators", "Persisted"})
	for _, fcr := range fcrs {
		table.Append(fcr.Summary(r))
	}
	table.Render()

	for _, fcr := range fcrs {
		fcr.CompilationErrorString(r)
	}
}

func (fcrs FeatureCompilationResults) Err() error {
	for _, fcr := range fcrs {
		if fcr.Err() != nil {
			return fcr.Err()
		}
	}
	return nil
}

func (r *repository) Verify(ctx context.Context, req *VerifyRequest) error {
	fcrs, err := r.Compile(ctx, &CompileRequest{
		Registry:        req.Registry,
		NamespaceFilter: req.NamespaceFilter,
		FeatureFilter:   req.FeatureFilter,
		// verification should not modify data
		DryRun: true,
		// verification should perform backwards compatibility check
		IgnoreBackwardsCompatibility: false,
		Verify:                       true,
		Verbose:                      false,
	})
	if err != nil {
		return errors.Wrap(err, "compile")
	}

	var hasDiff bool
	for _, fcr := range fcrs {
		if fcr.CompilationDiffExists {
			r.Logf("%s\n", r.Red(fmt.Sprintf("Found diff in generated files: %s/%s", fcr.NamespaceName, fcr.FeatureName)))
			hasDiff = true
		}
		if fcr.FormattingDiffExists {
			r.Logf("%s\n", r.Red(fmt.Sprintf("Found formatting diff: %s/%s.star", fcr.NamespaceName, fcr.FeatureName)))
			hasDiff = true
		}
	}
	if hasDiff {
		return errors.New("Found config(s) with compilation or formatting diffs")
	}
	return nil
}

func (r *repository) Compile(ctx context.Context, req *CompileRequest) ([]*FeatureCompilationResult, error) {
	if err := req.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate request")
	}

	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse metadata")
	}
	registry, err := r.registry(ctx, req.Registry)
	if err != nil {
		return nil, errors.Wrap(err, "registry")
	}
	nsContextTypes := make(map[string]protoreflect.MessageType)
	for ns, nsMd := range nsMDs {
		if nsMd.ContextProto != "" {
			r.Logf("%s: %s\n", ns, nsMd.ContextProto)
			ct, err := registry.FindMessageByName(protoreflect.FullName(nsMd.ContextProto))
			if err != nil {
				return nil, err
			}
			for i := 0; i < ct.Descriptor().Fields().Len(); i++ {
				f := ct.Descriptor().Fields().Get(i)
				switch f.Kind().String() {
				case
					"bool",
					"int64",
					"double",
					"string":
				default:
					return nil, errors.New("Invalid context type thingy make this better")
				}
			}
			nsContextTypes[ns] = ct
		}
	}

	// Step 1: collect. Find all features
	vffs, numNamespaces, err := r.findVersionedFeatureFiles(ctx, req.NamespaceFilter, req.FeatureFilter, req.Verify)
	if err != nil {
		return nil, errors.Wrap(err, "find configs")
	}
	oldNamespaces := make(map[string]struct{})
	var results FeatureCompilationResults
	for _, vff := range vffs {
		if vff.nv.Before(feature.LatestNamespaceVersion()) {
			oldNamespaces[vff.ff.NamespaceName] = struct{}{}
		}
		desiredVersion := vff.nv
		if req.Upgrade {
			desiredVersion = feature.LatestNamespaceVersion()
		}
		if err := desiredVersion.Supported(); errors.Is(err, feature.ErrUnknownVersion) {
			r.Logf("Ignoring %s/%s with version %s: %v\n", vff.ff.NamespaceName, vff.ff.Name, desiredVersion.String(), err)
			continue // don't compile unknown versions to preserve forwards compatibility.
		}
		results = append(results, &FeatureCompilationResult{NamespaceName: vff.ff.NamespaceName, FeatureName: vff.ff.Name, NamespaceVersion: desiredVersion})
	}
	r.Logf("Found %d configs across %d namespaces\n", len(results), numNamespaces)
	r.Logf("Compiling...\n")
	concurrency := 50
	if len(results) < 50 {
		concurrency = len(results)
	}

	// This object will hold a map from namespace to named segments
	// for that ns. For example, if a segment config has two segments: "foo == A, segmentA" and "bar == B, segmentB" in namespace "default"
	// this map would contain:
	//
	// {"default":{"segmentA": "foo == A", "segmentB": "foo == b"}}
	//
	nsNameToSegments := make(map[string]map[string]string)
	for _, fcr := range results {
		if fcr.FeatureName == "segments" {
			cf, err := r.CompileFeature(ctx, registry, nsContextTypes, fcr.NamespaceName, fcr.FeatureName, fcr.NamespaceVersion)
			if err != nil {
				fcr.CompilationError = err
			}
			rules := make(map[string]string)
			for _, override := range cf.Feature.Overrides {
				val, ok := override.Value.(string)
				if !ok {
					panic(fmt.Sprintf("should be a string: %+v", override.Value))
				}

				rules[val] = override.Rule
			}
			nsNameToSegments[fcr.NamespaceName] = rules
		}
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
					ff := feature.NewFeatureFile(fcr.NamespaceName, fcr.FeatureName)
					// format starlark file
					_, fmtDiffExists, err := r.FormatFeature(ctx, &ff, registry, req.DryRun, req.Verbose, nsNameToSegments[fcr.NamespaceName])
					if err != nil {
						fcr.CompilationError = errors.Wrap(err, "format")
						return
					}
					fcr.FormattingDiffExists = fmtDiffExists
					// compile feature
					cf, err := r.CompileFeature(ctx, registry, nsContextTypes, fcr.NamespaceName, fcr.FeatureName, fcr.NamespaceVersion)
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

	// persisting
	namespaces := make(map[string]struct{})
	for _, fcr := range results {
		if fcr.CompiledFeature == nil {
			continue
		}
		namespaces[fcr.NamespaceName] = struct{}{}
		err := fcr.NamespaceVersion.Supported()
		if errors.Is(err, feature.ErrUnknownVersion) {
			continue
		}
		if err != nil {
			return nil, err
		}
		ff := feature.NewFeatureFile(fcr.NamespaceName, fcr.FeatureName)
		compilePersisted, compileDiffExists, err := star.NewCompiler(registry, &ff, r).Persist(ctx, fcr.CompiledFeature.Feature, fcr.NamespaceVersion, req.IgnoreBackwardsCompatibility, req.DryRun)
		if err != nil {
			fcr.PersistenceError = err
			continue
		}
		fcr.CompilationDiffExists = compileDiffExists

		if compilePersisted {
			r.Logf("Generated diff for %s/%s\n", fcr.NamespaceName, fcr.CompiledFeature.Feature.Key)
		}
	}

	results.RenderSummary(r)

	if req.Upgrade && !req.DryRun {
		for ns := range namespaces {
			if err := metadata.UpdateNamespaceMetadata(ctx, "", ns, r, func(ncrm *metadata.NamespaceConfigRepoMetadata) {
				ncrm.Version = string(feature.LatestNamespaceVersion())
			}); err != nil {
				return nil, errors.Wrapf(err, "failed to upgrade namespace metadata")
			}
		}
	}

	// Print upgrade warning
	if !req.Upgrade && len(oldNamespaces) > 0 {
		r.Logf("-------------------\n")
		var oldNamespacesArr []string
		for oldNS := range oldNamespaces {
			oldNamespacesArr = append(oldNamespacesArr, oldNS)
		}
		sort.Strings(oldNamespacesArr)
		r.Logf(r.Yellow("Warning: The following namespaces need an upgrade:\n"))
		for _, oldNS := range oldNamespacesArr {
			r.Logf(fmt.Sprintf("\t%s\n", oldNS))
		}
		r.Logf("Run '%s' to perform the upgrade.\n", r.Bold("lekko compile --upgrade"))
	}

	if req.Upgrade && len(oldNamespaces) == 0 {
		r.Logf("Nothing to upgrade.\n")
	}

	return results, results.Err()
}

type versionedFeatureFile struct {
	ff *feature.FeatureFile
	nv feature.NamespaceVersion
}

func (r *repository) findVersionedFeatureFiles(ctx context.Context, namespaceFilter, featureFilter string, verify bool) ([]*versionedFeatureFile, int, error) {
	contents, err := r.GetContents(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "get contents")
	}
	var numNamespaces int
	var results []*versionedFeatureFile
	for nsMD, ffs := range contents {
		if len(namespaceFilter) > 0 && nsMD.Name != namespaceFilter {
			continue
		}
		numNamespaces++
		nv := feature.NewNamespaceVersion(nsMD.Version)
		if err != nil {
			return nil, 0, err
		}
		for _, ff := range ffs {
			ff := ff
			if len(featureFilter) > 0 && ff.Name != featureFilter {
				continue
			}
			if verify {
				if err := ff.Verify(); err != nil {
					return nil, 0, errors.Wrapf(err, "config %s/%s verify", ff.NamespaceName, ff.Name)
				}
				if err := feature.ComplianceCheck(ff, &nsMD); err != nil {
					return nil, 0, errors.Wrapf(err, "config %s/%s compliance check", ff.NamespaceName, ff.Name)
				}
			}
			results = append(results, &versionedFeatureFile{
				ff: &ff,
				nv: nv,
			})
		}
	}
	return results, numNamespaces, nil
}

func (r *repository) registry(ctx context.Context, registry *protoregistry.Types) (*protoregistry.Types, error) {
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

func (r *repository) BuildDynamicTypeRegistry(ctx context.Context, protoDirPath string) (*protoregistry.Types, error) {
	sTypes, err := prototypes.BuildDynamicTypeRegistry(ctx, protoDirPath, r)
	if err != nil {
		return nil, err
	}
	return sTypes.Types, err
}

// Actually regenerates the buf image, and writes it to the file system.
// Note: we don't have a way yet to run this from an ephemeral repo,
// because we need to first ensure that buf cmd line can be executed in the
// ephemeral env.
func (r *repository) ReBuildDynamicTypeRegistry(ctx context.Context, protoDirPath string, useExternalTypes bool) (*protoregistry.Types, error) {
	if !r.bufEnabled {
		return nil, errors.New("buf cmd line not enabled")
	}
	sTypes, err := prototypes.ReBuildDynamicTypeRegistry(ctx, protoDirPath, useExternalTypes, r)
	if err != nil {
		return nil, err
	}
	return sTypes.Types, err
}

func (r *repository) GetFileDescriptorSet(ctx context.Context, protoDirPath string) (*descriptorpb.FileDescriptorSet, error) {
	sTypes, err := prototypes.BuildDynamicTypeRegistry(ctx, protoDirPath, r)
	if err != nil {
		return nil, err
	}
	return sTypes.FileDescriptorSet, nil
}

func (r *repository) Format(ctx context.Context, verbose bool) error {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "parse metadata")
	}
	for ns := range nsMDs {
		ffs, err := r.GetFeatureFiles(ctx, ns)
		if err != nil {
			return errors.Wrap(err, "get config files")
		}

		for _, ff := range ffs {
			ff := ff
			formatted, _, err := r.FormatFeature(ctx, &ff, nil, false, verbose, nil)
			if err != nil {
				return errors.Wrapf(err, "format config '%s/%s", ff.NamespaceName, ff.Name)
			}
			if formatted {
				r.Logf("Formatted and rewrote %s/%s\n", ff.NamespaceName, ff.Name)
			}
		}
	}
	return nil
}

func (r *repository) FormatFeature(ctx context.Context, ff *feature.FeatureFile, registry *protoregistry.Types, dryRun, verbose bool, segments map[string]string) (persisted, diffExists bool, err error) {
	registry, err = r.registry(ctx, registry)
	if err != nil {
		return false, false, err
	}
	nsMD, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ff.NamespaceName, r)
	if err != nil {
		return false, false, errors.Wrap(err, "parse namespace metadata")
	}
	formatter := star.NewStarFormatter(
		ff.RootPath(ff.StarlarkFileName),
		ff.Name,
		r,
		dryRun,
		registry,
		feature.NewNamespaceVersion(nsMD.Version),
		segments,
	)
	// try static formatting
	persisted, diffExists, err = formatter.StaticFormat(ctx)
	if errors.Is(err, static.ErrUnsupportedStaticParsing) {
		// show warning
		r.Logf("%s\n", r.Yellow(fmt.Sprintf("[%s] Unable to statically parse config. Web features may be limited.", fmt.Sprintf("%s/%s", ff.NamespaceName, ff.Name))))
		if verbose {
			r.Logf("\t%v\n", err)
		}
	} else if err != nil {
		return false, false, err
	} else if persisted {
		return persisted, diffExists, nil
	}
	// fall back to regular formatting
	return formatter.Format(ctx)
}

// Adds a new feature to the given namespace using the given type.
// Returns an error if:
// the namespace doesn't exist, or
// a feature named featureName already exists
// Returns the path to the feature file that was written to disk.
func (r *repository) AddFeature(ctx context.Context, ns, featureName string, fType eval.ConfigType, protoMessageName string) (string, error) {
	if !isValidName(featureName) {
		return "", errors.Wrap(ErrInvalidName, "config")
	}
	nsMD, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	nv := feature.NewNamespaceVersion(nsMD.Version)
	if err != nil {
		return "", fmt.Errorf("error parsing namespace metadata: %w", err)
	}
	ffs, err := r.GetFeatureFiles(ctx, ns) // returns err if ns doesn't exist
	if err != nil {
		return "", fmt.Errorf("failed to get config files: %v", err)
	}
	for _, ff := range ffs {
		if ff.Name == featureName {
			return "", fmt.Errorf("config named %s already exists", featureName)
		}
	}

	var template []byte
	if fType == eval.ConfigTypeProto {
		template, err = r.addFeatureFromProto(ctx, protoMessageName, nv)
		if err != nil {
			return "", errors.Wrap(err, "add config from proto")
		}
	} else {
		template, err = star.GetTemplate(fType, nv)
		if err != nil {
			return "", errors.Wrap(err, "get template")
		}
	}

	featurePath := filepath.Join(ns, fmt.Sprintf("%s.star", featureName))
	if err := r.WriteFile(featurePath, template, 0600); err != nil {
		return "", fmt.Errorf("failed to add config: %v", err)
	}
	return featurePath, nil
}

// addFeatureFromProto uses reflection to generate a Starlark feature template specific to the message descriptor
func (r *repository) addFeatureFromProto(ctx context.Context, messageName string, nv feature.NamespaceVersion) ([]byte, error) {
	// Get the MessageType from the name, it involves loading the type registry again. This can probably be cached
	// from the initial call when the type names were computed
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, err
	}
	types, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return nil, err
	}

	mt, err := types.FindMessageByName(protoreflect.FullName(messageName))
	if err != nil {
		return nil, err
	}
	descriptor := mt.Descriptor()

	// packageMap maps a '.' separated package name to an importable alias
	packageMap := map[string]string{}
	fieldDefaults := []string{}

	descriptorPkgName := string(descriptor.ParentFile().Package())
	descriptorPkgAlias := packageAlias(descriptorPkgName)
	descriptorName := descriptorPkgAlias + "." + string(descriptor.Name())

	packageMap[descriptorPkgName] = descriptorPkgAlias

	// loop through the Fields to assign reasonable defaults for each field. If it's an imported type, we also need to include the package name.
	for i := 0; i < descriptor.Fields().Len(); i++ {
		field := descriptor.Fields().Get(i)
		fieldDefault := ""
		fieldDefault += string(field.Name()) + " = "
		switch field.Kind() {
		case protoreflect.MessageKind:
			if field.IsMap() {
				fieldDefault += "{}"
			} else {
				pkgName := string(field.Message().ParentFile().Package())
				packageMap[pkgName] = packageAlias(pkgName)
				instance := string(field.Message().FullName()) + "()"
				if field.IsList() {
					instance = fmt.Sprintf("[%s]", instance)
				}
				fieldDefault += instance
			}
		case protoreflect.EnumKind:
			pkgName := string(field.Enum().ParentFile().Package())
			packageMap[pkgName] = packageAlias(pkgName)
			fieldDefault += string(field.Enum().FullName()) + "." + string(field.Enum().Values().ByNumber(0).Name())
		case protoreflect.BytesKind:
			fieldDefault += "\"\""
		default:
			if field.IsList() {
				fieldDefault += "[]"
			} else {
				primitiveVal := field.Default().String()
				if primitiveVal == "" {
					primitiveVal = "\"\""
				}
				if primitiveVal == "false" {
					primitiveVal = "False"
				}
				fieldDefault += primitiveVal
			}
		}
		fieldDefaults = append(fieldDefaults, fieldDefault)
	}

	// for each default value, replace the package names with the package alias.
	// ideally, the package alias is resolved when generating the field defaults but
	// it was a bit tricky with Nested Messages.
	// eg. for a Message of: a.b.c.NestedMessage.FooBar, Name() only returns 'FooBar' and Package()
	// only returns 'a.b.c'. It was not easy (afaik) to get the Nested Type. The Parent() method seemed promising.
	// Note: this might generate the wrong package aliases if there are messages used in the same package hierarchy.
	for i, res := range fieldDefaults {
		for pkgName, alias := range packageMap {
			res = strings.Replace(res, pkgName, alias, 1)
		}
		fieldDefaults[i] = res
	}

	return star.RenderExistingProtoTemplate(star.ProtoStarInputs{
		Message:  descriptorName,
		Packages: packageMap,
		Fields:   fieldDefaults,
	}, nv)
}

func packageAlias(pkgName string) string {
	return strings.ReplaceAll(pkgName, ".", "_")
}

// GetProtoMessages returns a list of protobuf messages for a configuration repository
func (r *repository) GetProtoMessages(ctx context.Context) ([]string, error) {
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, err
	}
	types, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return nil, err
	}
	results := []string{}
	f := func(r protoreflect.MessageType) bool {
		results = append(results, string(r.Descriptor().FullName()))
		return true
	}
	types.RangeMessages(f)
	sort.Strings(results)
	return results, nil
}

// Removes the given feature. If the namespace or feature doesn't exist, returns
// an error.
func (r *repository) RemoveFeature(ctx context.Context, ns, featureName string) error {
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
	if !removed {
		return errors.Errorf("config %s does not exist", featureName)
	}
	return nil
}

// Adds the given namespace by adding it to lekko.root.yaml, and creating the
// directory structure for it.
func (r *repository) AddNamespace(ctx context.Context, name string) error {
	if !isValidName(name) {
		return errors.Wrap(ErrInvalidName, "namespace")
	}
	// First, try to find the namespace we wish to create
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, "", name, r)
	if errors.Is(err, os.ErrNotExist) {
		// if it doesn't exist, create it and exit.
		if err := metadata.CreateNamespaceMetadata(ctx, "", name, r, string(feature.LatestNamespaceVersion())); err != nil {
			return errors.Wrap(err, "create ns meta")
		}
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "parse ns meta")
	}
	return errors.New("ns already exists")
}

// Removes the given namespace from the repo, as well as from lekko.root.yaml.
func (r *repository) RemoveNamespace(ctx context.Context, ns string) error {
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
	return nil
}

func (r *repository) Eval(ctx context.Context, ns, featureName string, featureCtx map[string]interface{}) (*anypb.Any, eval.ConfigType, eval.ResultPath, error) {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, "parse metadata")
	}
	nsMD, ok := nsMDs[ns]
	if !ok {
		return nil, "", nil, fmt.Errorf("invalid namespace: %s", ns)
	}

	ff, err := r.GetFeatureFile(ctx, ns, featureName)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, "get config file")
	}

	if err := feature.ComplianceCheck(*ff, nsMD); err != nil {
		return nil, "", nil, errors.Wrap(err, "compliance check")
	}

	evalF, err := encoding.ParseFeature(ctx, "", *ff, nsMD, r)
	if err != nil {
		return nil, "", nil, err
	}
	ret, path, err := evalF.Evaluate(featureCtx)
	return ret, evalF.Type(), path, err
}

func (r *repository) Parse(ctx context.Context, ns, featureName string, registry *protoregistry.Types) (*featurev1beta1.StaticFeature, error) {
	nsMD, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil {
		return nil, errors.Wrap(err, "parse namespace metadata")
	}
	fc, err := r.GetFeatureContents(ctx, ns, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get config contents")
	}
	registry, err = r.registry(ctx, registry)
	if err != nil {
		return nil, err
	}
	filename := fc.File.RootPath(fc.File.StarlarkFileName)
	w := static.NewWalker(filename, fc.Star, registry, feature.NamespaceVersion(nsMD.Version))
	f, err := w.Build()
	if err != nil {
		return nil, errors.Wrap(err, "build")
	}
	f.Key = fc.File.Name
	f.FeatureOld.Key = fc.File.Name

	// Rewrite the bytes to the starfile path, based on the parse AST.
	// This is just an illustration, but in the future we could modify
	// the feature and use the following code to write it out.
	bytes, err := w.Mutate(f)
	if err != nil {
		return nil, errors.Wrap(err, "mutate")
	}
	if err := r.WriteFile(filename, bytes, 0600); err != nil {
		return nil, errors.Wrap(err, "failed to write file")
	}
	return f, nil
}

func (r *repository) GetContents(ctx context.Context) (map[metadata.NamespaceConfigRepoMetadata][]feature.FeatureFile, error) {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse root md")
	}
	ret := make(map[metadata.NamespaceConfigRepoMetadata][]feature.FeatureFile, len(nsMDs))
	for namespace, nsMD := range nsMDs {
		ffs, err := r.GetFeatureFiles(ctx, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "get config files")
		}
		ret[*nsMD] = ffs
	}
	return ret, nil
}

func (r *repository) ListNamespaces(ctx context.Context) ([]*metadata.NamespaceConfigRepoMetadata, error) {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse md")
	}
	var ret []*metadata.NamespaceConfigRepoMetadata
	for _, v := range nsMDs {
		ret = append(ret, v)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})
	return ret, nil
}

const maxNameLength = 64

var (
	allchars       = regexp.MustCompile(`^[a-z0-9_\-.]+$`)
	boundary       = regexp.MustCompile(`^[a-z0-9]+$`)
	ErrInvalidName = errors.New("invalid name")
)

// Simple rules for how to name namespaces and features.
// Only allows alphanumeric lowercase characters, plus '.-_'
// Cannot be too long, and cannot start or end with any special characters.
// See feature_test.go for examples.
// TODO: there is probably a way to do this in a single regexp.
func isValidName(name string) bool {
	return allchars.MatchString(name) &&
		len(name) <= maxNameLength &&
		boundary.MatchString(string(name[0])) &&
		boundary.MatchString(string(name[len(name)-1]))
}

func (r *repository) GetFeatureFiles(ctx context.Context, namespace string) ([]feature.FeatureFile, error) {
	ffs, err := feature.GroupFeatureFiles(ctx, namespace, r)
	if err != nil {
		return nil, errors.Wrap(err, "group config files")
	}
	return ffs, nil
}

func (r *repository) GetFeatureFile(ctx context.Context, namespace, featureName string) (*feature.FeatureFile, error) {
	ffs, err := r.GetFeatureFiles(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "get config files")
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
		return nil, fmt.Errorf("config '%s' not found in namespace '%s'", featureName, namespace)
	}
	return ff, nil
}

func (r *repository) GetFeatureContents(ctx context.Context, namespace, featureName string) (*feature.FeatureContents, error) {
	ff, err := r.GetFeatureFile(ctx, namespace, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get config file")
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
func (r *repository) GetFeatureHash(ctx context.Context, namespace, featureName string) (*plumbing.Hash, error) {
	ff, err := r.GetFeatureFile(ctx, namespace, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get config file")
	}
	hash, err := r.headHash()
	if err != nil {
		return nil, errors.Wrap(err, "working directory hash")
	}
	co, err := r.repo.CommitObject(*hash)
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

func (r *repository) ParseMetadata(ctx context.Context) (*metadata.RootConfigRepoMetadata, map[string]*metadata.NamespaceConfigRepoMetadata, error) {
	return metadata.ParseFullConfigRepoMetadataStrict(ctx, "", r)
}

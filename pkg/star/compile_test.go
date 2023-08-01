package star

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/star/prototypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type testFS struct {
	files map[string]string
}

func newTestFS(files map[string]string) *testFS {
	return &testFS{files}
}

func (fs *testFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return fmt.Errorf("not supported")
}

func (fs *testFS) MkdirAll(path string, perm os.FileMode) error {
	return fmt.Errorf("not supported")
}

func (fs *testFS) RemoveIfExists(path string) (bool, error) {
	return false, fmt.Errorf("not supported")
}

func (fs *testFS) GetFileContents(ctx context.Context, path string) ([]byte, error){
	contents, ok := fs.files[path]
	if !ok {
		return nil, fmt.Errorf("file does not exist: %s", path)
	}
	return []byte(contents), nil
}

func (fs *testFS) GetDirContents(ctx context.Context, path string) ([]fs.ProviderFile, error){
	return nil, fmt.Errorf("not supported")
}

func (fs *testFS) IsNotExist(err error) bool {
	return true
}

func getRegistry(t *testing.T) *protoregistry.Types {
	sTypes, err := prototypes.RegisterDynamicTypes(nil)
	if err != nil {
		t.Error()
	}
	return sTypes.Types	
}

func TestCompile_exportConfig(t *testing.T) {
	namespace := "default"
	featureName := "test"
	ff := feature.NewFeatureFile(namespace, featureName)
	fs := newTestFS(
		map[string]string{
			"default/test.star": `export(Config(description = "test", default = 1))`,
		},
	)
	c := NewCompiler(getRegistry(t), &ff, fs)
	cf, err := c.Compile(context.Background(), feature.NamespaceVersionV1Beta5)

	require.NoError(t, err)
	require.NotNil(t, cf)
	require.Equal(t, cf.Feature.Description, "test")
	require.Equal(t, cf.Feature.Value, int64(1))
}
func TestCompile_exportNotConfig(t *testing.T) {
	namespace := "default"
	featureName := "test"
	ff := feature.NewFeatureFile(namespace, featureName)
	fs := newTestFS(
		map[string]string{
			"default/test.star": `export(feature(description = "test", default = 1))`,
		},
	)
	c := NewCompiler(getRegistry(t), &ff, fs)
	_, err := c.Compile(context.Background(), feature.NamespaceVersionV1Beta5)
	require.Error(t, err)
}

func TestCompile_feature(t *testing.T) {
	namespace := "default"
	featureName := "test"
	ff := feature.NewFeatureFile(namespace, featureName)
	fs := newTestFS(
		map[string]string{
			"default/test.star": `result = feature(description = "test", default = 1)`,
		},
	)
	c := NewCompiler(getRegistry(t), &ff, fs)
	cf, err := c.Compile(context.Background(), feature.NamespaceVersionV1Beta5)

	require.NoError(t, err)
	require.NotNil(t, cf)
	require.Equal(t, cf.Feature.Description, "test")
	require.Equal(t, cf.Feature.Value, int64(1))
}
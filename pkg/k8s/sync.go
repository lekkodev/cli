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

package k8s

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	lekkoConfigMapPrefix string = "lekko."
	configMapLabel       string = "lekko"
	annotationKeyHash    string = "last-applied-hash"
	annotationKeyBranch  string = "last-applied-branch"
	annotationKeyUser    string = "last-applied-by"
)

type kubeClient struct {
	cs           *kubernetes.Clientset
	k8sNamespace string
	cr           *gh.ConfigRepo
}

// Returns an object that acts as lekko cli's gateway to kubernetes. Handles
// initializing the client, and operates on the single given namespace.
// TODO: handle multiple namespaces in the future?
func NewKubernetes(kubeConfigPath, k8sNamespace string, cr *gh.ConfigRepo) (*kubeClient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "build cfg from flags")
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "new for config")
	}
	return &kubeClient{
		cs:           clientset,
		k8sNamespace: k8sNamespace,
		cr:           cr,
	}, nil
}

// Apply will construct a representation of what k8s configmaps should look like
// based on the current, generated proto files in the repo. It will then apply
// that representation onto k8s, ensuring that k8s state matches the local working
// directory exactly. It will delete configmaps that don't exist in the config repo,
// and apply the ones that do.
// See https://kubernetes.io/docs/reference/kubectl/cheatsheet/#kubectl-apply
func (k *kubeClient) Apply(ctx context.Context, root string) error {
	if err := k.cr.AuthenticateGithub(ctx); err != nil {
		return errors.Wrap(err, "auth github")
	}
	provider := fs.LocalProvider()
	// Find all lekko configmaps first, so we can later delete ones that shouldn't exist
	result, err := k.cs.CoreV1().ConfigMaps(k.k8sNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: configMapLabel,
	})
	if err != nil {
		return errors.Wrap(err, "configmaps list")
	}
	existingConfigMaps := make(map[string]struct{})
	for _, item := range result.Items {
		existingConfigMaps[item.GetName()] = struct{}{}
	}

	_, nsMD, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, root, provider)
	if err != nil {
		return errors.Wrap(err, "failed to parse full config repo metadata")
	}

	for _, md := range nsMD {
		cmName := fmt.Sprintf("%s%s", lekkoConfigMapPrefix, md.Name)
		if err := k.applyLekkoNamespace(ctx, root, md, provider, cmName); err != nil {
			return fmt.Errorf("namespace %s: apply: %w", md.Name, err)
		}
		delete(existingConfigMaps, cmName)
	}

	// delete config maps that were not applyed
	for cmName := range existingConfigMaps {
		if err := k.cs.CoreV1().ConfigMaps(k.k8sNamespace).Delete(ctx, cmName, metav1.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "cm delete")
		}
	}
	return nil
}

func (k *kubeClient) fieldManager() string {
	return "lekko"
}

func (k *kubeClient) annotationKey(key string) string {
	return fmt.Sprintf("lekko/%s", key)
}

func (k *kubeClient) addAnnotations(cm *corev1.ConfigMapApplyConfiguration) error {
	hash, err := k.cr.WorkingDirectoryHash()
	if err != nil {
		return errors.Wrap(err, "wd hash")
	}
	user := k.cr.Secrets.GetGithubUser()
	branch, err := k.cr.BranchName()
	if err != nil {
		return errors.Wrap(err, "branch name")
	}
	cm.WithAnnotations(map[string]string{
		k.annotationKey(annotationKeyHash):   hash,
		k.annotationKey(annotationKeyUser):   user,
		k.annotationKey(annotationKeyBranch): branch,
	})
	return nil
}

func (k *kubeClient) applyLekkoNamespace(
	ctx context.Context,
	root string,
	nsMD *metadata.NamespaceConfigRepoMetadata,
	provider fs.Provider,
	cmName string,
) error {
	cm := corev1.ConfigMap(cmName, k.k8sNamespace)
	nsPath := filepath.Join(root, nsMD.Name)
	featureFiles, err := feature.GroupFeatureFiles(
		context.Background(),
		nsPath,
		nsMD,
		provider,
		false,
	)
	if err != nil {
		return fmt.Errorf("group feature files: %w", err)
	}
	for _, ff := range featureFiles {
		bytes, err := provider.GetFileContents(ctx, filepath.Join(nsPath, ff.CompiledProtoBinFileName))
		if err != nil {
			return fmt.Errorf("file %s: get file contents: %w", ff.Name, err)
		}
		cm.WithBinaryData(map[string][]byte{ff.Name: bytes})
	}
	if err := k.addAnnotations(cm); err != nil {
		return errors.Wrap(err, "add annotations")
	}
	result, err := k.cs.CoreV1().ConfigMaps(k.k8sNamespace).Apply(ctx, cm, metav1.ApplyOptions{
		FieldManager: k.fieldManager(),
	})
	if err != nil {
		return errors.Wrap(err, "cm apply")
	}
	fmt.Printf("successfully applied configmap '%s' with %d features\n", result.Name, len(result.BinaryData))
	return nil
}

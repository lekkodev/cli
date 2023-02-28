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
	"os"
	"text/tabwriter"

	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	lekkoConfigMapPrefix   string = "lekko."
	configMapLabel         string = "lekko"
	configMapSchemaVersion string = "v1beta1"
	annotationKeyHash      string = "last-applied-hash"
	annotationKeyBranch    string = "last-applied-branch"
	annotationKeyUser      string = "last-applied-by"
)

type kubeClient struct {
	cs           *kubernetes.Clientset
	k8sNamespace string
	r            *repo.Repo
}

// Returns an object that acts as lekko cli's gateway to kubernetes. Handles
// initializing the client, and operates on the single given namespace.
// TODO: handle multiple namespaces in the future?
func NewKubernetes(kubeConfigPath string, r *repo.Repo) (*kubeClient, error) {
	if kubeConfigPath == "" {
		return nil, errors.New("kubeConfigPath not provided")
	}
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}, nil)
	ns, _, err := cfg.Namespace()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get namespace from kube config")
	}
	fmt.Printf("Using kube namespace %s\n", ns)
	clientCfg, err := cfg.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "build cfg from flags")
	}
	clientset, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, errors.Wrap(err, "new for config")
	}
	return &kubeClient{
		cs:           clientset,
		k8sNamespace: ns,
		r:            r,
	}, nil
}

// func buildConfigFromFlags(kubeContext)

// Apply will construct a representation of what k8s configmaps should look like
// based on the current, generated proto files in the repo. It will then apply
// that representation onto k8s, ensuring that k8s state matches the local working
// directory exactly. It will delete configmaps that don't exist in the config repo,
// and apply the ones that do.
// See https://kubernetes.io/docs/reference/kubectl/cheatsheet/#kubectl-apply
func (k *kubeClient) Apply(ctx context.Context, username string) error {
	if len(username) == 0 {
		return errors.New("no username provided")
	}
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

	_, nsMD, err := k.r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to parse full config repo metadata")
	}

	for _, md := range nsMD {
		cmName := fmt.Sprintf("%s%s", lekkoConfigMapPrefix, md.Name)
		if err := k.applyLekkoNamespace(ctx, md.Name, cmName, username); err != nil {
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

func (k *kubeClient) List(ctx context.Context) error {
	// Find all lekko configmaps
	result, err := k.cs.CoreV1().ConfigMaps(k.k8sNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: configMapLabel,
	})
	if err != nil {
		return errors.Wrap(err, "configmaps list")
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	fmt.Fprintf(w, "k8s Namespace\tLekko Namespace\tFeature Name\tSize\n")
	for _, item := range result.Items {
		for featureName, featureBytes := range item.BinaryData {
			fmt.Fprintf(w, "%s\t%s\t%s\t[%d bytes]\n", item.GetNamespace(), item.GetName(), featureName, len(featureBytes))
		}
	}
	w.Flush()
	return nil
}

func (k *kubeClient) fieldManager() string {
	return "lekko"
}

func (k *kubeClient) annotationKey(key string) string {
	return fmt.Sprintf("lekko/%s", key)
}

func (k *kubeClient) humanReadableHash() (string, error) {
	hash, err := k.r.Hash()
	if err != nil {
		return "", err
	}
	var suffix string
	clean, err := k.r.IsClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	if !clean {
		suffix = "-dirty"
	}
	return fmt.Sprintf("%s%s", hash, suffix), nil
}

func (k *kubeClient) addAnnotations(cm *corev1.ConfigMapApplyConfiguration, username string) error {
	hash, err := k.humanReadableHash()
	if err != nil {
		return errors.Wrap(err, "wd hash")
	}
	branch, err := k.r.BranchName()
	if err != nil {
		return errors.Wrap(err, "branch name")
	}
	cm.WithAnnotations(map[string]string{
		k.annotationKey(annotationKeyHash):   hash,
		k.annotationKey(annotationKeyUser):   username,
		k.annotationKey(annotationKeyBranch): branch,
	})
	return nil
}

func (k *kubeClient) applyLekkoNamespace(
	ctx context.Context,
	namespaceName string,
	cmName string,
	username string,
) error {
	cm := corev1.ConfigMap(cmName, k.k8sNamespace)
	ffs, err := k.r.GetFeatureFiles(ctx, namespaceName)
	if err != nil {
		return fmt.Errorf("get feature files: %w", err)
	}
	for _, ff := range ffs {
		bytes, err := k.r.GetFileContents(ctx, ff.RootPath(ff.CompiledProtoBinFileName))
		if err != nil {
			return fmt.Errorf("file %s: get file contents: %w", ff.Name, err)
		}
		cm.WithBinaryData(map[string][]byte{ff.Name: bytes})
	}
	if err := k.addAnnotations(cm, username); err != nil {
		return errors.Wrap(err, "add annotations")
	}
	cm.WithLabels(map[string]string{
		configMapLabel: configMapSchemaVersion,
	})
	result, err := k.cs.CoreV1().ConfigMaps(k.k8sNamespace).Apply(ctx, cm, metav1.ApplyOptions{
		FieldManager: k.fieldManager(),
	})
	if err != nil {
		return errors.Wrap(err, "cm apply")
	}
	fmt.Printf("successfully applied configmap '%s' with %d features\n", result.Name, len(result.BinaryData))
	return nil
}

/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package termination

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment

	ctx = context.Background()
)

func Test_Reconciler(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Termination Handler Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crds")},
	}

	// Use our own scheme so we don't interfere with any test cases
	// that would initialise the scheme themselves.
	scheme := runtime.NewScheme()
	kubernetesscheme.AddToScheme(scheme)
	machinev1.AddToScheme(scheme)

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

// StartTestHandler starts the test handler
func StartTestHandler(th Handler) (chan struct{}, chan error) {
	stop := make(chan struct{})
	errs := make(chan error)
	go func() {
		errs <- th.Run(stop)
	}()
	return stop, errs
}

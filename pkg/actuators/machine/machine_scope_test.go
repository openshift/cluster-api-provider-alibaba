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

package machine

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	. "github.com/onsi/gomega"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	alibabacloudproviderv1 "github.com/openshift/cluster-api-provider-alibaba/pkg/apis/alibabacloudprovider/v1"
	alibabacloudclient "github.com/openshift/cluster-api-provider-alibaba/pkg/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

const testNamespace = "ms-test"

func machineWithSpec(spec *machinev1.AlibabaCloudMachineProviderConfig) *machinev1beta1.Machine {
	rawSpec, err := alibabacloudproviderv1.RawExtensionFromProviderSpec(spec)
	if err != nil {
		panic("Failed to encode raw extension from provider spec")
	}

	return &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ms-test",
			Namespace: testNamespace,
		},
		Spec: machinev1beta1.MachineSpec{
			ProviderSpec: machinev1beta1.ProviderSpec{
				Value: rawSpec,
			},
		},
	}
}

func TestGetUserData(t *testing.T) {
	userDataSecretName := "test-ms-secret"

	defaultProviderSpec := &machinev1.AlibabaCloudMachineProviderConfig{
		UserDataSecret: &corev1.LocalObjectReference{
			Name: userDataSecretName,
		},
	}

	testCases := []struct {
		testCase         string
		userDataSecret   *corev1.Secret
		providerSpec     *machinev1.AlibabaCloudMachineProviderConfig
		expectedUserdata []byte
		expectError      bool
	}{
		{
			testCase: "all good",
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userDataSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					userDataSecretKey: []byte("{}"),
				},
			},
			providerSpec:     defaultProviderSpec,
			expectedUserdata: []byte("{}"),
			expectError:      false,
		},
		{
			testCase:       "missing secret",
			userDataSecret: nil,
			providerSpec:   defaultProviderSpec,
			expectError:    true,
		},
		{
			testCase: "missing key in secret",
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userDataSecretName,
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"badKey": []byte("{}"),
				},
			},
			providerSpec: defaultProviderSpec,
			expectError:  true,
		},
		{
			testCase:         "no provider spec",
			userDataSecret:   nil,
			providerSpec:     nil,
			expectError:      false,
			expectedUserdata: nil,
		},
		{
			testCase:         "no user-data in provider spec",
			userDataSecret:   nil,
			providerSpec:     &machinev1.AlibabaCloudMachineProviderConfig{},
			expectError:      false,
			expectedUserdata: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testCase, func(t *testing.T) {
			clientObjs := []runtime.Object{}

			if tc.userDataSecret != nil {
				clientObjs = append(clientObjs, tc.userDataSecret)
			}

			fakeClient := fake.NewFakeClient(clientObjs...)

			ms := &machineScope{
				Context:      context.Background(),
				client:       fakeClient,
				machine:      machineWithSpec(tc.providerSpec),
				providerSpec: tc.providerSpec,
			}

			userData, err := ms.getUserData()
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			userDataBytes, _ := base64.StdEncoding.DecodeString(userData)
			if !bytes.Equal(userDataBytes, tc.expectedUserdata) {
				t.Errorf("Got: %q, Want: %q", userData, tc.expectedUserdata)
			}
		})
	}
}

func TestPatchMachine(t *testing.T) {
	g := NewWithT(t)

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "vendor", "github.com", "openshift", "api", "machine", "v1beta1"),
		},
	}

	cfg, err := testEnv.Start()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg).ToNot(BeNil())
	defer func() {
		g.Expect(testEnv.Stop()).To(Succeed())
	}()

	k8sClient, err := client.New(cfg, client.Options{})
	g.Expect(err).ToNot(HaveOccurred())

	alibabaCloudCredentialsSecret := stubAlibabaCloudCredentialsSecret()
	g.Expect(k8sClient.Create(context.TODO(), alibabaCloudCredentialsSecret)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(context.TODO(), alibabaCloudCredentialsSecret)).To(Succeed())
	}()

	userDataSecret := stubUserDataSecret()
	g.Expect(k8sClient.Create(context.TODO(), userDataSecret)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(context.TODO(), userDataSecret)).To(Succeed())
	}()

	failedPhase := "Failed"

	providerStatus := &machinev1.AlibabaCloudMachineProviderStatus{}

	testCases := []struct {
		name   string
		mutate func(*machinev1beta1.Machine)
		expect func(*machinev1beta1.Machine) error
	}{
		{
			name: "Test changing labels",
			mutate: func(m *machinev1beta1.Machine) {
				m.ObjectMeta.Labels["testlabel"] = "test"
			},
			expect: func(m *machinev1beta1.Machine) error {
				if m.ObjectMeta.Labels["testlabel"] != "test" {
					return fmt.Errorf("label \"testlabel\" %q not equal expected \"test\"", m.ObjectMeta.Labels["test"])
				}
				return nil
			},
		},
		{
			name: "Test setting phase",
			mutate: func(m *machinev1beta1.Machine) {
				m.Status.Phase = &failedPhase
			},
			expect: func(m *machinev1beta1.Machine) error {
				if m.Status.Phase != nil && *m.Status.Phase == failedPhase {
					return nil
				}
				return fmt.Errorf("phase is nil or not equal expected \"Failed\"")
			},
		},
		{
			name: "Test setting provider status",
			mutate: func(m *machinev1beta1.Machine) {
				instanceID := stubInstanceID
				instanceState := "running"
				providerStatus.InstanceID = &instanceID
				providerStatus.InstanceState = &instanceState
			},
			expect: func(m *machinev1beta1.Machine) error {
				providerStatus, err := alibabacloudproviderv1.ProviderStatusFromRawExtension(m.Status.ProviderStatus)
				if err != nil {
					return fmt.Errorf("unable to get provider status: %v", err)
				}

				if providerStatus.InstanceID == nil || *providerStatus.InstanceID != stubInstanceID {
					return fmt.Errorf("instanceID is nil or not equal expected \"%s\"", stubInstanceID)
				}

				if providerStatus.InstanceState == nil || *providerStatus.InstanceState != "running" {
					return fmt.Errorf("instanceState is nil or not equal expected \"running\"")
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewWithT(t)

			machine, err := stubMasterMachine()
			gs.Expect(err).ToNot(HaveOccurred())
			gs.Expect(machine).ToNot(BeNil())

			ctx := context.TODO()

			// Create the machine
			gs.Expect(k8sClient.Create(ctx, machine)).To(Succeed())
			defer func() {
				gs.Expect(k8sClient.Delete(ctx, machine)).To(Succeed())
			}()

			getMachine := func() error {
				machineKey := types.NamespacedName{Namespace: machine.Namespace, Name: machine.Name}
				return k8sClient.Get(ctx, machineKey, machine)
			}
			gs.Eventually(getMachine, timeout).Should(Succeed())

			machineScope, err := newMachineScope(machineScopeParams{
				client:  k8sClient,
				machine: machine,
				alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return nil, nil
				},
			})

			if err != nil {
				t.Fatal(err)
			}

			tc.mutate(machineScope.machine)

			machineScope.providerStatus = providerStatus

			gs.Expect(machineScope.patchMachine()).To(Succeed())
			checkExpectation := func() error {
				if err := getMachine(); err != nil {
					return err
				}
				return tc.expect(machine)
			}
			gs.Eventually(checkExpectation, timeout).Should(Succeed())

			machineResourceVersion := machine.ResourceVersion

			gs.Expect(machineScope.patchMachine()).To(Succeed())
			gs.Eventually(getMachine, timeout).Should(Succeed())
			gs.Expect(machine.ResourceVersion).To(Equal(machineResourceVersion))
		})
	}
}

func TestGetNetworkAddress(t *testing.T) {
	cases := []struct {
		name     string
		instance *ecs.Instance
		expected []corev1.NodeAddress
	}{
		{
			name:     "Instance is Nil",
			instance: nil,
			expected: nil,
		},
		{
			name: "Instance have private ipv4/v6  address",
			instance: &ecs.Instance{
				NetworkInterfaces: ecs.NetworkInterfacesInDescribeInstances{
					NetworkInterface: []ecs.NetworkInterface{
						{
							Ipv6Sets: ecs.Ipv6SetsInDescribeInstances{
								Ipv6Set: []ecs.Ipv6Set{
									{
										Ipv6Address: "0000:0000:0000:0000:0000:0000:ac10:0102",
									},
								},
							},
							PrivateIpSets: ecs.PrivateIpSetsInDescribeInstances{
								PrivateIpSet: []ecs.PrivateIpSet{
									{
										PrivateIpAddress: "172.16.1.2",
									},
								},
							},
						},
					},
				},
				PublicIpAddress: ecs.PublicIpAddressInDescribeInstances{
					IpAddress: []string{"8.8.8.8"},
				},
			},
			expected: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "::ac10:102",
				},
				{
					Type:    corev1.NodeInternalIP,
					Address: "172.16.1.2",
				},
				{
					Type:    corev1.NodeExternalIP,
					Address: "8.8.8.8",
				},
				{
					Type:    corev1.NodeInternalDNS,
					Address: stubMasterMachineName,
				},
			},
		},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs := NewWithT(t)

			machine, err := stubMasterMachine()
			gs.Expect(err).ToNot(HaveOccurred())
			gs.Expect(machine).ToNot(BeNil())

			machineScope, err := newMachineScope(machineScopeParams{
				client:  k8sClient,
				machine: machine,
				alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return nil, nil
				},
			})

			if err != nil {
				t.Fatal(err)
			}

			nodeAddress, _ := machineScope.getNetworkAddress(c.instance)
			if !reflect.DeepEqual(c.expected, nodeAddress) {
				t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, nodeAddress)
			}
		})
	}
}

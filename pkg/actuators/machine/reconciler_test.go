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
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"

	alibabacloudproviderv1 "github.com/openshift/cluster-api-provider-alibaba/pkg/apis/alibabacloudprovider/v1"

	configv1 "github.com/openshift/api/config/v1"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"

	"github.com/golang/mock/gomock"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"

	alibabacloudclient "github.com/openshift/cluster-api-provider-alibaba/pkg/client"
	"github.com/openshift/cluster-api-provider-alibaba/pkg/client/mock"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	"k8s.io/kubectl/pkg/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	// Add types to scheme
	machinev1beta1.AddToScheme(scheme.Scheme)
	configv1.AddToScheme(scheme.Scheme)
}

func Test_Create(t *testing.T) {
	// mock  API calls
	mockCtrl := gomock.NewController(t)
	mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)

	mockAlibabaCloudClient.EXPECT().DescribeImages(gomock.Any()).Return(stubDescribeImagesResponse(), nil).AnyTimes()
	mockAlibabaCloudClient.EXPECT().ListResourceGroups(gomock.Any()).Return(stubListResourceGroupsResponse(), nil).AnyTimes()
	mockAlibabaCloudClient.EXPECT().DescribeSecurityGroups(gomock.Any()).Return(nil, fmt.Errorf("describeSecurityGroups error")).AnyTimes()
	mockAlibabaCloudClient.EXPECT().DescribeVSwitches(gomock.Any()).Return(nil, fmt.Errorf("describeVSwitches error")).AnyTimes()
	mockAlibabaCloudClient.EXPECT().RunInstances(gomock.Any()).Return(stubRunInstancesResponse(), nil).AnyTimes()
	mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubRunningInstanceStauts, "192.168.1.0"), nil).AnyTimes()

	testCases := []struct {
		testcase                      string
		providerConfig                *machinev1.AlibabaCloudMachineProviderConfig
		userDataSecret                *corev1.Secret
		alibabaCloudCredentialsSecret *corev1.Secret
		expectedError                 error
	}{
		{
			testcase:                      "Create succeed",
			providerConfig:                stubProviderConfig(),
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			expectedError:                 nil,
		},
		{
			testcase:       "Bad userData",
			providerConfig: stubProviderConfig(),
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      alibabaCloudMasterUserDataSecretName,
					Namespace: defaultNamespace,
				},
				Data: map[string][]byte{
					"badKey": []byte(userDataBlob),
				},
			},
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			expectedError:                 errors.New("failed to get user data: secret /master-user-data-secret does not have userData field set. thus, no user data applied when creating an instance"),
		},
		{
			testcase: "Failed security groups return invalid config machine error",
			providerConfig: stubProviderConfigSecurityGroups(
				[]machinev1.AlibabaResourceReference{
					{
						Type: machinev1.AlibabaResourceReferenceTypeTags,
						Tags: &[]machinev1.Tag{
							{
								Key:   "Name",
								Value: "machine-sg",
							},
						},
					},
				},
			),
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			expectedError:                 errors.New("failed to create instance: error getting security groups ID: error describing securitygroup: describeSecurityGroups error"),
		},
		{
			testcase: "Failed vswitches return invalid config machine error",
			providerConfig: stubProviderConfigVSwitches(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeTags,
					Tags: &[]machinev1.Tag{
						{
							Key:   "Name",
							Value: "machine-vswitch",
						},
					},
				},
			),
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			expectedError:                 errors.New("failed to create instance: error getting vswitch ID: error describing vswitches: describeVSwitches error"),
		},
	}

	for _, tc := range testCases {
		// create fake resources
		t.Logf("testCase: %v", tc.testcase)

		encodedProviderConfig, err := alibabacloudproviderv1.RawExtensionFromProviderSpec(tc.providerConfig)
		if err != nil {
			t.Fatalf("Unexpected error")
		}
		machine, err := stubMasterMachine()
		if err != nil {
			t.Fatal(err)
		}
		machine.Spec.ProviderSpec = machinev1beta1.ProviderSpec{Value: encodedProviderConfig}

		fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, machine, tc.alibabaCloudCredentialsSecret, tc.userDataSecret)

		machineScope, err := newMachineScope(machineScopeParams{
			client:  fakeClient,
			machine: machine,
			alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
				return mockAlibabaCloudClient, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		reconciler := NewReconciler(machineScope)

		// test create
		err = reconciler.Create(context.TODO())

		if errors.Is(err, &machinecontroller.RequeueAfterError{}) {
			t.Error("RequeueAfterError should not be returned by reconciler.Create()")
		}

		if tc.expectedError != nil {
			if err == nil {
				t.Error("reconciler was expected to return error")
			}
			if err.Error() != tc.expectedError.Error() {
				t.Errorf("Expected: %v, got %v", tc.expectedError, err)
			}
		} else {
			if err != nil {
				t.Errorf("reconciler was not expected to return error: %v", err)
			}
		}
	}
}

func Test_Update(t *testing.T) {
	testCases := []struct {
		name               string
		machine            func() *machinev1beta1.Machine
		expectedError      error
		alibabacloudClient func(ctrl *gomock.Controller) alibabacloudclient.Client
	}{
		{
			name: "Successfully update the machine",
			machine: func() *machinev1beta1.Machine {
				machine, err := stubMasterMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}

				return machine
			},

			expectedError: nil,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeImages(gomock.Any()).Return(stubDescribeImagesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().ListResourceGroups(gomock.Any()).Return(stubListResourceGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeSecurityGroups(gomock.Any()).Return(stubDescribeSecurityGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeVSwitches(gomock.Any()).Return(stubDescribeVSwitchesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().TagResources(gomock.Any()).Return(&ecs.TagResourcesResponse{}, nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().RunInstances(gomock.Any()).Return(stubRunInstancesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubRunningInstanceStauts, "192.168.1.0"), nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Requeue if machine has providerID ",
			machine: func() *machinev1beta1.Machine {
				machine, err := stubMasterMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}

				machine.Spec.ProviderID = func() *string {
					providerID := "test-pid"
					return &providerID
				}()

				return machine
			},

			expectedError: &machinecontroller.RequeueAfterError{RequeueAfter: requeueAfterSeconds * time.Second},
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeImages(gomock.Any()).Return(stubDescribeImagesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().ListResourceGroups(gomock.Any()).Return(stubListResourceGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeSecurityGroups(gomock.Any()).Return(stubDescribeSecurityGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeVSwitches(gomock.Any()).Return(stubDescribeVSwitchesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().TagResources(gomock.Any()).Return(&ecs.TagResourcesResponse{}, nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().RunInstances(gomock.Any()).Return(stubRunInstancesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(&ecs.DescribeInstancesResponse{}, nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, tc.machine(), stubAlibabaCloudCredentialsSecret(), stubUserDataSecret())

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: tc.machine(),
				alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return tc.alibabacloudClient(ctrl), nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			reconciler := NewReconciler(machineScope)

			err = reconciler.Update(context.TODO())

			if tc.expectedError != nil {
				if err == nil {
					t.Error("reconciler was expected to return error")
				}

				if err.Error() != tc.expectedError.Error() {
					t.Errorf("expected: %v, got %v", tc.expectedError, err)
				}

			} else {
				if err != nil {
					t.Errorf("reconciler was not expected to return error: %v", err)
				}
			}
		})
	}
}

func Test_Delete(t *testing.T) {
	testCases := []struct {
		name               string
		machine            func() *machinev1beta1.Machine
		expectedError      error
		alibabacloudClient func(ctrl *gomock.Controller) alibabacloudclient.Client
	}{
		{
			name: "Successfully delete the machine",
			machine: func() *machinev1beta1.Machine {
				machine, err := stubMasterMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}

				return machine
			},

			expectedError: nil,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeImages(gomock.Any()).Return(stubDescribeImagesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().ListResourceGroups(gomock.Any()).Return(stubListResourceGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeSecurityGroups(gomock.Any()).Return(stubDescribeSecurityGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeVSwitches(gomock.Any()).Return(stubDescribeVSwitchesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().TagResources(gomock.Any()).Return(&ecs.TagResourcesResponse{}, nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().StopInstances(gomock.Any()).Return(&ecs.StopInstancesResponse{}, nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().RunInstances(gomock.Any()).Return(stubRunInstancesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubStoppedInstanceStatus, "192.168.1.0"), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DeleteInstances(gomock.Any()).Return(&ecs.DeleteInstancesResponse{}, nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, tc.machine(), stubAlibabaCloudCredentialsSecret(), stubUserDataSecret())

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: tc.machine(),
				alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return tc.alibabacloudClient(ctrl), nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			reconciler := NewReconciler(machineScope)

			err = reconciler.Delete(context.TODO())

			if tc.expectedError != nil {
				if err == nil {
					t.Error("reconciler was expected to return error")
				}

				if err.Error() != tc.expectedError.Error() {
					t.Errorf("expected: %v, got %v", tc.expectedError, err)
				}

			} else {
				if err != nil {
					t.Errorf("reconciler was not expected to return error: %v", err)
				}
			}
		})
	}
}

func Test_Exists(t *testing.T) {
	testCases := []struct {
		name               string
		machine            func() *machinev1beta1.Machine
		expectedError      error
		existsResult       bool
		alibabacloudClient func(ctrl *gomock.Controller) alibabacloudclient.Client
	}{
		{
			name: "Successfully find created instance",
			machine: func() *machinev1beta1.Machine {
				machine, err := stubMasterMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}

				return machine
			},
			existsResult:  true,
			expectedError: nil,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubRunningInstanceStauts, "192.168.1.1"), nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Requeue if machine has providerID and addresses are not set",
			machine: func() *machinev1beta1.Machine {
				machine, err := stubMasterMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}

				machine.Spec.ProviderID = func() *string {
					providerID := "test"
					return &providerID
				}()

				return machine
			},
			existsResult:  false,
			expectedError: &machinecontroller.RequeueAfterError{RequeueAfter: requeueAfterSeconds * time.Second},
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(&ecs.DescribeInstancesResponse{}, nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Fail to find instance",
			machine: func() *machinev1beta1.Machine {
				machine, err := stubMasterMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}

				return machine
			},
			existsResult:  false,
			expectedError: nil,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(&ecs.DescribeInstancesResponse{}, nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, tc.machine(), stubAlibabaCloudCredentialsSecret(), stubUserDataSecret())

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: tc.machine(),
				alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return tc.alibabacloudClient(ctrl), nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			reconciler := NewReconciler(machineScope)

			exists, err := reconciler.Exists(context.TODO())

			if tc.existsResult != exists {
				t.Errorf("expected reconciler tc.Exists() to return: %v, got %v", tc.existsResult, exists)
			}

			if tc.expectedError != nil {
				if err == nil {
					t.Error("reconciler was expected to return error")
				}

				if err.Error() != tc.expectedError.Error() {
					t.Errorf("expected: %v, got %v", tc.expectedError, err)
				}

			} else {
				if err != nil {
					t.Errorf("reconciler was not expected to return error: %v", err)
				}
			}
		})
	}
}

func Test_getMachineInstances(t *testing.T) {
	machine, err := stubMasterMachine()
	if err != nil {
		t.Fatalf("unable to build stub machine: %v", err)
	}

	alibabaCloudCredentialsSecret := stubAlibabaCloudCredentialsSecret()
	userDataSecret := stubUserDataSecret()

	testCases := []struct {
		testcase           string
		providerStatus     machinev1.AlibabaCloudMachineProviderStatus
		alibabacloudClient func(*gomock.Controller) alibabacloudclient.Client
		exists             bool
	}{
		{
			testcase:       "empty-status",
			providerStatus: machinev1.AlibabaCloudMachineProviderStatus{},
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockAlibabaCloudClient := mock.NewMockClient(ctrl)

				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(
					stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubRunningInstanceStauts, "192.168.0.1"),
					nil,
				).Times(1)

				return mockAlibabaCloudClient
			},
			exists: true,
		},
		{
			testcase: "instance-has-status-running",
			providerStatus: machinev1.AlibabaCloudMachineProviderStatus{
				InstanceID: &stubInstanceID,
			},
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockAlibabaCloudClient := mock.NewMockClient(ctrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(
					stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubRunningInstanceStauts, "192.168.0.1"),
					nil,
				).Times(1)

				return mockAlibabaCloudClient
			},
			exists: true,
		},
		{
			testcase: "instance-has-status-stopped",
			providerStatus: machinev1.AlibabaCloudMachineProviderStatus{
				InstanceID: &stubInstanceID,
			},
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockAlibabaCloudClient := mock.NewMockClient(ctrl)

				first := mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(
					stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubReleasedInstanceStatus, "192.168.0.1"),
					nil,
				).Times(1)

				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(
					stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubReleasedInstanceStatus, "192.168.0.1"),
					nil,
				).Times(1).After(first)

				return mockAlibabaCloudClient
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			alibabaCloudStatusRaw, err := alibabacloudproviderv1.RawExtensionFromProviderStatus(&tc.providerStatus)
			if err != nil {
				t.Fatal(err)
			}

			machineCopy := machine.DeepCopy()
			machineCopy.Status.ProviderStatus = alibabaCloudStatusRaw

			fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, machine, alibabaCloudCredentialsSecret, userDataSecret)
			mockAlibabaCloudClient := tc.alibabacloudClient(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machineCopy,
				alibabacloudClientBuilder: func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return mockAlibabaCloudClient, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			reconciler := NewReconciler(machineScope)

			instances, err := reconciler.getMachineInstances()
			if err != nil {
				t.Errorf("Unexpected error from getMachineInstances: %v", err)
			}
			if tc.exists != (len(instances) > 0) {
				t.Errorf("Expected instance exists: %t, got instances: %v", tc.exists, instances)
			}
		})
	}
}

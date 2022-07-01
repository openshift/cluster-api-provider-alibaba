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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"testing"

	alibabacloudclient "github.com/openshift/cluster-api-provider-alibaba/pkg/client"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/golang/mock/gomock"
	"github.com/openshift/cluster-api-provider-alibaba/pkg/client/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	machineapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/record"

	configv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	// Add types to scheme
	machinev1beta1.AddToScheme(scheme.Scheme)
	configv1.AddToScheme(scheme.Scheme)
}

func TestMachine(t *testing.T) {
	gs := NewWithT(t)

	alibabaCloudCredentialsSecret := stubAlibabaCloudCredentialsSecret()
	gs.Expect(k8sClient.Create(context.TODO(), alibabaCloudCredentialsSecret)).To(Succeed())
	defer func() {
		gs.Expect(k8sClient.Delete(context.TODO(), alibabaCloudCredentialsSecret)).To(Succeed())
	}()

	userDataSecret := stubUserDataSecret()
	gs.Expect(k8sClient.Create(context.TODO(), userDataSecret)).To(Succeed())
	defer func() {
		gs.Expect(k8sClient.Delete(context.TODO(), userDataSecret)).To(Succeed())
	}()

	testCases := []struct {
		name                          string
		error                         string
		operation                     func(actuator *Actuator, machine *machinev1beta1.Machine)
		userDataSecret                *corev1.Secret
		alibabaCloudCredentialsSecret *corev1.Secret
		event                         string
		invalidMachineScope           bool
		alibabacloudClient            func(ctrl *gomock.Controller) alibabacloudclient.Client
	}{
		{
			name: "Create machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Create(context.TODO(), machine)
			},
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			event:                         "InvalidConfiguration: failed to create machine \"alibabacloud-actuator-testing-master-machine\" scope: failed to create alibabacloud client: AlibabaCloud client error",
			invalidMachineScope:           true,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Create machine event succeed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Create(context.TODO(), machine)
			},
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			event:                         "Created Machine alibabacloud-actuator-testing-master-machine",
			invalidMachineScope:           false,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeImages(gomock.Any()).Return(stubDescribeImagesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().ListResourceGroups(gomock.Any()).Return(stubListResourceGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeSecurityGroups(gomock.Any()).Return(stubDescribeSecurityGroupsResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeVSwitches(gomock.Any()).Return(stubDescribeVSwitchesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().TagResources(gomock.Any()).Return(&ecs.TagResourcesResponse{}, nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().RunInstances(gomock.Any()).Return(stubRunInstancesResponse(), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesResponse(), nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Update machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Update(context.TODO(), machine)
			},
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			event:                         "InvalidConfiguration: failed to create machine \"alibabacloud-actuator-testing-master-machine\" scope: failed to create alibabacloud client: AlibabaCloud client error",
			invalidMachineScope:           true,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesResponse(), nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Update machine event succeed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Update(context.TODO(), machine)
			},
			event: "Updated Machine alibabacloud-actuator-testing-master-machine",
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesResponse(), nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Delete machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Delete(context.TODO(), machine)
			},
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			event:                         "DeleteError: failed to create machine \"alibabacloud-actuator-testing-master-machine\" scope: failed to create alibabacloud client: AlibabaCloud client error",
			invalidMachineScope:           true,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesResponse(), nil).AnyTimes()
				return mockAlibabaCloudClient
			},
		},
		{
			name: "Delete machine event succeed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Delete(context.TODO(), machine)
			},
			event: "Deleted machine \"alibabacloud-actuator-testing-master-machine\"",
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubStoppedInstanceStatus, "192.168.1.1"), nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().StopInstances(gomock.Any()).Return(&ecs.StopInstancesResponse{}, nil).AnyTimes()
				mockAlibabaCloudClient.EXPECT().DeleteInstances(gomock.Any()).Return(&ecs.DeleteInstancesResponse{}, nil).AnyTimes()

				return mockAlibabaCloudClient
			},
		},
		{
			name: "Exists machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Exists(context.TODO(), machine)
			},
			userDataSecret:                stubUserDataSecret(),
			alibabaCloudCredentialsSecret: stubAlibabaCloudCredentialsSecret(),
			event:                         "InvalidConfiguration: failed to create machine \"alibabacloud-actuator-testing-master-machine\" scope: failed to create alibabacloud client: AlibabaCloud client error",
			invalidMachineScope:           true,
			alibabacloudClient: func(ctrl *gomock.Controller) alibabacloudclient.Client {
				mockCtrl := gomock.NewController(t)
				mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
				return mockAlibabaCloudClient
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			gs := NewWithT(t)

			machine, err := stubMasterMachine()
			gs.Expect(err).ToNot(HaveOccurred())
			gs.Expect(stubMachine).ToNot(BeNil())

			// Create the machine
			gs.Expect(k8sClient.Create(ctx, machine)).To(Succeed())
			defer func() {
				gs.Expect(k8sClient.Delete(ctx, machine)).To(Succeed())
			}()

			// Ensure the machine has synced to the cache
			getMachine := func() error {
				machineKey := types.NamespacedName{Namespace: machine.Namespace, Name: machine.Name}
				return k8sClient.Get(ctx, machineKey, machine)
			}
			gs.Eventually(getMachine, timeout).Should(Succeed())

			mockCtrl := gomock.NewController(t)
			alibabacloudClientBuilder := func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
				return tc.alibabacloudClient(mockCtrl), nil
			}

			if tc.invalidMachineScope {
				alibabacloudClientBuilder = func(client runtimeclient.Client, secretName, namespace, region string, configManagedClient runtimeclient.Client) (alibabacloudclient.Client, error) {
					return nil, errors.New("AlibabaCloud client error")
				}
			}

			params := ActuatorParams{
				Client:                    k8sClient,
				EventRecorder:             eventRecorder,
				AlibabaCloudClientBuilder: alibabacloudClientBuilder,
				ReconcilerBuilder:         NewReconciler,
			}

			actuator := NewActuator(params)
			tc.operation(actuator, machine)

			eventList := &corev1.EventList{}
			waitForEvent := func() error {
				gs.Expect(k8sClient.List(ctx, eventList, client.InNamespace(machine.Namespace))).To(Succeed())
				if len(eventList.Items) != 1 {
					errorMsg := fmt.Sprintf("Expected len 1, got %d", len(eventList.Items))
					return errors.New(errorMsg)
				}
				return nil
			}

			gs.Eventually(waitForEvent, timeout).Should(Succeed())

			gs.Expect(eventList.Items[0].Message).To(Equal(tc.event))

			for i := range eventList.Items {
				gs.Expect(k8sClient.Delete(ctx, &eventList.Items[i])).To(Succeed())
			}
		})
	}
}

func TestHandleMachineErrors(t *testing.T) {
	masterMachine, err := stubMasterMachine()
	if err != nil {
		t.Fatal(err)
	}

	configs := make([]map[string]interface{}, 0)
	//create
	configs = append(configs, map[string]interface{}{
		"Name":        "Create event for create action",
		"EventAction": createEventAction,
		"Error":       machineapierrors.InvalidMachineConfiguration("failed to create machine %q scope: %v", masterMachine.Name, errors.New("failed to get machine config")),
		"Event":       fmt.Sprintf("Warning FailedCreate InvalidConfiguration: failed to create machine \"alibabacloud-actuator-testing-master-machine\" scope: %v", errors.New("failed to get machine config")),
	})

	configs = append(configs, map[string]interface{}{
		"Name":        "Create event for create action",
		"EventAction": createEventAction,
		"Error":       machineapierrors.InvalidMachineConfiguration("failed to reconcile machine %q: %v", masterMachine.Name, errors.New("failed to set machine cloud provider specifics")),
		"Event":       fmt.Sprintf("Warning FailedCreate InvalidConfiguration: failed to reconcile machine \"alibabacloud-actuator-testing-master-machine\": %v", errors.New("failed to set machine cloud provider specifics")),
	})

	for _, config := range configs {
		eventsChannel := make(chan string, 1)

		params := ActuatorParams{
			EventRecorder: &record.FakeRecorder{
				Events: eventsChannel,
			},
		}

		actuator := NewActuator(params)

		actuator.handleMachineError(masterMachine, config["Error"].(*machineapierrors.MachineError), config["EventAction"].(string))
		select {
		case event := <-eventsChannel:
			if event != config["Event"] {
				t.Errorf("Expected %q event, got %q", config["Event"], event)
			} else {
				t.Logf("ok")
			}
		}
	}
}

func getInstanceIds() string {
	instanceIds, _ := json.Marshal([]string{stubInstanceID})
	return string(instanceIds)
}

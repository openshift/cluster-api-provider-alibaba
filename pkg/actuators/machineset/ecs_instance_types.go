/*
Copyright The Kubernetes Authors.
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

package machineset

import (
	alibabacloudproviderv1 "github.com/AliyunContainerService/cluster-api-provider-alibabacloud/pkg/apis/alibabacloudprovider/v1beta1"
	alibabacloudClient "github.com/AliyunContainerService/cluster-api-provider-alibabacloud/pkg/client"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	"k8s.io/klog"
)

type instanceType struct {
	InstanceType string
	VCPU         int64
	MemoryMb     int64
	GPU          int64
}

// Check whether instanceType is correct, and return the corresponding CPU, MEM, and GPU data
func (r *Reconciler) getInstanceType(machineSet *machinev1.MachineSet, providerSpec *alibabacloudproviderv1.AlibabaCloudMachineProviderConfig) (*instanceType, bool) {
	credentialsSecretName := ""
	if providerSpec.CredentialsSecret != nil {
		credentialsSecretName = providerSpec.CredentialsSecret.Name
	}

	aliClient, err := alibabacloudClient.NewClient(r.Client, credentialsSecretName, machineSet.Namespace, providerSpec.RegionID, nil)
	if err != nil {
		klog.Errorf("Failed to create alibabacloud client: %v", err)
		return nil, false
	}

	instanceTypes := []string{providerSpec.InstanceType}
	describeInstanceTypesRequest := ecs.CreateDescribeInstanceTypesRequest()
	describeInstanceTypesRequest.RegionId = providerSpec.RegionID
	describeInstanceTypesRequest.Scheme = "https"
	describeInstanceTypesRequest.InstanceTypes = &instanceTypes

	response, err := aliClient.DescribeInstanceTypes(describeInstanceTypesRequest)
	if err != nil {
		klog.Errorf("Failed to describeInstanceTypes: %v", err)
		return nil, false
	}

	if len(response.InstanceTypes.InstanceType) <= 0 {
		klog.Errorf("%s no instanceType for given filters not found", providerSpec.InstanceType)
		return nil, false
	}

	it := response.InstanceTypes.InstanceType[0]

	return &instanceType{
		InstanceType: it.InstanceType,
		VCPU:         int64(it.CpuCoreCount),
		MemoryMb:     int64(it.MemorySize * 1024),
		GPU:          int64(it.GPUAmount),
	}, true
}

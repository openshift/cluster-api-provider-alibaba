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
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/resourcemanager"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/golang/mock/gomock"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/cluster-api-provider-alibaba/pkg/client/mock"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func createDescribeInstancesRequest(ids string) *ecs.DescribeInstancesRequest {
	request := ecs.CreateDescribeInstancesRequest()
	request.Scheme = "https"
	request.InstanceIds = ids
	return request
}

func TestRunInstances(t *testing.T) {
	machine, err := stubMasterMachine()
	if err != nil {
		t.Fatalf("Unable to build test machine manifest: %v", err)
	}

	providerConfig := stubProviderConfig()
	stubTagList := buildTagList(machine.Name, stubClusterID, providerConfig.Tags)

	cases := []struct {
		name                      string
		providerConfig            *machinev1.AlibabaCloudMachineProviderConfig
		securityGroupResponse     *ecs.DescribeSecurityGroupsResponse
		securityGroupErr          error
		vswitchesResponse         *vpc.DescribeVSwitchesResponse
		vswitchesErr              error
		imageResponse             *ecs.DescribeImagesResponse
		imageErr                  error
		listRescourceGroupRequest *resourcemanager.ListResourceGroupsRequest
		resourceGroupsResponse    *resourcemanager.ListResourceGroupsResponse
		resourceGroupesErr        error
		instancesRequest          *ecs.DescribeInstancesRequest
		instancesResponse         *ecs.DescribeInstancesResponse
		instancesErr              error
		runInstancesResponse      *ecs.RunInstancesResponse
		runInstancesErr           error
		succeeds                  bool
		runInstancesRequest       *ecs.RunInstancesRequest
	}{
		{
			name:           "Images api with  error",
			providerConfig: stubProviderConfig(),
			imageErr:       fmt.Errorf("error"),
		},
		{
			name:           "No Images found",
			providerConfig: stubProviderConfig(),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{},
				},
			},
			runInstancesErr: fmt.Errorf("error"),
		},
		{
			name:           "Images not available",
			providerConfig: stubProviderConfig(),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  "Disabled",
						},
					},
				},
			},
			runInstancesErr: fmt.Errorf("error"),
		},
		{
			name: "Security groups with ID",
			providerConfig: stubProviderConfigSecurityGroups(
				[]machinev1.AlibabaResourceReference{
					{
						Type: machinev1.AlibabaResourceReferenceTypeID,
						ID:   &stubSecurityGroupID,
					},
				},
			),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{
						{
							Id:   stubResourceGroupID,
							Name: stubResourceGroupName,
						},
					},
				},
			},
			vswitchesResponse: &vpc.DescribeVSwitchesResponse{
				VSwitches: vpc.VSwitches{
					VSwitch: []vpc.VSwitch{
						{
							VSwitchId: stubVSwitchID,
						},
					},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
			instancesRequest: createDescribeInstancesRequest(getInstanceIds()),
			instancesResponse: &ecs.DescribeInstancesResponse{
				Instances: ecs.InstancesInDescribeInstances{
					Instance: []ecs.Instance{
						{
							InstanceId: stubInstanceID,
							Status:     stubRunningInstanceStauts,
						},
					},
				},
			},
			runInstancesResponse: &ecs.RunInstancesResponse{
				InstanceIdSets: ecs.InstanceIdSets{
					InstanceIdSet: []string{stubInstanceID},
				},
			},
			succeeds: true,
			runInstancesRequest: &ecs.RunInstancesRequest{
				ImageId:         stubImageID,
				InstanceType:    stubInstanceType,
				SecurityGroupId: stubSecurityGroupID,
				VSwitchId:       stubVSwitchID,
				Tag:             covertToRunInstancesTag(stubTagList),
			},
		},
		{
			name: "Security groups with Tags",
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
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{
						{
							Id:   stubResourceGroupID,
							Name: stubResourceGroupName,
						},
					},
				},
			},
			vswitchesResponse: &vpc.DescribeVSwitchesResponse{
				VSwitches: vpc.VSwitches{
					VSwitch: []vpc.VSwitch{
						{
							VSwitchId: stubVSwitchID,
						},
					},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
			instancesRequest: createDescribeInstancesRequest(getInstanceIds()),
			instancesResponse: &ecs.DescribeInstancesResponse{
				Instances: ecs.InstancesInDescribeInstances{
					Instance: []ecs.Instance{
						{
							InstanceId: stubInstanceID,
							Status:     stubRunningInstanceStauts,
						},
					},
				},
			},
			runInstancesResponse: &ecs.RunInstancesResponse{
				InstanceIdSets: ecs.InstanceIdSets{
					InstanceIdSet: []string{stubInstanceID},
				},
			},
			succeeds: true,
			runInstancesRequest: &ecs.RunInstancesRequest{
				ImageId:         stubImageID,
				InstanceType:    stubInstanceType,
				SecurityGroupId: stubSecurityGroupID,
				VSwitchId:       stubVSwitchID,
				Tag:             covertToRunInstancesTag(stubTagList),
			},
		},
		{
			name: "No Security groups provided",
			providerConfig: stubProviderConfigSecurityGroups(
				[]machinev1.AlibabaResourceReference{},
			),
			securityGroupErr: fmt.Errorf("error"),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "No Security groups ID provided",
			providerConfig: stubProviderConfigSecurityGroups(
				[]machinev1.AlibabaResourceReference{
					{
						Type: machinev1.AlibabaResourceReferenceTypeID,
					},
				},
			),
			securityGroupErr: fmt.Errorf("error"),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter Security groups with tags  with error ",
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
			securityGroupErr: fmt.Errorf("error"),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter Security groups with empty tags  with error ",
			providerConfig: stubProviderConfigSecurityGroups(
				[]machinev1.AlibabaResourceReference{
					{
						Type: machinev1.AlibabaResourceReferenceTypeTags,
					},
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "No Security groups  found with  tags  ",
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
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{},
				},
			},
			runInstancesErr: fmt.Errorf("error"),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter Security groups with unknow type  with error ",
			providerConfig: stubProviderConfigSecurityGroups(
				[]machinev1.AlibabaResourceReference{
					{
						Type: machinev1.AlibabaResourceReferenceTypeName,
					},
				},
			),
			securityGroupErr: fmt.Errorf("error"),
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "No resourceGroup Id provided",
			providerConfig: stubProviderConfigResourceGroup(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeID,
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},

			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "resourceGroup api error ",
			providerConfig: stubProviderConfigResourceGroup(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeName,
					Name: &stubResourceGroupName,
				},
			),
			resourceGroupesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},

			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter resourceGroup by Name with empty Name error ",
			providerConfig: stubProviderConfigResourceGroup(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeName,
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},

			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter resourceGroup with wrong type",
			providerConfig: stubProviderConfigResourceGroup(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeTags,
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter resourceGroup with Name no response ",
			providerConfig: stubProviderConfigResourceGroup(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeName,
					Name: &stubResourceGroupName,
				},
			),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},

		// VSwitches
		{
			name: "Filter vswitches with ID ,but no ID provided ",
			providerConfig: stubProviderConfigVSwitches(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeID,
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter vswitches with Tags ,but no Tags provided ",
			providerConfig: stubProviderConfigVSwitches(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeTags,
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter vswitches with unknow type error  ",
			providerConfig: stubProviderConfigVSwitches(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeName,
				},
			),
			runInstancesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},
		{
			name: "Filter vswitches with tags  with api error   ",
			providerConfig: stubProviderConfigVSwitches(
				machinev1.AlibabaResourceReference{
					Type: machinev1.AlibabaResourceReferenceTypeTags,
					Tags: &[]machinev1.Tag{
						{
							Key:   "Name",
							Value: "test-vswitch",
						},
					},
				},
			),
			vswitchesErr: fmt.Errorf("error"),
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
		},

		// Run Instances
		{
			name: "Run instances api error ",
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
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{
						{
							Id:   stubResourceGroupID,
							Name: stubResourceGroupName,
						},
					},
				},
			},
			vswitchesResponse: &vpc.DescribeVSwitchesResponse{
				VSwitches: vpc.VSwitches{
					VSwitch: []vpc.VSwitch{
						{
							VSwitchId: stubVSwitchID,
						},
					},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
			instancesRequest: createDescribeInstancesRequest(getInstanceIds()),
			instancesResponse: &ecs.DescribeInstancesResponse{
				Instances: ecs.InstancesInDescribeInstances{
					Instance: []ecs.Instance{
						{
							InstanceId: stubInstanceID,
							Status:     stubRunningInstanceStauts,
						},
					},
				},
			},
			runInstancesResponse: &ecs.RunInstancesResponse{
				InstanceIdSets: ecs.InstanceIdSets{
					InstanceIdSet: []string{stubInstanceID},
				},
			},
			runInstancesErr: fmt.Errorf("error"),
			runInstancesRequest: &ecs.RunInstancesRequest{
				ImageId:         stubImageID,
				InstanceType:    stubInstanceType,
				SecurityGroupId: stubSecurityGroupID,
				VSwitchId:       stubVSwitchID,
				Tag:             covertToRunInstancesTag(stubTagList),
			},
		},
		{
			name: "Run instances : describe instances error  ",
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
			securityGroupResponse: &ecs.DescribeSecurityGroupsResponse{
				SecurityGroups: ecs.SecurityGroups{
					SecurityGroup: []ecs.SecurityGroup{
						{
							SecurityGroupId: stubSecurityGroupID,
						},
					},
				},
			},
			resourceGroupsResponse: &resourcemanager.ListResourceGroupsResponse{
				ResourceGroups: resourcemanager.ResourceGroups{
					ResourceGroup: []resourcemanager.ResourceGroup{
						{
							Id:   stubResourceGroupID,
							Name: stubResourceGroupName,
						},
					},
				},
			},
			vswitchesResponse: &vpc.DescribeVSwitchesResponse{
				VSwitches: vpc.VSwitches{
					VSwitch: []vpc.VSwitch{
						{
							VSwitchId: stubVSwitchID,
						},
					},
				},
			},
			imageResponse: &ecs.DescribeImagesResponse{
				Images: ecs.Images{
					Image: []ecs.Image{
						{
							ImageId: stubImageID,
							Status:  EcsImageStatusAvailable,
						},
					},
				},
			},
			instancesRequest: createDescribeInstancesRequest(getInstanceIds()),
			instancesResponse: &ecs.DescribeInstancesResponse{
				Instances: ecs.InstancesInDescribeInstances{
					Instance: []ecs.Instance{},
				},
			},
			runInstancesResponse: &ecs.RunInstancesResponse{
				InstanceIdSets: ecs.InstanceIdSets{
					InstanceIdSet: []string{stubInstanceID},
				},
			},
			runInstancesRequest: &ecs.RunInstancesRequest{
				ImageId:         stubImageID,
				InstanceType:    stubInstanceType,
				SecurityGroupId: stubSecurityGroupID,
				VSwitchId:       stubVSwitchID,
				Tag:             covertToRunInstancesTag(stubTagList),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)

			mockAlibabaCloudClient.EXPECT().DescribeImages(gomock.Any()).Return(tc.imageResponse, tc.imageErr).AnyTimes()
			mockAlibabaCloudClient.EXPECT().ListResourceGroups(gomock.Any()).Return(tc.resourceGroupsResponse, tc.resourceGroupesErr).AnyTimes()
			mockAlibabaCloudClient.EXPECT().DescribeSecurityGroups(gomock.Any()).Return(tc.securityGroupResponse, tc.securityGroupErr).AnyTimes()
			mockAlibabaCloudClient.EXPECT().DescribeVSwitches(gomock.Any()).Return(tc.vswitchesResponse, tc.vswitchesErr).AnyTimes()
			mockAlibabaCloudClient.EXPECT().RunInstances(gomock.Any()).Return(tc.runInstancesResponse, tc.runInstancesErr).AnyTimes()
			mockAlibabaCloudClient.EXPECT().DescribeInstances(gomock.Any()).Return(tc.instancesResponse, tc.instancesErr).AnyTimes()

			_, runErr := runInstances(machine, tc.providerConfig, "", mockAlibabaCloudClient)
			t.Log(runErr)
			if runErr == nil {
				if !tc.succeeds {
					t.Errorf("Call to runInstances did not fail as expected")
				}
			} else {
				if tc.succeeds {
					t.Errorf("Call to runInstances did not succeed as expected")
				}
			}
		})
	}
}

func TestBuildDescribeSecurityGroupsTag(t *testing.T) {
	cases := []struct {
		tagList  []machinev1.Tag
		expected []ecs.DescribeSecurityGroupsTag
	}{
		{
			// empty tags
			tagList:  []machinev1.Tag{},
			expected: []ecs.DescribeSecurityGroupsTag{},
		},
		{
			tagList: []machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
			expected: []ecs.DescribeSecurityGroupsTag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
		},
		{
			// multiple duplicate tags
			tagList: []machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
				{Key: "clusterSize", Value: "test-ClusterSizeDuplicatedValue"},
			},
			expected: []ecs.DescribeSecurityGroupsTag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
			},
		},
	}

	for i, c := range cases {
		actual := buildDescribeSecurityGroupsTag(c.tagList)
		if !reflect.DeepEqual(&c.expected, actual) {
			t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, actual)
		}
	}
}

func TestBuildDescribeVSwitchesTag(t *testing.T) {
	cases := []struct {
		tagList  []machinev1.Tag
		expected []vpc.DescribeVSwitchesTag
	}{
		{
			// empty tags
			tagList:  []machinev1.Tag{},
			expected: []vpc.DescribeVSwitchesTag{},
		},
		{
			tagList: []machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
			expected: []vpc.DescribeVSwitchesTag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
		},
		{
			// multiple duplicate tags
			tagList: []machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
				{Key: "clusterSize", Value: "test-ClusterSizeDuplicatedValue"},
			},
			expected: []vpc.DescribeVSwitchesTag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
			},
		},
	}

	for i, c := range cases {
		actual := buildDescribeVSwitchesTag(c.tagList)
		if !reflect.DeepEqual(&c.expected, actual) {
			t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, actual)
		}
	}
}

func TestBuildTagList(t *testing.T) {
	cases := []struct {
		name            string
		machineSpecTags []machinev1beta1.TagSpecification
		expected        []*machinev1.Tag
	}{
		{
			name:            "with empty  provider spec should return default tags",
			machineSpecTags: []machinev1beta1.TagSpecification{},
			expected: []*machinev1.Tag{
				{Key: "kubernetes.io/cluster/clusterID", Value: "owned"},
				{Key: "Name", Value: "machineName"},
				{Key: clusterOwnedKey, Value: clusterOwnedValue},
				{Key: machineTagKeyFrom, Value: machineTagValueFrom},
				{Key: machineIsvIntegrationTagKey, Value: machineTagValueFrom},
			},
		},
		{
			name: "should filter out bad tags from provider spec",
			machineSpecTags: []machinev1beta1.TagSpecification{
				{Name: "Name", Value: "badname"},
				{Name: "kubernetes.io/cluster/badid", Value: "badvalue"},
				{Name: "good", Value: "goodvalue"},
			},
			// Invalid tags get dropped and the valid clusterID and Name get applied last.
			expected: []*machinev1.Tag{
				{Key: "good", Value: "goodvalue"},
				{Key: "kubernetes.io/cluster/clusterID", Value: "owned"},
				{Key: "Name", Value: "machineName"},
				{Key: clusterOwnedKey, Value: clusterOwnedValue},
				{Key: machineTagKeyFrom, Value: machineTagValueFrom},
				{Key: machineIsvIntegrationTagKey, Value: machineTagValueFrom},
			},
		},
		{
			name: "tags from machine object should have precedence",
			machineSpecTags: []machinev1beta1.TagSpecification{
				{Name: "Name", Value: "badname"},
				{Name: "kubernetes.io/cluster/badid", Value: "badvalue"},
				{Name: "good", Value: "goodvalue"},
			},
			expected: []*machinev1.Tag{
				{Key: "good", Value: "goodvalue"},
				{Key: "kubernetes.io/cluster/clusterID", Value: "owned"},
				{Key: "Name", Value: "machineName"},
				{Key: clusterOwnedKey, Value: clusterOwnedValue},
				{Key: machineTagKeyFrom, Value: machineTagValueFrom},
				{Key: machineIsvIntegrationTagKey, Value: machineTagValueFrom},
			},
		},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var machineTags []machinev1.Tag
			for _, machineSpecTag := range c.machineSpecTags {
				machineTags = append(machineTags, machinev1.Tag{
					Key:   machineSpecTag.Name,
					Value: machineSpecTag.Value,
				})
			}

			actual := buildTagList("machineName", "clusterID", machineTags)
			if !reflect.DeepEqual(c.expected, actual) {
				t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, actual)
			}
		})
	}
}

func TestRemoveDuplicatedTags(t *testing.T) {
	cases := []struct {
		tagList  []*machinev1.Tag
		expected []*machinev1.Tag
	}{
		{
			// empty tags
			tagList:  []*machinev1.Tag{},
			expected: []*machinev1.Tag{},
		},
		{
			// no duplicate tags
			tagList: []*machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
			expected: []*machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
		},
		{
			// multiple duplicate tags
			tagList: []*machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
				{Key: "clusterSize", Value: "test-ClusterSizeDuplicatedValue"},
			},
			expected: []*machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
			},
		},
	}

	for i, c := range cases {
		actual := removeDuplicatedTags(c.tagList)
		if !reflect.DeepEqual(c.expected, actual) {
			t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, actual)
		}
	}
}

func TestCovertToRunInstancesTag(t *testing.T) {
	cases := []struct {
		tagList  []*machinev1.Tag
		expected []ecs.RunInstancesTag
	}{
		{
			// empty tags
			tagList:  []*machinev1.Tag{},
			expected: []ecs.RunInstancesTag{},
		},
		{
			tagList: []*machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
			expected: []ecs.RunInstancesTag{
				{Key: "clusterID", Value: "test-ClusterID"},
			},
		},
		{
			// multiple duplicate tags
			tagList: []*machinev1.Tag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
				{Key: "clusterSize", Value: "test-ClusterSizeDuplicatedValue"},
			},
			expected: []ecs.RunInstancesTag{
				{Key: "clusterID", Value: "test-ClusterID"},
				{Key: "clusterSize", Value: "test-ClusterSize"},
				{Key: "clusterSize", Value: "test-ClusterSizeDuplicatedValue"},
			},
		},
	}

	for i, c := range cases {
		actual := covertToRunInstancesTag(c.tagList)
		if !reflect.DeepEqual(&c.expected, actual) {
			t.Errorf("test #%d: expected %+v, got %+v", i, c.expected, actual)
		}
	}
}

func TestInstanceHasSupportedState(t *testing.T) {
	cases := []struct {
		instance       *ecs.Instance
		instanceStates []string
		expected       bool
	}{
		// empty instanceId
		{
			&ecs.Instance{
				InstanceId: "",
			},
			[]string{},
			true,
		},
		// empty Status
		{
			&ecs.Instance{
				Status: "",
			},
			[]string{},
			true,
		},
		// empty instanceStates
		{
			&ecs.Instance{
				InstanceId: stubInstanceID,
				Status:     stubInstanceStatus,
			},
			[]string{},
			false,
		},
	}

	for _, c := range cases {
		err := instanceHasSupportedState(c.instance, c.instanceStates)
		assert.Equal(t, c.expected, err != nil)
	}
}

func TestSortInstances(t *testing.T) {
	instances := []*ecs.Instance{
		{
			InstanceId: "i-abc",
			StartTime:  "2020-01-02T15:04:05Z",
		},
		{
			InstanceId: "i-abd",
			StartTime:  "2022-05-02T15:04:05Z",
		},
		{
			InstanceId: "i-abe",
			StartTime:  "2024-05-02T17:04:05Z",
		},
		{
			InstanceId: "i-abf",
			StartTime:  "",
		},
	}
	sortInstances(instances)

	assert.Equal(t, "i-abf", instances[0].InstanceId)
	assert.Equal(t, "i-abe", instances[1].InstanceId)
	assert.Equal(t, "i-abd", instances[2].InstanceId)
	assert.Equal(t, "i-abc", instances[3].InstanceId)

}

func TestCorrectExistingTags(t *testing.T) {
	machine, err := stubMachine(stubMasterMachineName, nil)
	if err != nil {
		t.Fatalf("Unable to build test machine manifest: %v", err)
	}
	clusterID, _ := getClusterID(machine)
	instance := ecs.Instance{
		InstanceId: stubInstanceID,
	}
	testCases := []struct {
		name               string
		tags               []ecs.Tag
		expectedCreateTags bool
	}{
		{
			name: "Valid Tags",
			tags: []ecs.Tag{
				{
					TagKey:   "kubernetes.io/cluster/" + clusterID,
					TagValue: "owned",
				},
				{
					TagKey:   "Name",
					TagValue: machine.Name,
				},
				{
					TagKey:   clusterOwnedKey,
					TagValue: clusterOwnedValue,
				},
			},
			expectedCreateTags: false,
		},
		{
			name: "Invalid Name Tag Correct Cluster",
			tags: []ecs.Tag{
				{
					TagKey:   "kubernetes.io/cluster/" + clusterID,
					TagValue: "owned",
				},
				{
					TagKey:   "Name",
					TagValue: "badname",
				},
			},
			expectedCreateTags: true,
		},
		{
			name: "Invalid Cluster Tag Correct Name",
			tags: []ecs.Tag{
				{
					TagKey:   "kubernetes.io/cluster/" + "badcluster",
					TagValue: "owned",
				},
				{
					TagKey:   "Name",
					TagValue: machine.Name,
				},
			},
			expectedCreateTags: true,
		},
		{
			name: "Both Tags Wrong",
			tags: []ecs.Tag{
				{
					TagKey:   "kubernetes.io/cluster/" + clusterID,
					TagValue: "bad value",
				},
				{
					TagKey:   "Name",
					TagValue: "bad name",
				},
			},
			expectedCreateTags: true,
		},
		{
			name:               "No Tags",
			tags:               nil,
			expectedCreateTags: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockAlibabaCloudClient := mock.NewMockClient(mockCtrl)
			instance.Tags = ecs.TagsInDescribeInstances{
				Tag: tc.tags,
			}

			if tc.expectedCreateTags {
				mockAlibabaCloudClient.EXPECT().TagResources(gomock.Any()).Return(&ecs.TagResourcesResponse{}, nil).MinTimes(1)
			}

			err := correctExistingTags(machine, "", &instance, mockAlibabaCloudClient)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

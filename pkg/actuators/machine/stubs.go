package machine

import (
	"fmt"
	"os"
	"strconv"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/resourcemanager"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"

	alibabacloudproviderv1 "github.com/openshift/cluster-api-provider-alibaba/pkg/apis/alibabacloudprovider/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultNamespace                     = "default"
	stubZoneID                           = "cn-beijing-f"
	stubRegionID                         = "cn-beijing"
	alibabaCloudCredentialsSecretName    = "alibabacloud-credentials-secret"
	alibabaCloudMasterUserDataSecretName = "master-user-data-secret"
	alibabaCloudWorkerUserDataSecretName = "worker-user-data-secret"

	stubMasterMachineName = "alibabacloud-actuator-testing-master-machine"
	stubWorkerMachineName = "alibabacloud-actuator-testing-worker-machine"

	stubKeyName                 = "alibabacloud-actuator-key-name"
	stubClusterID               = "alibabacloud-actuator-cluster"
	stubImageID                 = "centos_7_9_x64_20G_alibase_20210318.vhd"
	stubVpcID                   = "vpc-vk6f1qfd3w77gnmh"
	stubVSwitchID               = "vsw-sc0w64w2s3d9s8cu"
	stubInstanceID              = "i-bg2ss7v5ck5skyp9"
	stubSecurityGroupID         = "sg-h8ympu5av8hhtwks"
	stubResourceGroupID         = "rg-6ljxzbpksxaa0buw"
	stubResourceGroupName       = "test-rg"
	stubSystemDiskCategory      = "cloud_essd"
	stubSystemDiskSize          = 120
	stubInternetMaxBandwidthOut = 100
	stubPassword                = "Hello$1234"
	stubInstanceType            = "ecs.c6.2xlarge"
	stubInstanceStatus          = "Running"

	stubRunningInstanceStauts  = ECSInstanceStatusRunning
	stubPendingInstanceStatus  = ECSInstanceStatusPending
	stubStartingInstanceStatus = ECSInstanceStatusStarting
	stubStoppingInstanceStatus = ECSInstanceStatusStopping
	stubStoppedInstanceStatus  = ECSInstanceStatusStopped
	stubReleasedInstanceStatus = "Released"
)

const userDataBlob = `#!/bin/bash
echo "test"
`

func stubUserDataSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alibabaCloudMasterUserDataSecretName,
			Namespace: defaultNamespace,
		},
		Data: map[string][]byte{
			userDataSecretKey: []byte(userDataBlob),
		},
	}
}

func stubAlibabaCloudCredentialsSecret() *corev1.Secret {
	return generateAlibabaCloudCredentialsSecretFromEnv(alibabaCloudCredentialsSecretName, defaultNamespace)
}

func generateAlibabaCloudCredentialsSecretFromEnv(secretName, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"accessKeyID":     []byte(os.Getenv("ALIBABACLOUD_ACCESS_KEY_ID")),
			"accessKeySecret": []byte(os.Getenv("ALIBABACLOUD_SECRET_ACCESS_KEY")),
		},
	}
}

func stubProviderConfigSecurityGroups(groups []machinev1.AlibabaResourceReference) *machinev1.AlibabaCloudMachineProviderConfig {
	pc := stubProviderConfig()
	pc.SecurityGroups = groups
	return pc
}

func stubProviderConfigResourceGroup(group machinev1.AlibabaResourceReference) *machinev1.AlibabaCloudMachineProviderConfig {
	pc := stubProviderConfig()
	pc.SecurityGroups = []machinev1.AlibabaResourceReference{
		{
			Type: machinev1.AlibabaResourceReferenceTypeTags,
			Tags: &[]machinev1.Tag{
				{
					Key:   "Name",
					Value: "test-sg",
				},
			},
		},
	}
	pc.ResourceGroup = group
	return pc
}

func stubProviderConfigVSwitches(group machinev1.AlibabaResourceReference) *machinev1.AlibabaCloudMachineProviderConfig {
	pc := stubProviderConfig()
	pc.VSwitch = group
	return pc
}

func stubProviderConfig() *machinev1.AlibabaCloudMachineProviderConfig {
	return &machinev1.AlibabaCloudMachineProviderConfig{
		InstanceType: stubInstanceType,
		ImageID:      stubImageID,
		RegionID:     stubRegionID,
		ZoneID:       stubZoneID,
		SecurityGroups: []machinev1.AlibabaResourceReference{
			{
				Type: machinev1.AlibabaResourceReferenceTypeID,
				ID:   &stubSecurityGroupID,
			},
		},
		ResourceGroup: machinev1.AlibabaResourceReference{
			Type: machinev1.AlibabaResourceReferenceTypeID,
			ID:   &stubResourceGroupID,
		},
		VpcID: stubVpcID,
		VSwitch: machinev1.AlibabaResourceReference{
			Type: machinev1.AlibabaResourceReferenceTypeID,
			ID:   &stubVSwitchID,
		},
		SystemDisk: machinev1.SystemDiskProperties{
			Category: stubSystemDiskCategory,
			Size:     int64(stubSystemDiskSize),
		},
		DataDisks: []machinev1.DataDiskProperties{
			{
				Size:             100,
				Category:         "cloud_ssd",
				DiskEncryption:   machinev1.AlibabaDiskEncryptionDisabled,
				Name:             "my-disk",
				SnapshotID:       "sp-xxx",
				PerformanceLevel: "p2",
				DiskPreservation: machinev1.DeleteWithInstance,
			},
		},
		Bandwidth: machinev1.BandwidthProperties{
			InternetMaxBandwidthOut: int64(stubInternetMaxBandwidthOut),
		},
		UserDataSecret: &corev1.LocalObjectReference{
			Name: alibabaCloudMasterUserDataSecretName,
		},
		CredentialsSecret: &corev1.LocalObjectReference{
			Name: alibabaCloudCredentialsSecretName,
		},
		Tags: []machinev1.Tag{
			{Key: "openshift-node-group-config", Value: "node-config-master"},
			{Key: "host-type", Value: "master"},
			{Key: "sub-host-type", Value: "default"},
		},
	}
}

func stubMasterMachine() (*machinev1beta1.Machine, error) {
	masterMachine, err := stubMachine(stubMasterMachineName, map[string]string{
		"node-role.kubernetes.io/master": "",
		"node-role.kubernetes.io/infra":  "",
	})

	if err != nil {
		return nil, err
	}

	return masterMachine, nil
}

func stubWorkerMachine() (*machinev1beta1.Machine, error) {
	workerMachine, err := stubMachine(stubWorkerMachineName, map[string]string{
		"node-role.kubernetes.io/infra": "",
	})

	if err != nil {
		return nil, err
	}

	return workerMachine, nil
}

func stubMachine(machineName string, machineLabels map[string]string) (*machinev1beta1.Machine, error) {
	machineSpec := stubProviderConfig()

	providerSpec, err := alibabacloudproviderv1.RawExtensionFromProviderSpec(machineSpec)
	if err != nil {
		return nil, fmt.Errorf("codec.EncodeProviderSpec failed: %v", err)
	}

	machine := &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: defaultNamespace,
			Labels: map[string]string{
				machinev1beta1.MachineClusterIDLabel: stubClusterID,
			},
			Annotations: map[string]string{
				// skip node draining since it's not mocked
				machinecontroller.ExcludeNodeDrainingAnnotation: "",
			},
		},
		Spec: machinev1beta1.MachineSpec{
			ObjectMeta: machinev1beta1.ObjectMeta{
				Labels: machineLabels,
			},
			ProviderSpec: machinev1beta1.ProviderSpec{
				Value: providerSpec,
			},
		},
	}
	return machine, nil
}

func stubRunInstancesRequest() *ecs.RunInstancesRequest {
	request := ecs.CreateRunInstancesRequest()
	request.Scheme = "https"
	request.RegionId = stubRegionID
	request.InstanceType = stubInstanceType
	request.ImageId = stubImageID
	request.VSwitchId = stubVSwitchID
	request.SecurityGroupId = stubSecurityGroupID
	request.Password = stubPassword
	request.MinAmount = requests.NewInteger(1)
	request.Amount = requests.NewInteger(1)

	request.SystemDiskCategory = stubSystemDiskCategory
	request.SystemDiskSize = strconv.Itoa(stubSystemDiskSize)

	return request
}

func stubRunInstancesResponse() *ecs.RunInstancesResponse {
	response := ecs.CreateRunInstancesResponse()
	response.InstanceIdSets = ecs.InstanceIdSets{
		InstanceIdSet: []string{stubInstanceID},
	}

	return response
}

func stubDescribeInstancesResponse() *ecs.DescribeInstancesResponse {
	return stubDescribeInstancesWithParamsResponse(stubImageID, stubInstanceID, stubRunningInstanceStauts, "192.168.1.0")
}

func stubDescribeInstancesWithParamsResponse(imageID, instanceID string, state string, privateIP string) *ecs.DescribeInstancesResponse {
	return &ecs.DescribeInstancesResponse{
		Instances: ecs.InstancesInDescribeInstances{
			Instance: []ecs.Instance{
				{
					ImageId:    imageID,
					InstanceId: instanceID,
					Status:     state,
					RegionId:   stubRegionID,
					NetworkInterfaces: ecs.NetworkInterfacesInDescribeInstances{
						NetworkInterface: []ecs.NetworkInterface{
							{
								PrivateIpSets: ecs.PrivateIpSetsInDescribeInstances{
									PrivateIpSet: []ecs.PrivateIpSet{
										{
											PrivateIpAddress: privateIP,
										},
									},
								},
							},
						},
					},
					Tags: ecs.TagsInDescribeInstances{
						Tag: []ecs.Tag{
							{
								TagKey:   clusterFilterName,
								TagValue: stubMasterMachineName,
								Key:      clusterFilterName,
								Value:    stubMasterMachineName,
							},
							{
								Key:      clusterOwnedKey,
								Value:    clusterOwnedValue,
								TagKey:   clusterOwnedKey,
								TagValue: clusterOwnedValue,
							},
							{
								Key:      clusterFilterKeyPrefix + stubClusterID,
								Value:    clusterFilterValue,
								TagKey:   clusterFilterKeyPrefix + stubClusterID,
								TagValue: clusterFilterValue,
							},
						},
					},
				},
			},
		},
	}
}

func stubDescribeImagesResponse() *ecs.DescribeImagesResponse {
	return &ecs.DescribeImagesResponse{
		Images: ecs.Images{
			Image: []ecs.Image{
				{
					ImageId: stubImageID,
					Status:  EcsImageStatusAvailable,
				},
			},
		},
	}
}

func stubDescribeVSwitchesResponse() *vpc.DescribeVSwitchesResponse {
	return &vpc.DescribeVSwitchesResponse{
		VSwitches: vpc.VSwitches{
			VSwitch: []vpc.VSwitch{
				{
					VSwitchId: stubVSwitchID,
				},
			},
		},
	}
}

func stubListResourceGroupsResponse() *resourcemanager.ListResourceGroupsResponse {
	return &resourcemanager.ListResourceGroupsResponse{
		ResourceGroups: resourcemanager.ResourceGroups{
			ResourceGroup: []resourcemanager.ResourceGroup{
				{
					Id:   stubResourceGroupID,
					Name: stubResourceGroupName,
				},
			},
		},
	}
}

func stubDescribeSecurityGroupsResponse() *ecs.DescribeSecurityGroupsResponse {
	return &ecs.DescribeSecurityGroupsResponse{
		SecurityGroups: ecs.SecurityGroups{
			SecurityGroup: []ecs.SecurityGroup{
				{
					SecurityGroupId: stubSecurityGroupID,
				},
			},
		},
	}
}

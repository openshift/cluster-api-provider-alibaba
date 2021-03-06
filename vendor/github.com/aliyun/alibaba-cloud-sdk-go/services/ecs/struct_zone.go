package ecs

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

// Zone is a nested struct in ecs response
type Zone struct {
	ZoneNo                      string                                      `json:"ZoneNo" xml:"ZoneNo"`
	ZoneId                      string                                      `json:"ZoneId" xml:"ZoneId"`
	LocalName                   string                                      `json:"LocalName" xml:"LocalName"`
	ZoneType                    string                                      `json:"ZoneType" xml:"ZoneType"`
	AvailableResourceCreation   AvailableResourceCreation                   `json:"AvailableResourceCreation" xml:"AvailableResourceCreation"`
	AvailableVolumeCategories   AvailableVolumeCategories                   `json:"AvailableVolumeCategories" xml:"AvailableVolumeCategories"`
	AvailableInstanceTypes      AvailableInstanceTypesInDescribeZones       `json:"AvailableInstanceTypes" xml:"AvailableInstanceTypes"`
	AvailableDedicatedHostTypes AvailableDedicatedHostTypes                 `json:"AvailableDedicatedHostTypes" xml:"AvailableDedicatedHostTypes"`
	NetworkTypes                NetworkTypesInDescribeRecommendInstanceType `json:"NetworkTypes" xml:"NetworkTypes"`
	DedicatedHostGenerations    DedicatedHostGenerations                    `json:"DedicatedHostGenerations" xml:"DedicatedHostGenerations"`
	AvailableDiskCategories     AvailableDiskCategories                     `json:"AvailableDiskCategories" xml:"AvailableDiskCategories"`
	AvailableResources          AvailableResourcesInDescribeZones           `json:"AvailableResources" xml:"AvailableResources"`
}

package resourcemanager

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

// Account is a nested struct in resourcemanager response
type Account struct {
	ModifyTime            string `json:"ModifyTime" xml:"ModifyTime"`
	DelegationEnabledTime string `json:"DelegationEnabledTime" xml:"DelegationEnabledTime"`
	JoinTime              string `json:"JoinTime" xml:"JoinTime"`
	FolderId              string `json:"FolderId" xml:"FolderId"`
	DisplayName           string `json:"DisplayName" xml:"DisplayName"`
	AccountId             string `json:"AccountId" xml:"AccountId"`
	ServicePrincipal      string `json:"ServicePrincipal" xml:"ServicePrincipal"`
	AccountName           string `json:"AccountName" xml:"AccountName"`
	IdentityInformation   string `json:"IdentityInformation" xml:"IdentityInformation"`
	RecordId              string `json:"RecordId" xml:"RecordId"`
	Status                string `json:"Status" xml:"Status"`
	JoinMethod            string `json:"JoinMethod" xml:"JoinMethod"`
	ResourceDirectoryId   string `json:"ResourceDirectoryId" xml:"ResourceDirectoryId"`
	Type                  string `json:"Type" xml:"Type"`
}

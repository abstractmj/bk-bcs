/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CloudIPSpec defines the desired state of CloudIP
type CloudIPSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Address      string `json:"address"`
	VpcID        string `json:"vpcID"`
	Region       string `json:"region"`
	SubnetID     string `json:"subnetID"`
	SubnetCidr   string `json:"subnetCidr"`
	Cluster      string `json:"cluster"`
	Namespace    string `json:"namespace"`
	PodName      string `json:"podName"`
	WorkloadName string `json:"workloadName"`
	WorkloadKind string `json:"workloadKind"`
	ContainerID  string `json:"containerID"`
	Host         string `json:"host"`
	EniID        string `json:"eniID"`
	IsFixed      bool   `json:"isFixed"`
	KeepDuration string `json:"keepDuration"`
}

// CloudIPStatus defines the observed state of CloudIP
type CloudIPStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// CloudIP is the Schema for the cloudips API
type CloudIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudIPSpec   `json:"spec,omitempty"`
	Status CloudIPStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// CloudIPList contains a list of CloudIP
type CloudIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudIP{}, &CloudIPList{})
}

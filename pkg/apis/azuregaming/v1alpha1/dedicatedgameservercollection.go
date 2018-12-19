package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DedicatedGameServerCollection describes a DedicatedGameServerCollection resource
type DedicatedGameServerCollection struct {
	// TypeMeta is the metadata for the resource, like kind and apiversion
	meta_v1.TypeMeta `json:",inline"`
	// ObjectMeta contains the metadata for the particular object, including
	// things like...
	//  - name
	//  - namespace
	//  - self link
	//  - labels
	//  - ... etc ...
	meta_v1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the custom resource spec
	Spec   DedicatedGameServerCollectionSpec   `json:"spec"`
	Status DedicatedGameServerCollectionStatus `json:"status"`
}

// DedicatedGameServerCollectionSpec is the spec for a DedicatedGameServerCollection resource
type DedicatedGameServerCollectionSpec struct {
	// Message and SomeValue are example custom spec fields
	//
	// this is where you would put your custom resource data
	Replicas        int32                           `json:"replicas"`
	PortsToExpose   []int32                         `json:"portsToExpose"`
	Template        corev1.PodSpec                  `json:"template"`
	DGSFailBehavior DedicatedGameServerFailBehavior `json:"dgsFailBehavior,omitempty"`
	DGSMaxFailures  int32                           `json:"dgsMaxFailures,omitempty"`

	DGSActivePlayersAutoScalerDetails *DGSActivePlayersAutoScalerDetails `json:"dgsActivePlayersAutoScalerDetails,omitempty"`
}

// DGSActivePlayersAutoScalerDetails contains details about the autoscaling of the dedicated game server collection
type DGSActivePlayersAutoScalerDetails struct {
	MinimumReplicas            int    `json:"minimumReplicas"`
	MaximumReplicas            int    `json:"maximumReplicas"`
	ScaleInThreshold           int    `json:"scaleInThreshold"`
	ScaleOutThreshold          int    `json:"scaleOutThreshold"`
	Enabled                    bool   `json:"enabled"`
	CoolDownInMinutes          int    `json:"coolDownInMinutes"`
	LastScaleOperationDateTime string `json:"lastScaleOperationDateTime"`
	MaxPlayersPerServer        int    `json:"maxPlayersPerServer"`
}

// DedicatedGameServerCollectionStatus is the status for a DedicatedGameServerCollection resource
type DedicatedGameServerCollectionStatus struct {
	DGSTimesFailed      int32           `json:"dgsTimesFailed"`
	AvailableReplicas   int32           `json:"availableReplicas"`
	PodCollectionState  corev1.PodPhase `json:"podsState"`
	DGSCollectionHealth DGSColHealth    `json:"dgsHealth"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DedicatedGameServerCollectionList is a list of DedicatedGameServerCollectionList resources
type DedicatedGameServerCollectionList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`

	Items []DedicatedGameServerCollection `json:"items"`
}

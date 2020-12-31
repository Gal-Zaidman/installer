// Package ovirt contains ovirt-specific Terraform-variable logic.
package ovirt

import (
	"encoding/json"

	"github.com/openshift/cluster-api-provider-ovirt/pkg/apis/ovirtprovider/v1beta1"

	"github.com/openshift/installer/pkg/rhcos"
	"github.com/openshift/installer/pkg/tfvars/internal/cache"
)

// Auth is the collection of credentials that will be used by terrform.
type Auth struct {
	URL      string `json:"ovirt_url"`
	Username string `json:"ovirt_username"`
	Password string `json:"ovirt_password"`
	Cafile   string `json:"ovirt_cafile,omitempty"`
}

type config struct {
	Auth                       `json:",inline"`
	ClusterID                  string   `json:"ovirt_cluster_id"`
	StorageDomainID            string   `json:"ovirt_storage_domain_id"`
	NetworkName                string   `json:"ovirt_network_name,omitempty"`
	VNICProfileID              string   `json:"ovirt_vnic_profile_id,omitempty"`
	MasterAffinityGroupsNames  []string `json:"ovirt_masters_affinity_groups_names,omitempty"`
	ClusterAffinityGroupsNames []string `json:"ovirt_cluster_affinity_groups_names,omitempty"`
	BaseImageName              string   `json:"openstack_base_image_name,omitempty"`
	BaseImageLocalFilePath     string   `json:"openstack_base_image_local_file_path,omitempty"`
	MasterInstanceTypeID       string   `json:"ovirt_master_instance_type_id"`
	MasterVMType               string   `json:"ovirt_master_vm_type,omitempty"`
	MasterMemory               int32    `json:"ovirt_master_memory"`
	MasterCores                int32    `json:"ovirt_master_cores"`
	MasterSockets              int32    `json:"ovirt_master_sockets"`
	MasterOsDiskGB             int64    `json:"ovirt_master_os_disk_gb"`
}

// TFVars generates ovirt-specific Terraform variables.
func TFVars(
	auth Auth,
	clusterID string,
	storageDomainID string,
	networkName string,
	vnicProfileID string,
	baseImage string,
	infraID string,
	masterSpec *v1beta1.OvirtMachineProviderSpec,
	workers []*v1beta1.OvirtMachineProviderSpec,
) ([]byte, error) {
	cfg := config{
		Auth:                 auth,
		ClusterID:            clusterID,
		StorageDomainID:      storageDomainID,
		NetworkName:          networkName,
		VNICProfileID:        vnicProfileID,
		BaseImageName:        baseImage,
		MasterInstanceTypeID: masterSpec.InstanceTypeId,
		MasterVMType:         masterSpec.VMType,
		MasterOsDiskGB:       masterSpec.OSDisk.SizeGB,
		MasterMemory:         masterSpec.MemoryMB,
	}
	if masterSpec.CPU != nil {
		cfg.MasterCores = masterSpec.CPU.Cores
		cfg.MasterSockets = masterSpec.CPU.Sockets
	}

	imageName, isURL := rhcos.GenerateOpenStackImageName(baseImage, infraID)
	cfg.BaseImageName = imageName
	if isURL {
		imageFilePath, err := cache.DownloadImageFile(baseImage)
		if err != nil {
			return nil, err
		}
		cfg.BaseImageLocalFilePath = imageFilePath
	}
	cfg.MasterAffinityGroupsNames = createMasterAffinityGroupsNames(masterSpec.AffinityGroupsNames)
	cfg.ClusterAffinityGroupsNames = createClusterAffinityGroupsNames(
		workers, masterSpec.AffinityGroupsNames)
	return json.MarshalIndent(cfg, "", "  ")
}

func createMasterAffinityGroupsNames(mastersAffinityGroup []string) []string {
	affinityGroupsNames := make([]string, 0)
	for _, mastersAGName := range mastersAffinityGroup {
		if mastersAGName != "" {
			affinityGroupsNames = append(affinityGroupsNames, mastersAGName)
		}
	}
	return affinityGroupsNames
}

func createClusterAffinityGroupsNames(workers []*v1beta1.OvirtMachineProviderSpec, mastersAffinityGroup []string) []string {
	uniqeAGName := make(map[string]bool)
	for _, pool := range workers {
		for _, poolAGName := range pool.AffinityGroupsNames {
			if poolAGName != "" {
				uniqeAGName[poolAGName] = true
			}
		}
	}
	// If the affinity group exist on master list don't create it twice
	for _, mastersAGName := range mastersAffinityGroup {
		if _, ok := uniqeAGName[mastersAGName]; ok {
			delete(uniqeAGName, mastersAGName)
		}
	}
	affinityGroupsNames := make([]string, len(uniqeAGName))
	for k := range uniqeAGName {
		affinityGroupsNames = append(affinityGroupsNames, k)
	}
	return affinityGroupsNames
}

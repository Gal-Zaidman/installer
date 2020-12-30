package defaults

import (
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/ovirt"
)

// DefaultNetworkName is the default network name to use in a cluster
const DefaultNetworkName = "ovirtmgmt"

// SetPlatformDefaults sets the defaults for the platform.
func SetPlatformDefaults(p *ovirt.Platform, c *types.InstallConfig) {
	if p.NetworkName == "" {
		p.NetworkName = DefaultNetworkName
	}
	if p.AffinityGroupsNames == nil {
		p.AffinityGroupsNames = []string{
			c.ClusterName,
		}
	}
}

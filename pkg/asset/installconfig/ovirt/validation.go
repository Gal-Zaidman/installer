package ovirt

import (
	"fmt"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/AlecAivazis/survey.v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/ovirt"
	"github.com/openshift/installer/pkg/types/ovirt/validation"
)

// Validate executes ovirt specific validation
func Validate(ic *types.InstallConfig) error {
	allErrs := field.ErrorList{}
	ovirtPlatformPath := field.NewPath("platform", "ovirt")

	if ic.Platform.Ovirt == nil {
		return errors.New(field.Required(
			ovirtPlatformPath,
			"validation requires a Engine platform configuration").Error())
	}

	allErrs = append(
		allErrs,
		validation.ValidatePlatform(ic.Platform.Ovirt, ovirtPlatformPath)...)

	con, err := NewConnection()
	if err != nil {
		return err
	}
	defer con.Close()

	if err := validateVNICProfile(*ic.Ovirt, con); err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(ovirtPlatformPath.Child("vnicProfileID"), ic.Ovirt.VNICProfileID, err.Error()))
	}
	if ic.ControlPlane != nil && ic.ControlPlane.Platform.Ovirt != nil {
		allErrs = append(
			allErrs,
			validateMachinePool(con, field.NewPath("controlPlane", "platform", "ovirt"), ic.ControlPlane.Platform.Ovirt)...)
	}
	for idx, compute := range ic.Compute {
		fldPath := field.NewPath("compute").Index(idx)
		if compute.Platform.Ovirt != nil {
			allErrs = append(
				allErrs,
				validateMachinePool(con, fldPath.Child("platform", "ovirt"), compute.Platform.Ovirt)...)
		}
	}

	allErrs = append(
		allErrs,
		validateAffinityGroups(ic, ovirtPlatformPath.Child("affinityGroups"), con)...)

	return allErrs.ToAggregate()
}

func validateMachinePool(con *ovirtsdk.Connection, child *field.Path, pool *ovirt.MachinePool) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateInstanceTypeID(con, child, pool)...)
	return allErrs
}

// validateAffinityGroups validates that the affinity group definitions on all machinePools are valid
// - Affinity group contains valid fields
// - Affinity group doesn't already exist in the cluster
// - oVirt cluster has sufficient resources to fulfil the affinity group constraints
func validateAffinityGroups(ic *types.InstallConfig, fldPath *field.Path, con *ovirtsdk.Connection) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateAffinityGroupFields(*ic.Ovirt, fldPath)...)
	allErrs = append(allErrs, validateExistingAffinityGroup(con, *ic.Ovirt, fldPath)...)
	allErrs = append(allErrs, validateAffinityGroupDuplicate(ic.Ovirt.AffinityGroups)...)
	allErrs = append(allErrs, validateClusterResources(con, ic, fldPath)...)

	return allErrs
}

func validateAffinityGroupFields(platform ovirt.Platform, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, ag := range platform.AffinityGroups {
		if ag.Name == "" {
			allErrs = append(
				allErrs,
				field.Invalid(fldPath, ag,
					fmt.Sprintf("Invalid affinity group %v: name must be not empty", ag.Name)))
		}
		if ag.Priority < 0 || ag.Priority > 5 {
			allErrs = append(
				allErrs,
				field.Invalid(fldPath, ag,
					fmt.Sprintf(
						"Invalid affinity group %v: priority value must be between 0-5 found priority %v",
						ag.Name,
						ag.Priority)))
		}
	}
	return allErrs
}

// validateExistingAffinityGroup checks that there is no affinity group with the same name in the cluster
func validateExistingAffinityGroup(con *ovirtsdk.Connection, platform ovirt.Platform, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	res, err := con.SystemService().ClustersService().
		ClusterService(platform.ClusterID).AffinityGroupsService().List().Send()
	if err != nil {
		allErrs = append(
			allErrs,
			field.InternalError(
				fldPath,
				errors.Errorf("failed listing affinity groups for cluster %v", platform.ClusterID)))
	}
	for _, ag := range res.MustGroups().Slice() {
		for _, agNew := range platform.AffinityGroups {
			if ag.MustName() == agNew.Name {
				allErrs = append(
					allErrs,
					field.Invalid(
						fldPath,
						ag,
						fmt.Sprintf("affinity group %v already exist in cluster %v", agNew.Name, platform.ClusterID)))
			}
		}
	}
	return allErrs
}

// validateAffinityGroupDuplicate checks that there is no duplicated affinity group with different fields
func validateAffinityGroupDuplicate(agList []*ovirt.AffinityGroup) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, ag1 := range agList {
		for _, ag2 := range agList[i+1:] {
			if ag1.Name == ag2.Name {
				if ag1.Priority != ag2.Priority ||
					ag1.Description != ag2.Description ||
					ag1.Enforcing != ag2.Enforcing {
					allErrs = append(
						allErrs,
						&field.Error{
							Type: field.ErrorTypeDuplicate,
							BadValue: errors.Errorf("Error validating affinity groups: found same "+
								"affinity group defined twice with different fields %v anf %v", ag1, ag2)})
				}
			}
		}
	}
	return allErrs
}

func validateClusterResources(con *ovirtsdk.Connection, ic *types.InstallConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	mAgReplicas := make(map[string]int)
	for _, agn := range ic.ControlPlane.Platform.Ovirt.AffinityGroupsNames {
		mAgReplicas[agn] = mAgReplicas[agn] + int(*ic.ControlPlane.Replicas)
	}
	for _, compute := range ic.Compute {
		for _, agn := range compute.Platform.Ovirt.AffinityGroupsNames {
			mAgReplicas[agn] = mAgReplicas[agn] + int(*compute.Replicas)
		}
	}

	clusterName, err := GetClusterName(con, ic.Ovirt.ClusterID)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
	}
	hosts, err := FindHostsInCluster(con, clusterName)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
	}
	for _, ag := range ic.Ovirt.AffinityGroups {
		if _, found := mAgReplicas[ag.Name]; found {
			if len(hosts) < mAgReplicas[ag.Name] {
				msg := fmt.Sprintf("Affinity Group %v cannot be fulfilled, oVirt cluster doesn't"+
					"have enough hosts: found %v hosts but %v replicas assigned to affinity group",
					ag.Name, len(hosts), mAgReplicas[ag.Name])
				if ag.Enforcing {
					allErrs = append(allErrs, field.Invalid(fldPath, ag, msg))
				} else {
					logrus.Warning(msg)
				}
			}
		}
	}
	return allErrs
}

func validateInstanceTypeID(con *ovirtsdk.Connection, child *field.Path, machinePool *ovirt.MachinePool) field.ErrorList {
	allErrs := field.ErrorList{}
	if machinePool.InstanceTypeID != "" {
		_, err := con.SystemService().InstanceTypesService().InstanceTypeService(machinePool.InstanceTypeID).Get().Send()
		if err != nil {
			allErrs = append(allErrs, field.NotFound(child.Child("instanceTypeID"), machinePool.InstanceTypeID))
		}
	}
	return allErrs
}

// authenticated takes an ovirt platform and validates
// its connection to the API by establishing
// the connection and authenticating successfully.
// The API connection is closed in the end and must leak
// or be reused in any way.
func authenticated(c *Config) survey.Validator {
	return func(val interface{}) error {
		connection, err := ovirtsdk.NewConnectionBuilder().
			URL(c.URL).
			Username(c.Username).
			Password(fmt.Sprint(val)).
			CAFile(c.CAFile).
			CACert([]byte(c.CABundle)).
			Insecure(c.Insecure).
			Build()

		if err != nil {
			return errors.Errorf("failed to construct connection to Engine platform %s", err)
		}

		defer connection.Close()

		err = connection.Test()
		if err != nil {
			return errors.Errorf("failed to connect to Engine platform %s", err)
		}
		return nil
	}

}

// validate the provided vnic profile exists and belongs the the cluster network
func validateVNICProfile(platform ovirt.Platform, con *ovirtsdk.Connection) error {
	if platform.VNICProfileID != "" {
		profiles, err := FetchVNICProfileByClusterNetwork(con, platform.ClusterID, platform.NetworkName)
		if err != nil {
			return err
		}

		for _, p := range profiles {
			if platform.VNICProfileID == p.MustId() {
				return nil
			}
		}

		return fmt.Errorf(
			"vNic profile ID %s does not belong to cluster network %s",
			platform.VNICProfileID,
			platform.NetworkName)
	}
	return nil
}

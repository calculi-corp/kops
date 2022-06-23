/*
Copyright 2020 The Kubernetes Authors.

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

package azuretasks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

type ApplicationSecurityGroupID struct {
	SubscriptionID               string
	ResourceGroupName            string
	ApplicationSecurityGroupName string
}

func (r *ApplicationSecurityGroupID) String() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/applicationSecurityGroups/%s",
		r.SubscriptionID,
		r.ResourceGroupName,
		r.ApplicationSecurityGroupName)
}

// +kops:fitask
type SecurityGroupRule struct {
	ID            *string
	Name          *string
	Lifecycle     fi.Lifecycle
	ResourceGroup *ResourceGroup

	CIDR       *string
	IPv6CIDR   *string
	PrefixList *string
	Protocol   *string
	Priority   *int32
	AccessType *string // Allow or Deny

	// FromPort is the lower-bound (inclusive) of the port-range
	FromPort *string
	// ToPort is the upper-bound (inclusive) of the port-range
	ToPort                               *string
	SourceApplicationSecurityGroups      *[]ApplicationSecurityGroup // source of the network traffic - applications attached to this ASG
	DestinationApplicationSecurityGroups *[]ApplicationSecurityGroup // destination of the network traffic - applications attached to this ASG
	NetworkSecurityGroup                 *NetworkSecurityGroup       // The NSG where this Rule will be attached to

	Egress *bool
}

func (e *SecurityGroupRule) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

// Find discovers the Network security group in the cloud provider
func (sr *SecurityGroupRule) Find(c *fi.Context) (*SecurityGroupRule, error) {
	cloud := c.Cloud.(azure.AzureCloud)
	l, err := cloud.SecurityRules().List(context.TODO(), *sr.ResourceGroup.Name, *sr.NetworkSecurityGroup.Name)
	if err != nil {
		return nil, err
	}
	var found *network.SecurityRule
	for _, v := range l {
		if *v.Name == *sr.Name {
			found = &v
			klog.V(2).Infof("found matching security rule %q", *found.ID)
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	return &SecurityGroupRule{
		Name:      sr.Name,
		Lifecycle: sr.Lifecycle,
		ResourceGroup: &ResourceGroup{
			Name: sr.ResourceGroup.Name,
		},
		NetworkSecurityGroup: &NetworkSecurityGroup{
			Name: sr.NetworkSecurityGroup.Name,
		},
	}, nil
}

// CheckChanges returns an error if a change is not allowed.
func (*SecurityGroupRule) CheckChanges(a, e, changes *SecurityGroupRule) error {
	if a == nil {
		// Check if required fields are set when a new resource is created.
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		return nil
	}

	// Check if unchanegable fields won't be changed.
	if changes.Name != nil {
		return fi.CannotChangeField("Name")
	}
	return nil
}

// GetDependencies returns a slice of tasks on which the tasks depends on.
func (p *SecurityGroupRule) GetDependencies(tasks map[string]fi.Task) []fi.Task {
	return nil
}

// RenderAzure creates or updates an Network security group.
func (*SecurityGroupRule) RenderAzure(t *azure.AzureAPITarget, a, e, changes *SecurityGroupRule) error {
	if a == nil {
		klog.Infof("Creating a new security rule with name: %s", fi.StringValue(e.Name))
	} else {
		klog.Infof("Updating security rule with name: %s", fi.StringValue(e.Name))
	}

	var sourceAsg = []network.ApplicationSecurityGroup{}
	var destinationAsg = []network.ApplicationSecurityGroup{}

	for _, group := range *e.SourceApplicationSecurityGroups {

		var applicationSecurityGroupID = ApplicationSecurityGroupID{
			SubscriptionID:               t.Cloud.SubscriptionID(),
			ResourceGroupName:            *e.ResourceGroup.Name,
			ApplicationSecurityGroupName: *group.Name,
		}

		asg := network.ApplicationSecurityGroup{ID: to.StringPtr(applicationSecurityGroupID.String())}

		sourceAsg = append(sourceAsg, asg)
	}

	for _, group := range *e.DestinationApplicationSecurityGroups {

		var applicationSecurityGroupID = ApplicationSecurityGroupID{
			SubscriptionID:               t.Cloud.SubscriptionID(),
			ResourceGroupName:            *e.ResourceGroup.Name,
			ApplicationSecurityGroupName: *group.Name,
		}

		asg := network.ApplicationSecurityGroup{ID: to.StringPtr(applicationSecurityGroupID.String())}

		destinationAsg = append(destinationAsg, asg)
	}

	direction := "Inbound"

	if *e.Egress {
		direction = "Outbound"
	}

	sr := network.SecurityRule{
		Name: e.Name,
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                             network.SecurityRuleProtocol(*e.Protocol),
			Priority:                             e.Priority,
			Access:                               network.SecurityRuleAccess(*e.AccessType),
			Direction:                            network.SecurityRuleDirection(direction),
			SourcePortRange:                      e.FromPort,
			DestinationPortRange:                 e.ToPort,
			SourceApplicationSecurityGroups:      &sourceAsg,
			DestinationApplicationSecurityGroups: &destinationAsg,
		},
	}

	return t.Cloud.SecurityRules().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.NetworkSecurityGroup.Name,
		*e.Name,
		sr)
}

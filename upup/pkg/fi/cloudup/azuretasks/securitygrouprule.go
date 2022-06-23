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

// +kops:fitask
type SecurityGroupRule struct {
	ID                   *string
	Name                 *string
	Lifecycle            fi.Lifecycle
	ResourceGroup        *ResourceGroup
	NetworkSecurityGroup *NetworkSecurityGroup

	CIDR       *string
	IPv6CIDR   *string
	PrefixList *string
	Protocol   *string
	Priority   *int32

	// FromPort is the lower-bound (inclusive) of the port-range
	FromPort *int64
	// ToPort is the upper-bound (inclusive) of the port-range
	ToPort                         *int64
	SourceApplicationSecurityGroup *ApplicationSecurityGroup

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

// RenderAzure creates or updates an Network security group.
func (*SecurityGroupRule) RenderAzure(t *azure.AzureAPITarget, a, e, changes *SecurityGroupRule) error {
	if a == nil {
		klog.Infof("Creating a new security rule with name: %s", fi.StringValue(e.Name))
	} else {
		klog.Infof("Updating security rule with name: %s", fi.StringValue(e.Name))
	}

	fromPort := fmt.Sprintf("%v", *e.FromPort)
	toPort := fmt.Sprintf("%v", *e.ToPort)
	any := "Any"

	sr := network.SecurityRule{
		Name: to.StringPtr(*e.Name),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.SecurityRuleProtocol(*e.Protocol),
			Priority:                 e.Priority,
			Access:                   network.SecurityRuleAccess("Allow"),
			Direction:                network.SecurityRuleDirection("Inbound"),
			SourcePortRange:          &fromPort,
			DestinationPortRange:     &toPort,
			SourceAddressPrefix:      &any,
			DestinationAddressPrefix: &any,
		},
	}

	return t.Cloud.SecurityRules().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.NetworkSecurityGroup.Name,
		*e.Name,
		sr)
}

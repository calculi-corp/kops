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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

type NetworkSecurityGroup struct {
	Name          *string
	Lifecycle     fi.Lifecycle
	ResourceGroup *ResourceGroup

	ID          *string
	Description *string

	// Shared is set if this is a shared security group (one we don't create or own)
	Shared *bool

	Tags map[string]*string
}

// CompareWithID returns the Name of the Network security group
func (nsg *NetworkSecurityGroup) CompareWithID() *string {
	return nsg.Name
}

// Find discovers the Network security group in the cloud provider
func (nsg *NetworkSecurityGroup) Find(c *fi.Context) (*NetworkSecurityGroup, error) {
	cloud := c.Cloud.(azure.AzureCloud)
	l, err := cloud.NetworkSecurityGroup().List(context.TODO(), *nsg.ResourceGroup.Name)
	if err != nil {
		return nil, err
	}
	var found *network.SecurityGroup
	for _, v := range l {
		if *v.Name == *nsg.Name {
			found = &v
			klog.V(2).Infof("found matching Network security group %q", *found.ID)
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	return &NetworkSecurityGroup{
		Name:      nsg.Name,
		Lifecycle: nsg.Lifecycle,
		ResourceGroup: &ResourceGroup{
			Name: nsg.ResourceGroup.Name,
		},
		Tags: found.Tags,
	}, nil
}

func (nsg *NetworkSecurityGroup) Run(c *fi.Context) error {
	c.Cloud.(azure.AzureCloud).AddClusterTags(nsg.Tags)
	return fi.DefaultDeltaRunMethod(nsg, c)
}

// CheckChanges returns an error if a change is not allowed.
func (*NetworkSecurityGroup) CheckChanges(a, e, changes *NetworkSecurityGroup) error {
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
func (*NetworkSecurityGroup) RenderAzure(t *azure.AzureAPITarget, a, e, changes *NetworkSecurityGroup) error {
	if a == nil {
		klog.Infof("Creating a new Network security group with name: %s", fi.StringValue(e.Name))
	} else {
		klog.Infof("Updating an Network security group with name: %s", fi.StringValue(e.Name))
	}

	nsg := network.SecurityGroup{
		Name:     to.StringPtr(*e.Name),
		Location: to.StringPtr(t.Cloud.Region()),
		Tags:     e.Tags,
	}

	return t.Cloud.NetworkSecurityGroup().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.Name,
		nsg)
}

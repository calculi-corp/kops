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
	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

type ApplicationSecurityGroup struct {
	Name          *string
	Lifecycle     fi.Lifecycle
	ResourceGroup *ResourceGroup

	ID          *string
	Description *string

	// Shared is set if this is a shared security group (one we don't create or own)
	Shared *bool

	Tags map[string]*string
}

// CompareWithID returns the Name of the Application security group
func (asg *ApplicationSecurityGroup) CompareWithID() *string {
	return asg.Name
}

// Find discovers the Application security group in the cloud provider
func (asg *ApplicationSecurityGroup) Find(c *fi.Context) (*ApplicationSecurityGroup, error) {
	cloud := c.Cloud.(azure.AzureCloud)
	l, err := cloud.ApplicationSecurityGroup().List(context.TODO(), *asg.ResourceGroup.Name)
	if err != nil {
		return nil, err
	}
	var found *network.ApplicationSecurityGroup
	for _, v := range l {
		if *v.Name == *asg.Name {
			found = &v
			klog.V(2).Infof("found matching Application security group %q", *found.ID)
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	return &ApplicationSecurityGroup{
		Name:      asg.Name,
		Lifecycle: asg.Lifecycle,
		ResourceGroup: &ResourceGroup{
			Name: asg.ResourceGroup.Name,
		},
		Tags: found.Tags,
	}, nil
}

// Run implements fi.Task.Run.
func (asg *ApplicationSecurityGroup) Run(c *fi.Context) error {
	c.Cloud.(azure.AzureCloud).AddClusterTags(asg.Tags)
	return fi.DefaultDeltaRunMethod(asg, c)
}

// CheckChanges returns an error if a change is not allowed.
func (*ApplicationSecurityGroup) CheckChanges(a, e, changes *ApplicationSecurityGroup) error {
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

// RenderAzure creates or updates an Application security group.
func (*ApplicationSecurityGroup) RenderAzure(t *azure.AzureAPITarget, a, e, changes *ApplicationSecurityGroup) error {
	if a == nil {
		klog.Infof("Creating a new Application security group with name: %s", fi.StringValue(e.Name))
	} else {
		klog.Infof("Updating an Application security group with name: %s", fi.StringValue(e.Name))
	}

	asg := network.ApplicationSecurityGroup{
		Name:     e.Name,
		Location: to.StringPtr(t.Cloud.Region()),
		Tags:     e.Tags,
	}

	return t.Cloud.ApplicationSecurityGroup().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.Name,
		asg)
}

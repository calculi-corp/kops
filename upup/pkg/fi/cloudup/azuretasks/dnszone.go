/*
Copyright 2019 The Kubernetes Authors.

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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/go-autorest/autorest/to"

	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

// DNSZone is a zone object in a dns provider
// +kops:fitask
type DNSZone struct {
	Name          *string
	Lifecycle     fi.Lifecycle
	ResourceGroup *ResourceGroup

	VirtualNetworkName string
	DNSName            *string
	ZoneID             *string

	// Shared is set if this is a shared security group (one we don't create or own)
	Shared *bool

	Tags map[string]*string

	Private *bool
}

type virtualNetworkID struct {
	SubscriptionID     string
	ResourceGroupName  string
	VirtualNetworkName string
}

// String returns the loadbalancer ID in the path format.
func (v *virtualNetworkID) String() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s",
		v.SubscriptionID,
		v.ResourceGroupName,
		v.VirtualNetworkName,
	)
}

var _ fi.CompareWithID = &DNSZone{}

func (dz *DNSZone) CompareWithID() *string {
	return dz.Name
}

func (dz *DNSZone) Find(c *fi.Context) (*DNSZone, error) {
	cloud := c.Cloud.(azure.AzureCloud)
	l, err := cloud.DNSZone().List(context.TODO(), *dz.ResourceGroup.Name)
	if err != nil {
		return nil, err
	}
	var found *armdns.Zone
	for _, v := range l {
		if *v.Name == *dz.Name {
			found = &v
			klog.V(2).Infof("found matching DNS zone %q", *found.ID)
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	return &DNSZone{
		Name:      dz.Name,
		Lifecycle: dz.Lifecycle,
		ResourceGroup: &ResourceGroup{
			Name: dz.ResourceGroup.Name,
		},
		Tags: found.Tags,
	}, nil
}

func (dz *DNSZone) Run(c *fi.Context) error {
	c.Cloud.(azure.AzureCloud).AddClusterTags(dz.Tags)
	return fi.DefaultDeltaRunMethod(dz, c)
}

// CheckChanges returns an error if a change is not allowed.
func (*DNSZone) CheckChanges(a, e, changes *DNSZone) error {
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

func (*DNSZone) RenderAzure(t *azure.AzureAPITarget, a, e, changes *DNSZone) error {
	if a == nil {
		klog.Infof("Creating a new DNS zone with name: %s", fi.StringValue(e.Name))
	} else {
		klog.Infof("Updating a DNS zone with name: %s", fi.StringValue(e.Name))
	}

	zone := armdns.Zone{
		Name:       to.StringPtr(*e.Name),
		Location:   to.StringPtr(t.Cloud.Region()),
		Tags:       e.Tags,
		Properties: &armdns.ZoneProperties{},
	}

	if *e.Private {
		*zone.Properties.ZoneType = armdns.ZoneTypePrivate

		var virtualNetworkID = virtualNetworkID{
			SubscriptionID:     t.Cloud.SubscriptionID(),
			ResourceGroupName:  *e.ResourceGroup.Name,
			VirtualNetworkName: *&e.VirtualNetworkName,
		}
		zone.Properties.RegistrationVirtualNetworks = []*armdns.SubResource{
			{
				ID: to.StringPtr(virtualNetworkID.String()),
			},
		}
	} else {
		*zone.Properties.ZoneType = armdns.ZoneTypePublic
	}

	return t.Cloud.DNSZone().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.Name,
		zone,
		nil,
	)
}

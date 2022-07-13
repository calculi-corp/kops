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
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/go-autorest/autorest/to"

	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

// RecordSet is a DNS record set
// +kops:fitask
type RecordSet struct {
	Name          *string
	Lifecycle     fi.Lifecycle
	ResourceGroup *ResourceGroup
	Fqdn          *string

	VirtualNetworkName *string
	DNSZone            *string
	RelativeRecordSetName *string
	LoadBalancerName *string

	// Shared is set if this is a shared security group (one we don't create or own)
	Shared *bool

	Private *bool
}

var _ fi.CompareWithID = &RecordSet{}

func (rs *RecordSet) CompareWithID() *string {
	return rs.Name
}

func (rs *RecordSet) Find(c *fi.Context) (*RecordSet, error) {
	cloud := c.Cloud.(azure.AzureCloud)
	l, err := cloud.RecordSet().List(context.TODO(), *rs.ResourceGroup.Name, *rs.DNSZone)
	if err != nil {
		return nil, err
	}
	var found *armdns.RecordSet
	for _, v := range l.Value {
		if *v.Name == *rs.Name {
			found = v
			klog.V(2).Infof("found matching record set %q", *found.ID)
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	return &RecordSet{
		Name:      rs.Name,
		Lifecycle: rs.Lifecycle,
		ResourceGroup: &ResourceGroup{
			Name: rs.ResourceGroup.Name,
		},
		
	}, nil
}

func (rs *RecordSet) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(rs, c)
}

// CheckChanges returns an error if a change is not allowed.
func (*RecordSet) CheckChanges(a, e, changes *RecordSet) error {
	if a == nil {
		// Check if required fields are set when a new resource is created.
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		return nil
	}
	return nil
}

func (*RecordSet) RenderAzure(t *azure.AzureAPITarget, a, e, changes *RecordSet) error {
	if a == nil {
		klog.Infof("Creating a new Record set with name: %s", fi.StringValue(e.Name))
	} else {
		klog.Infof("Updating a Record set with name: %s", fi.StringValue(e.Name))
	}

	var lbID = &loadBalancerID{
		SubscriptionID: t.Cloud.SubscriptionID(),
		ResourceGroupName: *e.ResourceGroup.Name,
		LoadBalancerName: *e.LoadBalancerName,
	}

	recordSetProperties := armdns.RecordSetProperties{
		Fqdn: e.Fqdn,
		TargetResource: &armdns.SubResource{
			ID: &lbID.LoadBalancerName,
		},
	}

	recordSet := armdns.RecordSet{
		Name: to.StringPtr(*e.Name),
		Type: to.StringPtr(string(armdns.RecordTypeA)),
		Properties: &recordSetProperties,
	}

	relativeRecordSetName := strings.ReplaceAll(*e.Fqdn, fmt.Sprintf(".%s",*e.DNSZone), "")

	return t.Cloud.RecordSet().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.DNSZone,
		relativeRecordSetName,
		armdns.RecordTypeA,
		recordSet,
		nil,
	)
}

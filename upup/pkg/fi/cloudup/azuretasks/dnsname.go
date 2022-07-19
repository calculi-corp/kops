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
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest/to"

	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
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

	VirtualNetworkName    *string
	DNSZone               *string
	RelativeRecordSetName *string
	LoadBalancerName      *string // Used as SubResource when provisioning Public record set and used to fetch the ARecords to map to a Private record set
	// Azure private record sets do not have a SubResource field to map to.

	TTL     *int64
	Private *bool
	Shared  *bool
}

var _ fi.CompareWithID = &RecordSet{}

func (rs *RecordSet) CompareWithID() *string {
	return rs.Name
}

func (rs *RecordSet) Find(c *fi.Context) (*RecordSet, error) {
	cloud := c.Cloud.(azure.AzureCloud)
	if !isPrivateDNS(c) { // public
		l, err := cloud.PublicRecordSet().List(context.TODO(), *rs.ResourceGroup.Name, *rs.DNSZone)
		if err != nil {
			return nil, err
		}

		var found *armdns.RecordSet
		for _, v := range l.Value {
			if *v.Name == *rs.Name {
				found = v
				klog.V(2).Infof("found matching public record set %q", *found.ID)
				break
			}
		}
		if found == nil {
			return nil, nil
		}
	} else { // private
		l, err := cloud.PrivateRecordSet().List(context.TODO(), *rs.ResourceGroup.Name, *rs.DNSZone)
		if err != nil {
			return nil, err
		}

		var found *privatedns.RecordSet
		for _, v := range *l.Value {
			if *v.Name == *rs.Name {
				found = &v
				klog.V(2).Infof("found matching private record set %q", *found.ID)
				break
			}
		}
		if found == nil {
			return nil, nil
		}
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
		SubscriptionID:    t.Cloud.SubscriptionID(),
		ResourceGroupName: *e.ResourceGroup.Name,
		LoadBalancerName:  *e.LoadBalancerName,
	}

	if !*e.Private { // Public record set
		recordSetProperties := armdns.RecordSetProperties{
			Fqdn: e.Fqdn,
			TargetResource: &armdns.SubResource{
				ID: to.StringPtr(lbID.String()),
			},
			TTL: e.TTL,
		}

		recordSet := armdns.RecordSet{
			Name:       e.Name,
			Type:       to.StringPtr(string(armdns.RecordTypeA)),
			Properties: &recordSetProperties,
		}

		return t.Cloud.PublicRecordSet().CreateOrUpdate(
			context.TODO(),
			*e.ResourceGroup.Name,
			*e.DNSZone,
			*e.RelativeRecordSetName,
			armdns.RecordTypeA,
			recordSet,
			nil,
		)
	}

	// Private record set
	var aRecord = &privatedns.ARecord{}
	lbAddress, err := getLoadBalancerFrontEndAddress(t, *e.ResourceGroup.Name, *e.LoadBalancerName)
	if err != nil {
		return fmt.Errorf("unable to find load balancer front end address configuration - %v", err)
	}

	aRecord.Ipv4Address = lbAddress

	recordSet := privatedns.RecordSet{
		Name: e.Name,
		Type: to.StringPtr(string(privatedns.A)),
		RecordSetProperties: &privatedns.RecordSetProperties{
			ARecords: &[]privatedns.ARecord{
				*aRecord,
			},
			TTL:              e.TTL,
			IsAutoRegistered: to.BoolPtr(true),
			Fqdn:             e.Fqdn,
		},
	}
	// Private record set
	return t.Cloud.PrivateRecordSet().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.DNSZone,
		*e.RelativeRecordSetName,
		privatedns.A,
		recordSet,
	)
}

func isPrivateDNS(c *fi.Context) bool {
	var private bool

	topology := c.Cluster.Spec.Topology
	if topology != nil && topology.DNS != nil {
		if topology.DNS.Type == kops.DNSTypePrivate {
			private = true
		}
	}
	return private
}

func getLoadBalancerFrontEndAddress(t *azure.AzureAPITarget, resourceGroup, loadBalancerName string) (*string, error) {
	loadBalancers, err := t.Cloud.LoadBalancer().List(context.TODO(), resourceGroup)
	if err != nil {
		return nil, err
	}

	for _, lb := range loadBalancers {
		if *lb.Name == loadBalancerName {
			for _, feIPConfig := range *lb.FrontendIPConfigurations {
				if feIPConfig.PublicIPAddress != nil {
					ipAddr, err := getPublicIPAddressFromName(t, resourceGroup, loadBalancerName)
					if err != nil {
						return nil, err
					}
					klog.Infof("Found load balancer public address: %v", *ipAddr)
					return ipAddr, nil
				}

				if feIPConfig.PrivateIPAddress != nil {
					klog.Infof("Found load balancer private address: %v", *feIPConfig.PrivateIPAddress)
					return feIPConfig.PrivateIPAddress, nil
				}
			}
		}
	}

	return nil, nil
}

func getPublicIPAddressFromName(t *azure.AzureAPITarget, resourceGroup, publicIPAddressName string) (*string, error) {
	publicIPAddresses, err := t.Cloud.PublicIPAddress().List(context.TODO(), resourceGroup)
	if err != nil {
		return nil, err
	}

	for _, addr := range publicIPAddresses {
		if *addr.Name == publicIPAddressName {
			return addr.IPAddress, nil
		}
	}

	return nil, nil
}

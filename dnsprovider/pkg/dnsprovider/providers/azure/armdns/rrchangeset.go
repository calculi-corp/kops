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

package azuredns

import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
)

var _ dnsprovider.ResourceRecordChangeset = &ResourceRecordChangeset{}

type ResourceRecordChangeset struct {
	zone   *Zone
	rrsets *ResourceRecordSets

	additions []dnsprovider.ResourceRecordSet
	removals  []dnsprovider.ResourceRecordSet
	upserts   []dnsprovider.ResourceRecordSet
}

func (c *ResourceRecordChangeset) Add(rrset dnsprovider.ResourceRecordSet) dnsprovider.ResourceRecordChangeset {
	c.additions = append(c.additions, rrset)
	return c
}

func (c *ResourceRecordChangeset) Remove(rrset dnsprovider.ResourceRecordSet) dnsprovider.ResourceRecordChangeset {
	c.removals = append(c.removals, rrset)
	return c
}

func (c *ResourceRecordChangeset) Upsert(rrset dnsprovider.ResourceRecordSet) dnsprovider.ResourceRecordChangeset {
	c.upserts = append(c.upserts, rrset)
	return c
}

func (c *ResourceRecordChangeset) Apply(ctx context.Context) error {
	// Empty changesets should be a relatively quick no-op
	if c.IsEmpty() {
		return nil
	}

	for _, removal := range c.removals {
		relativeRecordSetName := AzureRelativeRecordSetName(removal.Name(), c.zone.Name())
		if c.zone.zoneType == PublicZoneType { // public record set
			_, err := c.rrsets.publicRecordSetClient.Delete(ctx,
				os.Getenv("AZURE_RESOURCEGROUP_NAME"), c.zone.Name(), relativeRecordSetName,
				armdns.RecordType(removal.Type()), &armdns.RecordSetsClientDeleteOptions{})
			if err != nil {
				return err
			}
			continue
		}

		// private record set
		_, err := c.rrsets.privateRecordSetClient.Delete(ctx,
			os.Getenv("AZURE_RESOURCEGROUP_NAME"), c.zone.Name(), privatedns.RecordType(removal.Type()), relativeRecordSetName,
			"")
		if err != nil {
			return err
		}
	}

	for _, addition := range c.additions {
		if err := c.createOrUpdateAzureRecordSet(addition); err != nil {
			return err
		}
	}

	for _, upsert := range c.upserts {
		if err := c.createOrUpdateAzureRecordSet(upsert); err != nil {
			return err
		}
	}

	return nil
}

func (c *ResourceRecordChangeset) IsEmpty() bool {
	return len(c.removals) == 0 && len(c.additions) == 0 && len(c.upserts) == 0
}

// ResourceRecordSets returns the parent ResourceRecordSets
func (c *ResourceRecordChangeset) ResourceRecordSets() dnsprovider.ResourceRecordSets {
	return c.rrsets
}

func (c *ResourceRecordChangeset) createOrUpdateAzureRecordSet(recordSetInput dnsprovider.ResourceRecordSet) error {
	relativeRecordSetName := AzureRelativeRecordSetName(recordSetInput.Name(), c.zone.Name())
	if c.zone.zoneType == PublicZoneType { // public record set
		var arecords = []*armdns.ARecord{}
		for _, rrdata := range recordSetInput.Rrdatas() {
			arecord := &armdns.ARecord{
				IPv4Address: &rrdata,
			}
			arecords = append(arecords, arecord)
		}
		recordSetProperties := armdns.RecordSetProperties{
			Fqdn:     to.StringPtr(recordSetInput.Name()),
			ARecords: arecords,
			TTL: to.Int64Ptr(recordSetInput.Ttl()),
		}

		recordSet := armdns.RecordSet{
			Name:       to.StringPtr(recordSetInput.Name()),
			Type:       to.StringPtr(string(recordSetInput.Type())),
			Properties: &recordSetProperties,
		}

		_, err := c.rrsets.publicRecordSetClient.CreateOrUpdate(context.TODO(),
			os.Getenv("AZURE_RESOURCEGROUP_NAME"), c.zone.Name(), relativeRecordSetName,
			armdns.RecordType(recordSetInput.Type()), recordSet, &armdns.RecordSetsClientCreateOrUpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	}

	var arecords = []privatedns.ARecord{}
	for _, rrdata := range recordSetInput.Rrdatas() {
		arecord := privatedns.ARecord{
			Ipv4Address: &rrdata,
		}
		arecords = append(arecords, arecord)
	}
	recordSetProperties := privatedns.RecordSetProperties{
		Fqdn:     to.StringPtr(recordSetInput.Name()),
		ARecords: &arecords,
		TTL: to.Int64Ptr(recordSetInput.Ttl()),
	}

	recordSet := privatedns.RecordSet{
		Name:                to.StringPtr(recordSetInput.Name()),
		Type:                to.StringPtr(string(recordSetInput.Type())),
		RecordSetProperties: &recordSetProperties,
	}

	_, err := c.rrsets.privateRecordSetClient.CreateOrUpdate(context.TODO(),
		os.Getenv("AZURE_RESOURCEGROUP_NAME"), c.zone.Name(), privatedns.RecordType(recordSetInput.Type()),
		relativeRecordSetName, recordSet, "", "")
	if err != nil {
		return err
	}
	return nil
}

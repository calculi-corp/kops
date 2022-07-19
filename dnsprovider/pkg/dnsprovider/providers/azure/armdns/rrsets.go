package azuredns

import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider/rrstype"
)

var _ dnsprovider.ResourceRecordSets = ResourceRecordSets{}

type ResourceRecordSets struct {
	zone                   *Zone
	publicRecordSetClient  *armdns.RecordSetsClient
	privateRecordSetClient *privatedns.RecordSetsClient
}

func (rrsets ResourceRecordSets) List() ([]dnsprovider.ResourceRecordSet, error) {
	var list []dnsprovider.ResourceRecordSet
	var ctx = context.TODO()

	if rrsets.zone.zoneType == PublicZoneType { // list public record sets
		pager := rrsets.publicRecordSetClient.NewListAllByDNSZonePager(os.Getenv("AZURE_RESOURCEGROUP_NAME"), rrsets.zone.Name(), &armdns.RecordSetsClientListAllByDNSZoneOptions{})
		for pager.More() {
			nextResult, err := pager.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			for _, v := range nextResult.Value {
				r := &ResourceRecordSet{
					publicimpl: *v,
					zoneType:   PublicZoneType,
					recordSets: &rrsets,
				}
				list = append(list, r)
			}
		}
		return list, nil
	}

	// list private record sets
	pager, err := rrsets.privateRecordSetClient.List(ctx, os.Getenv("AZURE_RESOURCEGROUP_NAME"), rrsets.zone.Name(), nil, "*")
	if err != nil {
		return nil, err
	}

	for _, v := range pager.Values() {
		r := &ResourceRecordSet{
			privateimpl: v,
			zoneType:    PrivateZoneType,
			recordSets:  &rrsets,
		}
		list = append(list, r)
	}
	return list, nil
}

func (rrsets ResourceRecordSets) Get(name string) ([]dnsprovider.ResourceRecordSet, error) {
	var list []dnsprovider.ResourceRecordSet
	var ctx = context.TODO()

	if rrsets.zone.zoneType == PublicZoneType { // get public record set
		pager := rrsets.publicRecordSetClient.NewListAllByDNSZonePager(os.Getenv("AZURE_RESOURCEGROUP_NAME"), rrsets.zone.Name(), &armdns.RecordSetsClientListAllByDNSZoneOptions{})
		for pager.More() {
			nextResult, err := pager.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			for _, v := range nextResult.Value {
				if v.Name == &name {
					r := &ResourceRecordSet{
						publicimpl: *v,
						zoneType:   PublicZoneType,
						recordSets: &rrsets,
					}
					list = append(list, r)
				}
			}
		}
		return list, nil
	}

	// get private record set
	pager, err := rrsets.privateRecordSetClient.List(ctx, os.Getenv("AZURE_RESOURCEGROUP_NAME"), rrsets.zone.Name(), nil, "*")
	if err != nil {
		return nil, err
	}

	for _, v := range pager.Values() {
		if v.Name == &name {
			r := &ResourceRecordSet{
				privateimpl: v,
				zoneType:    PrivateZoneType,
				recordSets:  &rrsets,
			}
			list = append(list, r)
		}
	}
	return list, nil
}

func (rrsets ResourceRecordSets) StartChangeset() dnsprovider.ResourceRecordChangeset {
	return &ResourceRecordChangeset{
		zone:   rrsets.zone,
		rrsets: &rrsets,
	}
}

func (rrsets ResourceRecordSets) New(name string, rrdatas []string, ttl int64, rrstype rrstype.RrsType) dnsprovider.ResourceRecordSet {
	rrstypeStr := string(rrstype)

	if rrsets.zone.zoneType == PublicZoneType { // public recotd set
		rrs := &armdns.RecordSet{
			Name: &name,
			Type: &rrstypeStr,
			Properties: &armdns.RecordSetProperties{
				TTL: &ttl,
			},
		}

		for _, rrdata := range rrdatas {
			rrs.Properties.ARecords = append(rrs.Properties.ARecords, &armdns.ARecord{
				IPv4Address: &rrdata,
			})
		}

		return ResourceRecordSet{
			publicimpl: *rrs,
			zoneType:   PublicZoneType,
			recordSets: &rrsets,
		}
	}

	rrs := privatedns.RecordSet{ // private record set
		Name: &name,
		Type: &rrstypeStr,
		RecordSetProperties: &privatedns.RecordSetProperties{
			TTL:      &ttl,
			ARecords: &[]privatedns.ARecord{},
		},
	}
	for _, rrdata := range rrdatas {
		*rrs.RecordSetProperties.ARecords = append(*rrs.RecordSetProperties.ARecords, privatedns.ARecord{
			Ipv4Address: &rrdata,
		})
	}
	return ResourceRecordSet{
		privateimpl: rrs,
		zoneType:    PrivateZoneType,
		recordSets:  &rrsets,
	}
}

// Zone returns the parent zone
func (rrset ResourceRecordSets) Zone() dnsprovider.Zone {
	return rrset.zone
}

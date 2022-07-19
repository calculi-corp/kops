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
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider/rrstype"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
)

var _ dnsprovider.ResourceRecordSet = ResourceRecordSet{}

type ResourceRecordSet struct {
	publicimpl  armdns.RecordSet
	privateimpl privatedns.RecordSet
	zoneType    string // "public" or "private"
	recordSets  *ResourceRecordSets
}

func (r ResourceRecordSet) Name() string {
	if r.zoneType == PublicZoneType {
		return *r.publicimpl.Name
	}
	return *r.privateimpl.Name
}

func (r ResourceRecordSet) Rrdatas() []string {
	if r.zoneType == PublicZoneType {
		result := make([]string, len(r.publicimpl.Properties.ARecords))
		for i, arecord := range r.publicimpl.Properties.ARecords { // we are only creating A records for Azure - so only processing A records
			result[i] = *arecord.IPv4Address
		}
		return result
	}
	result := make([]string, len(*r.privateimpl.ARecords))
	for i, arecord := range *r.privateimpl.ARecords { // we are only creating A records for Azure - so only processing A records
		result[i] = *arecord.Ipv4Address
	}
	return result

}

func (r ResourceRecordSet) Ttl() int64 {
	if r.zoneType == PublicZoneType {
		return *r.publicimpl.Properties.TTL
	}
	return *r.privateimpl.TTL
}

func (r ResourceRecordSet) Type() rrstype.RrsType {
	if r.zoneType == PublicZoneType {
		return rrstype.RrsType(string(armdns.RecordTypeA))
	}
	return rrstype.RrsType(privatedns.A)
}

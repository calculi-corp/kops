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
	"os"

	"k8s.io/klog/v2"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

var _ dnsprovider.Zone = &Zone{}

type Zone struct {
	publicimpl  armdns.Zone
	privateimpl privatedns.PrivateZone
	zoneType    string // "public" or "private"
	zones       *Zones
}

func (z *Zone) Name() string {
	if z.zoneType == PublicZoneType {
		return *z.publicimpl.Name
	}
	return *z.privateimpl.Name
}

func (z *Zone) ID() string {
	if z.zoneType == PublicZoneType {
		return *z.publicimpl.ID
	}
	return *z.privateimpl.ID
}

func (z *Zone) ResourceRecordSets() (dnsprovider.ResourceRecordSets, bool) {
	if z.zoneType == PublicZoneType { // public record set
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			klog.Fatalf("Could not get default Azure credentials - %v", err)
		}
		c, err := armdns.NewRecordSetsClient(os.Getenv("AZURE_SUBSCRIPTION_ID"), cred, &arm.ClientOptions{})
		return &ResourceRecordSets{zone: z,
			publicRecordSetClient: c}, true
	}

	// private record set
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		klog.Fatalf("Could not get default Azure credentials - %v", err)
	}
	c := privatedns.NewRecordSetsClient(os.Getenv("AZURE_SUBSCRIPTION_ID"))
	c.Authorizer = authorizer

	return &ResourceRecordSets{
		zone:                   z,
		privateRecordSetClient: &c,
	}, true
}

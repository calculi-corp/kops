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

	"k8s.io/kops/dnsprovider/pkg/dnsprovider"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
)

// Compile time check for interface adherence
var _ dnsprovider.Zones = Zones{}

type Zones struct {
	interface_ *Interface
}

func (zones Zones) List() ([]dnsprovider.Zone, error) {
	var zoneList []dnsprovider.Zone
	var ctx = context.TODO()

	// List public zones
	pager := zones.interface_.publicZonesClient.NewListByResourceGroupPager(os.Getenv("AZURE_RESOURCEGROUP_NAME"),
		&armdns.ZonesClientListByResourceGroupOptions{Top: nil})
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range nextResult.Value {
			z := &Zone{
				publicimpl: *v,
				zoneType:   PublicZoneType,
				zones:      &zones,
			}
			zoneList = append(zoneList, z)
		}
	}

	// List private zones
	privateZonesPager, err := zones.interface_.privateZonesClient.ListByResourceGroup(ctx, os.Getenv("AZURE_RESOURCEGROUP_NAME"), nil)
	if err != nil {
		return zoneList, err
	}

	for _, v := range privateZonesPager.Values() {
		z := &Zone{
			privateimpl: v,
			zoneType:    PrivateZoneType,
			zones:       &zones,
		}
		zoneList = append(zoneList, z)
	}
	return zoneList, nil
}

func (zones Zones) Add(zone dnsprovider.Zone) (dnsprovider.Zone, error) {
	return zone, nil
}

func (zones Zones) Remove(zone dnsprovider.Zone) error {
	return nil
}

func (zones Zones) New(name string) (dnsprovider.Zone, error) {
	return &Zone{
		zones: &zones,
	}, nil
}

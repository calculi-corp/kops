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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
)

// Compile time check for interface adherence
var _ dnsprovider.Interface = Interface{}

type Interface struct {
	publicZonesClient  armdns.ZonesClient
	privateZonesClient privatedns.PrivateZonesClient
}

func New(publicZonesClient armdns.ZonesClient, privateZonesClient privatedns.PrivateZonesClient) *Interface {
	return &Interface{
		publicZonesClient:  publicZonesClient,
		privateZonesClient: privateZonesClient,
	}
}

func (i Interface) Zones() (zones dnsprovider.Zones, supported bool) {
	return Zones{&i}, true
}

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

package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest"
)

// DNSZoneClient is a client for managing DNS zones
type DNSZoneClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, zoneName string, parameters privatedns.PrivateZone) error
	List(ctx context.Context, resourceGroupName string) ([]privatedns.PrivateZone, error)
	Delete(ctx context.Context, resourceGroupName, zoneName string) error
}

type dnsZoneClientImpl struct {
	c *privatedns.PrivateZonesClient
}

var _ DNSZoneClient = &dnsZoneClientImpl{}

func (c *dnsZoneClientImpl) CreateOrUpdate(ctx context.Context, resourceGroupName string, zoneName string, parameters privatedns.PrivateZone) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, zoneName, parameters, "", "*") // ifMatch set to "", ifNoneMatch set to "*" to prevent overwrite of a zone
	return err
}

func (c *dnsZoneClientImpl) List(ctx context.Context, resourceGroupName string) ([]privatedns.PrivateZone, error) {
	var l []privatedns.PrivateZone

	pager, err := c.c.ListByResourceGroup(ctx, resourceGroupName, nil)
	if err != nil {
		return nil, err
	}
	for _, v := range pager.Values() {
		l = append(l, v)
	}
	return l, nil
}

func (c *dnsZoneClientImpl) Delete(ctx context.Context, resourceGroupName, zoneName string) error {
	poller, err := c.c.Delete(ctx, resourceGroupName, zoneName, "")
	if err != nil {
		return fmt.Errorf("error deleting dns zone: %v", err)
	}
	err = poller.WaitForCompletionRef(ctx, autorest.Client{PollingDuration: 0})
	if err != nil {
		return fmt.Errorf("error waiting for DNS zone deletion completion: %s", err)
	}
	return nil
}

func newDNSZoneClientImpl(subscriptionID string, authorizer autorest.Authorizer) *dnsZoneClientImpl {
	c := privatedns.NewPrivateZonesClient(subscriptionID)
	c.Authorizer = authorizer
	return &dnsZoneClientImpl{
		c: &c,
	}
}

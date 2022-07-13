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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/go-autorest/autorest"
	"k8s.io/klog/v2"
)

// DNSZoneClient is a client for managing DNS zones
type DNSZoneClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, zoneName string, parameters armdns.Zone, options *armdns.ZonesClientCreateOrUpdateOptions) error
	List(ctx context.Context, resourceGroupName string) ([]armdns.Zone, error)
	Delete(ctx context.Context, resourceGroupName, zoneName string) error
}

type dnsZoneClientImpl struct {
	c *armdns.ZonesClient
}

var _ DNSZoneClient = &dnsZoneClientImpl{}

func (c *dnsZoneClientImpl) CreateOrUpdate(ctx context.Context, resourceGroupName string, zoneName string, parameters armdns.Zone, options *armdns.ZonesClientCreateOrUpdateOptions) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, zoneName, parameters, options)
	return err
}

func (c *dnsZoneClientImpl) List(ctx context.Context, resourceGroupName string) ([]armdns.Zone, error) {
	var l []armdns.Zone

	pager := c.c.NewListByResourceGroupPager(resourceGroupName,
		&armdns.ZonesClientListByResourceGroupOptions{Top: nil})
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range nextResult.Value {
			l = append(l, *v)
		}
	}
	return l, nil
}

func (c *dnsZoneClientImpl) Delete(ctx context.Context, resourceGroupName, zoneName string) error {
	poller, err := c.c.BeginDelete(ctx, resourceGroupName, zoneName, &armdns.ZonesClientBeginDeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting dns zone: %v", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("error waiting for DNS zone deletion completion: %s", err)
	}
	return nil
}

func newDNSZoneClientImpl(subscriptionID string, authorizer autorest.Authorizer) *dnsZoneClientImpl {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		klog.Fatalf("Could not get default Azure credentials")
	}
	c, err := armdns.NewZonesClient(subscriptionID, cred, &arm.ClientOptions{})
	return &dnsZoneClientImpl{
		c: c,
	}
}

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

// RecordSetClient is a client for managing DNS entries
type PublicRecordSetClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string,
		recordType armdns.RecordType, parameters armdns.RecordSet, options *armdns.RecordSetsClientCreateOrUpdateOptions) error
	List(ctx context.Context, resourceGroupName, zoneName string) (armdns.RecordSetListResult, error)
	Delete(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string, recordType armdns.RecordType) error
}

type publicRecordSetClient struct {
	c *armdns.RecordSetsClient
}

var _ PublicRecordSetClient = &publicRecordSetClient{}

func (c *publicRecordSetClient) CreateOrUpdate(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string,
	recordType armdns.RecordType, parameters armdns.RecordSet, options *armdns.RecordSetsClientCreateOrUpdateOptions) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, zoneName, relativeRecordSetName,
		recordType, parameters, options)
	return err
}

func (c *publicRecordSetClient) List(ctx context.Context, resourceGroupName, zoneName string) (armdns.RecordSetListResult, error) {
	var l armdns.RecordSetListResult

	var records = []*armdns.RecordSet{}

	pager := c.c.NewListAllByDNSZonePager(resourceGroupName, zoneName,
		&armdns.RecordSetsClientListAllByDNSZoneOptions{})
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return l, err
		}
		for _, v := range nextResult.Value {
			records = append(records, v)
		}
	}
	l.Value = records
	return l, nil
}

func (c *publicRecordSetClient) Delete(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string, recordType armdns.RecordType) error {
	_, err := c.c.Delete(ctx, resourceGroupName, zoneName, relativeRecordSetName,
		recordType, &armdns.RecordSetsClientDeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting dns record set: %v", err)
	}
	return nil
}

func newPublicRecordSetsClient(subscriptionID string, authorizer autorest.Authorizer) *publicRecordSetClient {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		klog.Fatalf("Could not get default Azure credentials - %v", err)
	}
	c, err := armdns.NewRecordSetsClient(subscriptionID, cred, &arm.ClientOptions{})
	if err != nil {
		klog.Fatalf("Could not initialize Azure record sets client - %v", err)
	}
	return &publicRecordSetClient{
		c: c,
	}
}

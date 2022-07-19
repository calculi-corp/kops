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

// RecordSetClient is a client for managing DNS entries
type PrivateRecordSetClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string,
		recordType privatedns.RecordType, parameters privatedns.RecordSet) error
	List(ctx context.Context, resourceGroupName, zoneName string) (privatedns.RecordSetListResult, error)
	Delete(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string, recordType privatedns.RecordType) error
}

type privateRecordSetClient struct {
	c *privatedns.RecordSetsClient
}

var _ PrivateRecordSetClient = &privateRecordSetClient{}

func (c *privateRecordSetClient) CreateOrUpdate(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string,
	recordType privatedns.RecordType, parameters privatedns.RecordSet) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, zoneName, privatedns.RecordType(recordType), relativeRecordSetName, parameters, "", "*")
	return err
}

func (c *privateRecordSetClient) List(ctx context.Context, resourceGroupName, zoneName string) (privatedns.RecordSetListResult, error) {
	var l privatedns.RecordSetListResult

	var records = []privatedns.RecordSet{}

	pager, err := c.c.List(ctx, resourceGroupName, zoneName, nil, "")
	if err != nil {
		return l, err
	}

	for _, v := range pager.Values() {
		records = append(records, v)
	}
	l.Value = &records
	return l, nil
}

func (c *privateRecordSetClient) Delete(ctx context.Context, resourceGroupName, zoneName, relativeRecordSetName string, recordType privatedns.RecordType) error {
	_, err := c.c.Delete(ctx, resourceGroupName, zoneName, privatedns.RecordType(recordType), relativeRecordSetName, "")
	if err != nil {
		return fmt.Errorf("error deleting dns record set: %v", err)
	}
	return nil
}

func newPrivateRecordSetsClient(subscriptionID string, authorizer autorest.Authorizer) *privateRecordSetClient {
	c := privatedns.NewRecordSetsClient(subscriptionID)
	c.Authorizer = authorizer
	return &privateRecordSetClient{
		c: &c,
	}
}

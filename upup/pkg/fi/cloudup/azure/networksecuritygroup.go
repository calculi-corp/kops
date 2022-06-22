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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-06-01/network"
	"github.com/Azure/go-autorest/autorest"
)

// NetworkSecurityGroupClient is a client for managing Network Security Groups
type NetworkSecurityGroupClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, networkSecurityGroupName string, parameters network.SecurityGroup) error
	List(ctx context.Context, resourceGroupName string) ([]network.SecurityGroup, error)
	Delete(ctx context.Context, resourceGroupName, networkSecurityGroupName string) error
}

type networkSecurityGroupClientImpl struct {
	c *network.SecurityGroupsClient
}

var _ NetworkSecurityGroupClient = &networkSecurityGroupClientImpl{}

func (c *networkSecurityGroupClientImpl) CreateOrUpdate(ctx context.Context, resourceGroupName string, networkSecurityGroupName string, parameters network.SecurityGroup) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, networkSecurityGroupName, parameters)
	return err
}

func (c *networkSecurityGroupClientImpl) List(ctx context.Context, resourceGroupName string) ([]network.SecurityGroup, error) {
	var l []network.SecurityGroup
	for iter, err := c.c.ListComplete(ctx, resourceGroupName); iter.NotDone(); err = iter.Next() {
		if err != nil {
			return nil, err
		}
		l = append(l, iter.Value())
	}
	return l, nil
}

func (c *networkSecurityGroupClientImpl) Delete(ctx context.Context, resourceGroupName, networkSecurityGroupName string) error {
	future, err := c.c.Delete(ctx, resourceGroupName, networkSecurityGroupName)
	if err != nil {
		return fmt.Errorf("error deleting network security group: %s", err)
	}
	if err := future.WaitForCompletionRef(ctx, c.c.Client); err != nil {
		return fmt.Errorf("error waiting for network security group deletion completion: %s", err)
	}
	return nil
}

func newNetworkSecurityGroupClientImpl(subscriptionID string, authorizer autorest.Authorizer) *networkSecurityGroupClientImpl {
	c := network.NewSecurityGroupsClient(subscriptionID)
	c.Authorizer = authorizer
	return &networkSecurityGroupClientImpl{
		c: &c,
	}
}

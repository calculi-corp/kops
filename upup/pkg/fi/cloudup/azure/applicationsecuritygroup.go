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

// ApplicationSecurityGroupClient is a client for managing Application Security Groups
type ApplicationSecurityGroupClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, applicationSecurityGroupName string, parameters network.ApplicationSecurityGroup) error
	List(ctx context.Context, resourceGroupName string) ([]network.ApplicationSecurityGroup, error)
	Delete(ctx context.Context, resourceGroupName, applicationSecurityGroupName string) error
}

type applicationSecurityGroupClientImpl struct {
	c *network.ApplicationSecurityGroupsClient
}

var _ ApplicationSecurityGroupClient = &applicationSecurityGroupClientImpl{}

func (c *applicationSecurityGroupClientImpl) CreateOrUpdate(ctx context.Context, resourceGroupName string, applicationSecurityGroupName string, parameters network.ApplicationSecurityGroup) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, applicationSecurityGroupName, parameters)
	return err
}

func (c *applicationSecurityGroupClientImpl) List(ctx context.Context, resourceGroupName string) ([]network.ApplicationSecurityGroup, error) {
	var l []network.ApplicationSecurityGroup
	for iter, err := c.c.ListComplete(ctx, resourceGroupName); iter.NotDone(); err = iter.Next() {
		if err != nil {
			return nil, err
		}
		l = append(l, iter.Value())
	}
	return l, nil
}

func (c *applicationSecurityGroupClientImpl) Delete(ctx context.Context, resourceGroupName, applicationSecurityGroupName string) error {
	future, err := c.c.Delete(ctx, resourceGroupName, applicationSecurityGroupName)
	if err != nil {
		return fmt.Errorf("error deleting application security group: %s", err)
	}
	if err := future.WaitForCompletionRef(ctx, c.c.Client); err != nil {
		return fmt.Errorf("error waiting for application security group deletion completion: %s", err)
	}
	return nil
}

func newApplicationSecurityGroupClientImpl(subscriptionID string, authorizer autorest.Authorizer) *applicationSecurityGroupClientImpl {
	c := network.NewApplicationSecurityGroupsClient(subscriptionID)
	c.Authorizer = authorizer
	return &applicationSecurityGroupClientImpl{
		c: &c,
	}
}

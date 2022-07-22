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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest"
)

// SecurityRuleClient is a client for managing Security Rules
type SecurityRulesClient interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, networkSecurityGroupName string, securityRuleName string, parameters network.SecurityRule) error
	List(ctx context.Context, resourceGroupName string, networkSecurityGroupName string) ([]network.SecurityRule, error)
	Delete(ctx context.Context, resourceGroupName, networkSecurityGroupName string, securityRuleName string) error
}

type securityRulesClientImpl struct {
	c *network.SecurityRulesClient
}

var _ SecurityRulesClient = &securityRulesClientImpl{}

func (c *securityRulesClientImpl) CreateOrUpdate(ctx context.Context, resourceGroupName string, networkSecurityGroupName string, securityRuleName string, parameters network.SecurityRule) error {
	_, err := c.c.CreateOrUpdate(ctx, resourceGroupName, networkSecurityGroupName, securityRuleName, parameters)
	return err
}

func (c *securityRulesClientImpl) List(ctx context.Context, resourceGroupName string, networkSecurityGroupName string) ([]network.SecurityRule, error) {
	var l []network.SecurityRule
	for iter, err := c.c.ListComplete(ctx, resourceGroupName, networkSecurityGroupName); iter.NotDone(); err = iter.Next() {
		if err != nil {
			return nil, err
		}
		l = append(l, iter.Value())
	}
	return l, nil
}

func (c *securityRulesClientImpl) Delete(ctx context.Context, resourceGroupName, networkSecurityGroupName string, securityRuleName string) error {
	future, err := c.c.Delete(ctx, resourceGroupName, networkSecurityGroupName, securityRuleName)
	if err != nil {
		return fmt.Errorf("error deleting security rule: %s", err)
	}
	if err := future.WaitForCompletionRef(ctx, c.c.Client); err != nil {
		return fmt.Errorf("error waiting for security rule deletion completion: %s", err)
	}
	return nil
}

func newSecurityRulesClientImpl(subscriptionID string, authorizer autorest.Authorizer) *securityRulesClientImpl {
	c := network.NewSecurityRulesClient(subscriptionID)
	c.Authorizer = authorizer
	return &securityRulesClientImpl{
		c: &c,
	}
}

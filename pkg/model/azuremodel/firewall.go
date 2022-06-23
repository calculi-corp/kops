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

package azuremodel

import (
	"fmt"

	"k8s.io/klog"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azuretasks"
)

// FirewallModelBuilder configures firewall network objects
type FirewallModelBuilder struct {
	*AzureModelContext
	Lifecycle fi.Lifecycle
}

type NetworkSecurityGroupInfo struct {
	Name   string
	Suffix string
	Task   *azuretasks.NetworkSecurityGroup
}

type ApplicationSecurityGroupInfo struct {
	Name   string
	Suffix string
	Task   *azuretasks.ApplicationSecurityGroup
}

var _ fi.ModelBuilder = &FirewallModelBuilder{}

func (b *FirewallModelBuilder) Build(c *fi.ModelBuilderContext) error {
	nsgNodeGroups, asgNodeGroups, err := b.buildNodeRules(c)
	if err != nil {
		return err
	}

	nsgMasterGroups, asgMasterGroups, err := b.buildMasterRules(c, asgNodeGroups)
	if err != nil {
		return err
	}

	for _, group := range asgMasterGroups {
		c.AddTask(group.Task)
	}

	for _, group := range asgNodeGroups {
		c.AddTask(group.Task)
	}

	for _, group := range nsgMasterGroups {
		c.AddTask(group.Task)
	}

	for _, group := range nsgNodeGroups {
		c.AddTask(group.Task)
	}
	return nil
}

func (b *FirewallModelBuilder) buildNodeRules(c *fi.ModelBuilderContext) ([]NetworkSecurityGroupInfo,
	[]ApplicationSecurityGroupInfo, error) {
	nsgNodeGroups, asgNodeGroups, err := b.GetSecurityGroups(kops.InstanceGroupRoleNode)
	if err != nil {
		return nil, nil, err
	}

	for _, group := range nsgNodeGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		c.AddTask(group.Task)
	}

	for _, group := range asgNodeGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		c.AddTask(group.Task)
	}

	return nsgNodeGroups, asgNodeGroups, nil
}

func (b *FirewallModelBuilder) buildMasterRules(c *fi.ModelBuilderContext, asgNodeGroups []ApplicationSecurityGroupInfo) ([]NetworkSecurityGroupInfo,
	[]ApplicationSecurityGroupInfo, error) {
	nsgMasterGroups, asgMasterGroups, err := b.GetSecurityGroups(kops.InstanceGroupRoleMaster)
	if err != nil {
		return nil, nil, err
	}

	for _, group := range nsgMasterGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		c.AddTask(group.Task)
	}

	for _, group := range asgMasterGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		c.AddTask(group.Task)
	}

	// Masters can talk to masters
	for _, src := range asgMasterGroups {
		t := &azuretasks.SecurityGroupRule{
			Name:                           fi.String("master-to-master-" + *src.Task.Name),
			Lifecycle:                      b.Lifecycle,
			ResourceGroup:                  b.LinkToResourceGroup(),
			SourceApplicationSecurityGroup: src.Task,
			Tags:                           b.Cluster.Spec.CloudLabels,
		}
		AddDirectionalGroupRule(c, t)
	}

	// Masters can talk to nodes
	for _, src := range asgNodeGroups {
		t := &azuretasks.SecurityGroupRule{
			Name:                           fi.String("master-to-node" + *src.Task.Name),
			Lifecycle:                      b.Lifecycle,
			ResourceGroup:                  b.LinkToResourceGroup(),
			SourceApplicationSecurityGroup: src.Task,
			Tags:                           b.Cluster.Spec.CloudLabels,
		}
		AddDirectionalGroupRule(c, t)
	}

	return nsgMasterGroups, asgMasterGroups, nil
}

func (b *AzureModelContext) GetSecurityGroups(role kops.InstanceGroupRole) ([]NetworkSecurityGroupInfo,
	[]ApplicationSecurityGroupInfo, error) {
	var nsgBaseGroup *azuretasks.NetworkSecurityGroup
	var asgBaseGroup *azuretasks.ApplicationSecurityGroup

	if role == kops.InstanceGroupRoleMaster {
		name := b.SecurityGroupName(role)
		nsgBaseGroup = &azuretasks.NetworkSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Network Security group for masters"),
			RemoveExtraRules: []string{
				"port=22",   // SSH
				"port=443",  // k8s api
				"port=2380", // etcd main peer
				"port=2381", // etcd events peer
				"port=4001", // etcd main
				"port=4002", // etcd events
				"port=4789", // VXLAN
				"port=179",  // Calico
				"port=8443", // k8s api secondary listener

				// TODO: UDP vs TCP
				// TODO: Protocol 4 for calico
			},
			Tags: map[string]*string{},
		}

		asgBaseGroup = &azuretasks.ApplicationSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Application Security group for masters"),
			Tags:         map[string]*string{},
		}
	} else if role == kops.InstanceGroupRoleNode {
		name := b.SecurityGroupName(role)
		nsgBaseGroup = &azuretasks.NetworkSecurityGroup{
			Name:             fi.String(name),
			Description:      fi.String("Network Security group for nodes"),
			RemoveExtraRules: []string{"port=22"},
			Tags:             map[string]*string{},
		}

		asgBaseGroup = &azuretasks.ApplicationSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Application Security group for nodes"),
			Tags:        map[string]*string{},
		}
	} else if role == kops.InstanceGroupRoleBastion {
		name := b.SecurityGroupName(role)
		nsgBaseGroup = &azuretasks.NetworkSecurityGroup{
			Name:             fi.String(name),
			Description:      fi.String("Network Security group for bastion"),
			RemoveExtraRules: []string{"port=22"},
		}

		asgBaseGroup = &azuretasks.ApplicationSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Application Security group for bastion"),
			Tags:        map[string]*string{},
		}
	} else {
		return nil, nil, fmt.Errorf("not a supported security group type")
	}

	var nsgGroups []NetworkSecurityGroupInfo
	var asgGroups []ApplicationSecurityGroupInfo

	done := make(map[string]bool)

	// Build groups that specify a SecurityGroupOverride
	allOverrides := true
	for _, ig := range b.InstanceGroups {
		if ig.Spec.Role != role {
			continue
		}

		if ig.Spec.SecurityGroupOverride == nil {
			allOverrides = false
			continue
		}

		name := fi.StringValue(ig.Spec.SecurityGroupOverride)

		// De-duplicate security groups
		if done[name] {
			continue
		}
		done[name] = true

		sgName := fmt.Sprintf("%v-%v", fi.StringValue(ig.Spec.SecurityGroupOverride), role)
		t := &azuretasks.NetworkSecurityGroup{
			Name:        &sgName,
			ID:          ig.Spec.SecurityGroupOverride,
			Shared:      fi.Bool(true),
			Description: nsgBaseGroup.Description,
		}
		// Because the SecurityGroup is shared, we don't set RemoveExtraRules
		// This does mean we don't check them.  We might want to revisit this in future.

		suffix := "-" + name

		nsgGroups = append(nsgGroups, NetworkSecurityGroupInfo{
			Name:   name,
			Suffix: suffix,
			Task:   t,
		})
	}

	// Add the default SecurityGroup, if any InstanceGroups are using the default
	if !allOverrides {
		nsgGroups = append(nsgGroups, NetworkSecurityGroupInfo{
			Name: fi.StringValue(nsgBaseGroup.Name),
			Task: nsgBaseGroup,
		})
	}

	asgGroups = append(asgGroups, ApplicationSecurityGroupInfo{
		Name: fi.StringValue(asgBaseGroup.Name),
		Task: asgBaseGroup,
	})
	return nsgGroups, asgGroups, nil
}

func AddDirectionalGroupRule(c *fi.ModelBuilderContext, t *azuretasks.SecurityGroupRule) {
	klog.V(8).Infof("Adding rule %v", t.Name)
	c.AddTask(t)
}

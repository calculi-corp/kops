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

	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azuretasks"
)

const (
	baseSGRulePriorityMasterToMaster          = 100
	baseSGRulePriorityMasterToNode            = 200
	baseSGRulePriorityNodeToNode              = 300
	sGRulePriorityMasterSSHAccess             = 400
	sGRulePriorityMasterApiServerPublicAccess = 500
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
	asgNodeGroups, err := b.buildNodeRules(c)
	if err != nil {
		return err
	}

	err = b.buildMasterRules(c, asgNodeGroups)

	return err
}

func (b *FirewallModelBuilder) buildNodeRules(c *fi.ModelBuilderContext) ([]ApplicationSecurityGroupInfo, error) {
	_, asgNodeGroups, err := b.GetSecurityGroups(kops.InstanceGroupRoleNode)
	if err != nil {
		return nil, err
	}

	for _, group := range asgNodeGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		c.AddTask(group.Task)
	}

	return asgNodeGroups, nil
}

func (b *FirewallModelBuilder) getSecurityRules(c *fi.ModelBuilderContext, asgMasterGroups []ApplicationSecurityGroupInfo, asgNodeGroups []ApplicationSecurityGroupInfo) []*azuretasks.SecurityGroupRule {

	var securityGroupRules = []*azuretasks.SecurityGroupRule{}

	// Masters can talk to masters
	var masterSourceASGs = []azuretasks.ApplicationSecurityGroup{}
	var masterDestinationASGs = []azuretasks.ApplicationSecurityGroup{}
	for _, asg := range asgMasterGroups {
		masterSourceASGs = append(masterSourceASGs, *asg.Task)
		masterDestinationASGs = append(masterDestinationASGs, *asg.Task)
	}

	var priority int32 = baseSGRulePriorityMasterToMaster
	rule := &azuretasks.SecurityGroupRule{
		Name:                                 fi.String("master-to-master"),
		SourceApplicationSecurityGroups:      &masterSourceASGs,
		DestinationApplicationSecurityGroups: &masterDestinationASGs,
		Protocol:                             to.StringPtr("*"),
		AccessType:                           to.StringPtr("Allow"),
		Egress:                               to.BoolPtr(false),
		ToPort:                               to.StringPtr("*"),
		FromPort:                             to.StringPtr("*"),
		Priority:                             to.Int32Ptr(priority),
	}

	securityGroupRules = append(securityGroupRules, rule)

	sshRule := b.sshAccessMaster(c, sGRulePriorityMasterSSHAccess) // add security group rules to allow SSH access to master nodes. One for each master node
	securityGroupRules = append(securityGroupRules, sshRule)

	if b.Cluster.Spec.API.LoadBalancer.Type == kops.LoadBalancerTypePublic {
		apiRule := b.publicApiServerAccess(c, sGRulePriorityMasterApiServerPublicAccess) // add security group rules to allow external access to API server via 443
		securityGroupRules = append(securityGroupRules, apiRule)
	}

	// Masters can talk to nodes
	var sourceASGsMasterNode = []azuretasks.ApplicationSecurityGroup{}
	var destinationASGsMasterNode = []azuretasks.ApplicationSecurityGroup{}
	for _, asg := range asgMasterGroups {
		sourceASGsMasterNode = append(sourceASGsMasterNode, *asg.Task)
	}
	for _, asg := range asgNodeGroups {
		destinationASGsMasterNode = append(destinationASGsMasterNode, *asg.Task)
	}

	priority = baseSGRulePriorityMasterToNode

	rule = &azuretasks.SecurityGroupRule{
		Name:                                 fi.String("master-to-node"),
		SourceApplicationSecurityGroups:      &sourceASGsMasterNode,
		DestinationApplicationSecurityGroups: &destinationASGsMasterNode,
		Protocol:                             to.StringPtr("*"),
		AccessType:                           to.StringPtr("Allow"),
		Egress:                               to.BoolPtr(false),
		ToPort:                               to.StringPtr("*"),
		FromPort:                             to.StringPtr("*"),
		Priority:                             to.Int32Ptr(priority),
	}

	securityGroupRules = append(securityGroupRules, rule)

	// Nodes can talk to nodes
	var nodeSourceASGs = []azuretasks.ApplicationSecurityGroup{}
	var nodeDestinationASGs = []azuretasks.ApplicationSecurityGroup{}
	for _, asg := range asgNodeGroups {
		nodeSourceASGs = append(nodeSourceASGs, *asg.Task)
		nodeDestinationASGs = append(nodeDestinationASGs, *asg.Task)
	}

	priority = baseSGRulePriorityNodeToNode
	rule = &azuretasks.SecurityGroupRule{
		Name:                                 fi.String("node-to-node"),
		SourceApplicationSecurityGroups:      &nodeSourceASGs,
		DestinationApplicationSecurityGroups: &nodeDestinationASGs,
		Protocol:                             to.StringPtr("*"),
		AccessType:                           to.StringPtr("Allow"),
		Egress:                               to.BoolPtr(false),
		ToPort:                               to.StringPtr("*"),
		FromPort:                             to.StringPtr("*"),
		Priority:                             to.Int32Ptr(priority),
	}

	securityGroupRules = append(securityGroupRules, rule)

	return securityGroupRules
}

func (b *FirewallModelBuilder) buildMasterRules(c *fi.ModelBuilderContext, asgNodeGroups []ApplicationSecurityGroupInfo) error {
	nsgMasterGroups, asgMasterGroups, err := b.GetSecurityGroups(kops.InstanceGroupRoleMaster)
	if err != nil {
		return err
	}

	for _, group := range asgMasterGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		c.AddTask(group.Task)
	}

	for _, group := range nsgMasterGroups {
		group.Task.Lifecycle = b.Lifecycle
		group.Task.ResourceGroup = b.LinkToResourceGroup()
		group.Task.Rules = b.getSecurityRules(c, asgMasterGroups, asgNodeGroups)
		c.AddTask(group.Task)
	}

	return nil
}

func (b *FirewallModelBuilder) sshAccessMaster(c *fi.ModelBuilderContext, priority int32) *azuretasks.SecurityGroupRule {
	rule := &azuretasks.SecurityGroupRule{
		Name:            fi.String("ssh-to-master"),
		SourceCIDRs:     to.StringSlicePtr(b.Cluster.Spec.SSHAccess),
		DestinationCIDR: to.StringPtr("VirtualNetwork"),
		Protocol:        to.StringPtr("TCP"),
		AccessType:      to.StringPtr("Allow"),
		Egress:          to.BoolPtr(false),
		ToPort:          to.StringPtr("22"),
		FromPort:        to.StringPtr("*"),
		Priority:        to.Int32Ptr(priority),
	}

	return rule
}

func (b *FirewallModelBuilder) publicApiServerAccess(c *fi.ModelBuilderContext, priority int32) *azuretasks.SecurityGroupRule {

	rule := &azuretasks.SecurityGroupRule{
		Name:            fi.String("public-api-server-access"),
		SourceCIDR:      to.StringPtr("Internet"),
		DestinationCIDR: to.StringPtr("VirtualNetwork"),
		Protocol:        to.StringPtr("TCP"),
		AccessType:      to.StringPtr("Allow"),
		Egress:          to.BoolPtr(false),
		ToPort:          to.StringPtr("443"),
		FromPort:        to.StringPtr("*"),
		Priority:        to.Int32Ptr(priority),
	}

	return rule
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
			Tags:        b.CloudTags(name),
		}

		asgBaseGroup = &azuretasks.ApplicationSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Application Security group for masters"),
			Tags:        b.CloudTags(name),
		}
	} else if role == kops.InstanceGroupRoleNode {
		name := b.SecurityGroupName(role)
		nsgBaseGroup = &azuretasks.NetworkSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Network Security group for nodes"),
			Tags:        b.CloudTags(name),
		}

		asgBaseGroup = &azuretasks.ApplicationSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Application Security group for nodes"),
			Tags:        b.CloudTags(name),
		}
	} else if role == kops.InstanceGroupRoleBastion {
		name := b.SecurityGroupName(role)
		nsgBaseGroup = &azuretasks.NetworkSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Network Security group for bastion"),
		}

		asgBaseGroup = &azuretasks.ApplicationSecurityGroup{
			Name:        fi.String(name),
			Description: fi.String("Application Security group for bastion"),
			Tags:        b.CloudTags(name),
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
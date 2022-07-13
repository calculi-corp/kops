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
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azuretasks"
)

// APILoadBalancerModelBuilder builds a LoadBalancer for accessing the API
type DNSModelBuilder struct {
	*AzureModelContext

	Lifecycle         fi.Lifecycle
	SecurityLifecycle fi.Lifecycle
}

var _ fi.ModelBuilder = &DNSModelBuilder{}

// Build builds tasks for creating a K8s API server for Azure.
func (b *DNSModelBuilder) Build(c *fi.ModelBuilderContext) error {
	if !b.UseLoadBalancerForAPI() { // DNS is only created for Load balancer endpoints
		return nil
	}

	var private bool
	if b.Cluster.Spec.Topology.DNS.Type == kops.DNSTypePrivate {
		private = true
	}

	// Create DNS Zone
	dz := &azuretasks.DNSZone{
		Name:          fi.String(b.NameForLoadBalancer()),
		Lifecycle:     b.Lifecycle,
		ResourceGroup: b.LinkToResourceGroup(),

		VirtualNetworkName: fi.String(b.NameForVirtualNetwork()),
		Shared:             fi.Bool(len(b.Cluster.Spec.DNSZone) > 0),
		Tags:               map[string]*string{},
		Private:            to.BoolPtr(private),
	}

	c.AddTask(dz)
	return nil
}
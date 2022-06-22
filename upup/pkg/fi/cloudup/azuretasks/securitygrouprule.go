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

package azuretasks

import "k8s.io/kops/upup/pkg/fi"

// +kops:fitask
type SecurityGroupRule struct {
	ID        *string
	Name      *string
	Lifecycle fi.Lifecycle
	ResourceGroup *ResourceGroup

	CIDR          *string
	IPv6CIDR      *string
	PrefixList    *string
	Protocol      *string

	// FromPort is the lower-bound (inclusive) of the port-range
	FromPort *int64
	// ToPort is the upper-bound (inclusive) of the port-range
	ToPort      *int64
	SourceApplicationSecurityGroup *ApplicationSecurityGroup

	Egress *bool

	Tags map[string]string
}

func (e *SecurityGroupRule) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}
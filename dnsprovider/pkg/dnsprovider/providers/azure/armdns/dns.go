/*
Copyright 2019 The Kubernetes Authors.

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

package azuredns

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
)

const (
	ProviderName    = "azure-dns"
	PublicZoneType  = "public"
	PrivateZoneType = "private"
)

func init() {
	dnsprovider.RegisterDNSProvider(ProviderName, func(config io.Reader) (dnsprovider.Interface, error) {
		return newZonesClient(config)
	})
}

func newZonesClient(_ io.Reader) (*Interface, error) {
	// Public DNS zone
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	publicClient, err := armdns.NewZonesClient(os.Getenv("AZURE_SUBSCRIPTION_ID"), cred, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}

	// Private DNS zone
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, err
	}

	privateClient := privatedns.NewPrivateZonesClient(os.Getenv("AZURE_SUBSCRIPTION_ID"))
	privateClient.Authorizer = authorizer
	return New(*publicClient, *&privateClient), nil
}

func AzureRelativeRecordSetName(fqdn, zoneName string) string {
	replacer := fmt.Sprintf(".%s.*", zoneName)
	re := regexp.MustCompile(replacer)
	return re.ReplaceAllString(fqdn, "")
}

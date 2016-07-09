package provider

import (
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/Azure/azure-sdk-for-go/arm/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/NetSys/quilt/join"
)

var mutex sync.Mutex

type fakeAzureClient struct {
	securityGroups map[string]securityGroup
}

type securityGroup struct {
	rgName string
	value  network.SecurityGroup
	rules  map[string]securityRule
}

type securityRule struct {
	rgName  string
	nsgName string
	value   network.SecurityRule
}

func newFakeAzureClient() azureAPI {
	return fakeAzureClient{
		securityGroups: make(map[string]securityGroup),
	}
}

func newFakeAzureCluster() azureCluster {
	return azureCluster{
		azureClient: newFakeAzureClient(),
		namespace:   "test-namespace",
	}
}

func (client fakeAzureClient) ifaceCreate(rgName string, ifaceName string,
	param network.Interface, cancel <-chan struct{}) (result autorest.Response,
	err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) ifaceDelete(rgName string, ifaceName string,
	cancel <-chan struct{}) (result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) ifaceGet(rgName string, ifaceName string, expand string) (
	result network.Interface, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return network.Interface{}, nil
}

func (client fakeAzureClient) publicIPCreate(rgName string, pipAddrName string,
	param network.PublicIPAddress, cancel <-chan struct{}) (result autorest.Response,
	err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) publicIPDelete(rgName string, pipAddrName string,
	cancel <-chan struct{}) (result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) publicIPGet(rgName string, pipAddrName string,
	expand string) (result network.PublicIPAddress, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return network.PublicIPAddress{}, nil
}

func (client fakeAzureClient) securityGroupCreate(rgName string, nsgName string,
	param network.SecurityGroup, cancel <-chan struct{}) (result autorest.Response,
	err error) {
	mutex.Lock()
	defer mutex.Unlock()
	client.securityGroups[nsgName] = securityGroup{
		rgName: rgName,
		value:  param,
		rules:  make(map[string]securityRule),
	}
	return autorest.Response{}, nil
}

func (client fakeAzureClient) securityGroupList(rgName string) (
	result network.SecurityGroupListResult, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	groups := []network.SecurityGroup{}
	for _, securityGroup := range client.securityGroups {
		if securityGroup.rgName == rgName {
			groups = append(groups, securityGroup.value)
		}
	}
	result = network.SecurityGroupListResult{
		Value: &groups,
	}
	return result, nil
}

func (client fakeAzureClient) securityRuleCreate(rgName string,
	nsgName string, secRuleName string,
	param network.SecurityRule,
	cancel <-chan struct{}) (result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	securityGroup := client.securityGroups[nsgName]
	securityGroup.rules[*param.Name] = securityRule{
		rgName:  rgName,
		nsgName: nsgName,
		value:   param,
	}
	client.securityGroups[nsgName] = securityGroup
	return autorest.Response{}, nil
}

func (client fakeAzureClient) securityRuleDelete(rgName string, nsgName string,
	secRuleName string, cancel <-chan struct{}) (result autorest.Response,
	err error) {
	mutex.Lock()
	defer mutex.Unlock()
	securityGroup := client.securityGroups[nsgName]
	delete(securityGroup.rules, secRuleName)
	return autorest.Response{}, nil
}

func (client fakeAzureClient) securityRuleList(rgName string, nsgName string) (
	result network.SecurityRuleListResult, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	rules := []network.SecurityRule{}
	securityGroup := client.securityGroups[nsgName]
	for _, securityRule := range securityGroup.rules {
		rules = append(rules, securityRule.value)
	}
	result = network.SecurityRuleListResult{
		Value: &rules,
	}
	return result, nil
}

func (client fakeAzureClient) vnetCreate(rgName string, virtualNetworkName string,
	param network.VirtualNetwork, cancel <-chan struct{}) (result autorest.Response,
	err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) vnetList(rgName string) (
	result network.VirtualNetworkListResult, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return network.VirtualNetworkListResult{}, nil
}

func (client fakeAzureClient) rgCreate(rgName string, param resources.ResourceGroup) (
	result resources.ResourceGroup, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return resources.ResourceGroup{}, nil
}

func (client fakeAzureClient) rgDelete(rgName string, cancel <-chan struct{}) (
	result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) storageListByRg(rgName string) (
	result storage.AccountListResult, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return storage.AccountListResult{}, nil
}

func (client fakeAzureClient) storageCheckName(
	accountName storage.AccountCheckNameAvailabilityParameters) (
	result storage.CheckNameAvailabilityResult, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return storage.CheckNameAvailabilityResult{}, nil
}

func (client fakeAzureClient) storageCreate(rgName string, accountName string,
	param storage.AccountCreateParameters, cancel <-chan struct{}) (
	result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) storageGet(rgName string, accountName string) (
	result storage.Account, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return storage.Account{}, nil
}

func (client fakeAzureClient) vmCreate(rgName string, vmName string,
	param compute.VirtualMachine, cancel <-chan struct{}) (
	result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) vmDelete(rgName string, vmName string,
	cancel <-chan struct{}) (result autorest.Response, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return autorest.Response{}, nil
}

func (client fakeAzureClient) vmList(rgName string) (
	result compute.VirtualMachineListResult, err error) {
	mutex.Lock()
	defer mutex.Unlock()
	return compute.VirtualMachineListResult{}, nil
}

type int32Slice []int32

func (slice int32Slice) Get(i int) interface{} {
	return slice[i]
}

func (slice int32Slice) Len() int {
	return len(slice)
}

func TestSetACLs(t *testing.T) {
	fakeClst := newFakeAzureCluster()
	cancel := make(chan struct{})
	asterisk := "*"

	localACLs := []string{"10.0.0.1"}

	setACLs := func(localACLs []string) {
		if err := fakeClst.SetACLs(localACLs); err != nil {
			t.Error(err)
		}
	}

	checkACLs := func(localACLs []string) {
		securityGroups, err := fakeClst.azureClient.securityGroupList(
			resourceGroupName)
		if err != nil {
			t.Error(err)
		}

		for _, securityGroup := range *securityGroups.Value {
			securityRules, err := fakeClst.azureClient.securityRuleList(
				resourceGroupName, *securityGroup.Name)
			if err != nil {
				t.Error(err)
			}

			// Inbound and Outbound ACLs should end up be the same.
			cloudInACLs := make(map[string]struct{})
			cloudOutACLs := make(map[string]struct{})

			for _, cloudRule := range *securityRules.Value {
				properties := cloudRule.Properties
				if properties.Protocol != network.Asterisk ||
					*properties.SourcePortRange != asterisk ||
					*properties.DestinationPortRange != asterisk ||
					properties.Access != network.Allow {
					t.Error("Unexpected cloud rule properties")
				}

				if properties.Direction == network.Inbound {
					address := *properties.SourceAddressPrefix
					if _, ok := cloudInACLs[address]; ok {
						t.Error("Duplicate rule in cloud")
					}
					cloudInACLs[address] = struct{}{}
				} else if properties.Direction == network.Outbound {
					address := *properties.DestinationAddressPrefix
					if _, ok := cloudOutACLs[address]; ok {
						t.Error("Duplicate rule in cloud")
					}
					cloudOutACLs[address] = struct{}{}
				}
			}

			// Ensure that for each IP, there is one inbound rule and one
			// outbound rule.
			if len(cloudInACLs) != len(cloudOutACLs) {
				t.Error("Inbound and outbound rules don't match")
			}

			for rule := range cloudInACLs {
				if _, ok := cloudOutACLs[rule]; !ok {
					t.Error("Inbound and outbound rules don't match")
				}
			}

			// Ensure that cloud ACLs equals local ACLs.
			if len(cloudInACLs) != len(localACLs) {
				t.Error("Cloud and local rules don't match")
			}

			for _, rule := range localACLs {
				if _, ok := cloudInACLs[rule]; !ok {
					t.Error("Cloud and local rules don't match")
				}
			}
		}
	}

	// One security group, one ACL rule.
	nsg1 := network.SecurityGroup{
		Name:     stringPtr("1"),
		ID:       stringPtr("1"),
		Location: stringPtr("location1"),
		Tags:     &map[string]*string{nsTag: &fakeClst.namespace},
	}
	fakeClst.azureClient.securityGroupCreate(resourceGroupName, *nsg1.Name, nsg1,
		cancel)

	setACLs(localACLs)
	checkACLs(localACLs)

	// One security group, two ACL rules.
	localACLs = []string{"10.0.0.1", "10.0.0.2"}
	setACLs(localACLs)
	checkACLs(localACLs)

	// One security group, one ACL rule again.
	localACLs = []string{"10.0.0.1"}
	setACLs(localACLs)
	checkACLs(localACLs)

	// Two security group, two ACL rules.
	localACLs = []string{"10.0.0.3", "10.0.0.4"}
	nsg2 := network.SecurityGroup{
		Name:     stringPtr("2"),
		ID:       stringPtr("2"),
		Location: stringPtr("location2"),
		Tags:     &map[string]*string{nsTag: &fakeClst.namespace},
	}
	fakeClst.azureClient.securityGroupCreate(resourceGroupName, *nsg2.Name, nsg2,
		cancel)

	setACLs(localACLs)
	checkACLs(localACLs)
}

func TestSyncSecurityRules(t *testing.T) {
	fakeClst := newFakeAzureCluster()
	cancel := make(chan struct{})
	nsg := network.SecurityGroup{
		Name:     stringPtr("test-nsg"),
		ID:       stringPtr("test-nsg"),
		Location: stringPtr("location1"),
		Tags:     &map[string]*string{nsTag: &fakeClst.namespace},
	}
	fakeClst.azureClient.securityGroupCreate(resourceGroupName, *nsg.Name, nsg,
		cancel)

	sync := func(localRules *[]network.SecurityRule,
		cloudRules *[]network.SecurityRule) ([]int32, error) {
		if err := fakeClst.syncSecurityRules(*nsg.Name, *localRules,
			*cloudRules); err != nil {
			return nil, err
		}

		result, err := fakeClst.azureClient.securityRuleList(resourceGroupName,
			*nsg.Name)
		if err != nil {
			return nil, err
		}

		*cloudRules = *result.Value

		cloudPriorities := []int32{}
		for _, rule := range *cloudRules {
			properties := *rule.Properties
			cloudPriorities = append(cloudPriorities, *properties.Priority)
		}

		return cloudPriorities, nil
	}

	checkSync := func(localRules []network.SecurityRule, expectedPriorities []int32,
		cloudRules []network.SecurityRule, cloudPriorities []int32) {
		priorityKey := func(val interface{}) interface{} {
			return val.(int32)
		}

		pair, left, right := join.HashJoin(int32Slice(expectedPriorities),
			int32Slice(cloudPriorities), priorityKey, priorityKey)
		if len(pair) != len(cloudRules) || len(left) != 0 || len(right) != 0 {
			t.Log(pair, left, right)
			t.Error("Error setting security rule priorities.")
		}

		ruleKey := func(val interface{}) interface{} {
			property := val.(network.SecurityRule).Properties
			return struct {
				sourcePortRange      string
				sourceAddressPrefix  string
				destinationPortRange string
				destAddressPrefix    string
				direction            network.SecurityRuleDirection
			}{
				sourcePortRange:      *property.SourcePortRange,
				sourceAddressPrefix:  *property.SourceAddressPrefix,
				destinationPortRange: *property.DestinationPortRange,
				destAddressPrefix:    *property.DestinationAddressPrefix,
				direction:            property.Direction,
			}
		}

		pair, left, right = join.HashJoin(securityRuleSlice(localRules),
			securityRuleSlice(cloudRules), ruleKey, ruleKey)
		if len(pair) != len(cloudRules) || len(left) != 0 || len(right) != 0 {
			t.Error("Error setting security rules.")
		}
	}

	// Initially add two rules.
	rule1 := network.SecurityRule{
		ID:   stringPtr("1"),
		Name: stringPtr("1"),
		Properties: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.Asterisk,
			SourcePortRange:          stringPtr("*"),
			SourceAddressPrefix:      stringPtr("10.0.0.1"),
			DestinationPortRange:     stringPtr("*"),
			DestinationAddressPrefix: stringPtr("*"),
			Access:    network.Allow,
			Direction: network.Inbound,
		},
	}

	rule2 := network.SecurityRule{
		ID:   stringPtr("2"),
		Name: stringPtr("2"),
		Properties: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.Asterisk,
			SourcePortRange:          stringPtr("*"),
			SourceAddressPrefix:      stringPtr("10.0.0.2"),
			DestinationPortRange:     stringPtr("*"),
			DestinationAddressPrefix: stringPtr("*"),
			Access:    network.Allow,
			Direction: network.Inbound,
		},
	}

	localRules := []network.SecurityRule{rule1, rule2}
	cloudRules := []network.SecurityRule{}

	expectedPriorities := []int32{100, 101}
	cloudPriorities, err := sync(&localRules, &cloudRules)
	if err != nil {
		t.Error(err)
	}

	checkSync(localRules, expectedPriorities, cloudRules, cloudPriorities)

	// Add two more rules.
	rule3 := network.SecurityRule{
		ID:   stringPtr("3"),
		Name: stringPtr("3"),
		Properties: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.Asterisk,
			SourcePortRange:          stringPtr("*"),
			SourceAddressPrefix:      stringPtr("*"),
			DestinationPortRange:     stringPtr("*"),
			DestinationAddressPrefix: stringPtr("10.0.0.3"),
			Access:    network.Allow,
			Direction: network.Inbound,
		},
	}

	rule4 := network.SecurityRule{
		ID:   stringPtr("4"),
		Name: stringPtr("4"),
		Properties: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.Asterisk,
			SourcePortRange:          stringPtr("*"),
			SourceAddressPrefix:      stringPtr("*"),
			DestinationPortRange:     stringPtr("*"),
			DestinationAddressPrefix: stringPtr("10.0.0.4"),
			Access:    network.Allow,
			Direction: network.Inbound,
		},
	}

	localRules = append(localRules, rule3, rule4)
	expectedPriorities = []int32{100, 101, 102, 103}
	cloudPriorities, err = sync(&localRules, &cloudRules)
	if err != nil {
		t.Error(err)
	}

	checkSync(localRules, expectedPriorities, cloudRules, cloudPriorities)

	// Add duplicate rules.
	localRules = append(localRules, rule3, rule4)
	expectedPriorities = []int32{100, 101, 102, 103}
	cloudPriorities, err = sync(&localRules, &cloudRules)
	if err != nil {
		t.Error(err)
	}

	checkSync(localRules, expectedPriorities, cloudRules, cloudPriorities)

	// Keep rule1, and add two new rules.
	rule5 := network.SecurityRule{
		ID:   stringPtr("5"),
		Name: stringPtr("5"),
		Properties: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.Asterisk,
			SourcePortRange:          stringPtr("*"),
			SourceAddressPrefix:      stringPtr("1.2.3.4"),
			DestinationPortRange:     stringPtr("*"),
			DestinationAddressPrefix: stringPtr("*"),
			Access:    network.Allow,
			Direction: network.Inbound,
		},
	}

	rule6 := network.SecurityRule{
		ID:   stringPtr("6"),
		Name: stringPtr("6"),
		Properties: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.Asterisk,
			SourcePortRange:          stringPtr("*"),
			SourceAddressPrefix:      stringPtr("5.6.7.8"),
			DestinationPortRange:     stringPtr("*"),
			DestinationAddressPrefix: stringPtr("*"),
			Access:    network.Allow,
			Direction: network.Inbound,
		},
	}

	localRules = []network.SecurityRule{rule1, rule5, rule6}
	expectedPriorities = []int32{100, 101, 102}
	cloudPriorities, err = sync(&localRules, &cloudRules)
	if err != nil {
		t.Error(err)
	}

	checkSync(localRules, expectedPriorities, cloudRules, cloudPriorities)
}

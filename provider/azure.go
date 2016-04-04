package provider

// First, download Publish Settings using the following link:
// https://manage.windowsazure.com/PublishSettings/
// Save the file as ~/.azure/azure.publishsettings
//
// Second, if there is no storage account in the subscription yet, add a classic storage
// account from the portal (not resource group)

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"

	"github.com/Azure/azure-sdk-for-go/management"
	"github.com/Azure/azure-sdk-for-go/management/hostedservice"
	"github.com/Azure/azure-sdk-for-go/management/virtualmachine"
	"github.com/Azure/azure-sdk-for-go/management/vmutils"
	uuid "github.com/satori/go.uuid"
)

const storageAccount string = "netsysstorage"
const clusterLocation string = "Central US"
const vmImage string = "b39f27a8b8c64d52b05eac6a62ebad85__Ubuntu-15_10-amd64-server-20151116.1-en-us-30GB"
const username string = "ubuntu"

type azureCluster struct {
	azureClient    management.Client
	hsClient       hostedservice.HostedServiceClient
	vmClient       virtualmachine.VirtualMachineClient
	namespace      string
	cloudConfig    string
	storageAccount string
	location       string
	vmImage        string
	username       string
	userPassword   string // Required password is a randomly generated UUID.
}

// Create an Azure cluster.
func (clst *azureCluster) Start(conn db.Conn, clusterID int, namespace string, keys []string) error {
	if namespace == "" {
		return errors.New("namespace cannot be empty")
	}

	keyfile := filepath.Join(os.Getenv("HOME"), ".azure", "azure.publishsettings")

	azureClient, err := management.ClientFromPublishSettingsFile(keyfile, "")
	if err != nil {
		return errors.New("error retrieving azure client from publishsettings")
	}

	clst.azureClient = azureClient
	clst.hsClient = hostedservice.NewClient(azureClient)
	clst.vmClient = virtualmachine.NewClient(azureClient)
	clst.namespace = namespace
	clst.cloudConfig = cloudConfigUbuntu(keys, "wily")
	clst.storageAccount = storageAccount
	clst.location = clusterLocation
	clst.vmImage = vmImage
	clst.username = username
	clst.userPassword = uuid.NewV4().String() // Randomly generate pwd

	return nil
}

// Retrieve list of instances.
func (clst *azureCluster) Get() ([]Machine, error) {
	var mList []Machine

	hsResponse, err := clst.hsClient.ListHostedServices()
	if err != nil {
		return nil, err
	}

	for _, hs := range hsResponse.HostedServices {
		if hs.Description != clst.namespace {
			continue
		}
		id := hs.ServiceName

		// Will return empty string if the hostedservice does not have a deployment.
		// e.g. some hosted services contains only a storage account, but no deployment.
		deploymentName, err := clst.vmClient.GetDeploymentName(id)
		if err != nil {
			return nil, err
		}

		if deploymentName == "" {
			clst.instanceDel(id)
			continue
		}

		deploymentResponse, err := clst.vmClient.GetDeployment(id, deploymentName)
		if err != nil {
			return nil, err
		}

		roleInstance := deploymentResponse.RoleInstanceList[0]
		privateIP := roleInstance.IPAddress
		publicIP := roleInstance.InstanceEndpoints[0].Vip
		size := roleInstance.RoleName

		mList = append(mList, Machine{
			ID:        id,
			PublicIP:  publicIP,
			PrivateIP: privateIP,
			Provider:  db.Azure,
			Size:      size,
		})
	}

	return mList, nil
}

// Boot Azure instances (blocking by calling instanceNew).
func (clst *azureCluster) Boot(bootSet []Machine) error {
	if len(bootSet) < 0 {
		panic("boot count cannot be negative")
	}

	for _, m := range bootSet {
		name := "di-" + uuid.NewV4().String()
		if err := clst.instanceNew(name, m.Size, clst.cloudConfig); err != nil {
			return err
		}
	}

	return nil
}

// Delete Azure instances (blocking by calling instanceDel).
func (clst *azureCluster) Stop(ids []string) error {
	for _, id := range ids {
		if err := clst.instanceDel(id); err != nil {
			return err
		}
	}
	return nil
}

// Disconnect.
func (clst *azureCluster) Disconnect() {
	// nothing
}

func (clst *azureCluster) PickBestSize(ram dsl.Range, cpu dsl.Range, maxPrice float64) string {
	return ""
}

// Create one Azure instance (blocking).
func (clst *azureCluster) instanceNew(name string, vmSize string, cloudConfig string) error {
	// create hostedservice
	err := clst.hsClient.CreateHostedService(
		hostedservice.CreateHostedServiceParameters{
			ServiceName: name,
			Description: clst.namespace,
			Location:    clst.location,
			Label:       base64.StdEncoding.EncodeToString([]byte(name)),
		})
	if err != nil {
		return err
	}

	role := vmutils.NewVMConfiguration(name, vmSize)
	mediaLink := fmt.Sprintf(
		"http://%s.blob.core.windows.net/vhds/%s.vhd",
		clst.storageAccount,
		name)
	vmutils.ConfigureDeploymentFromPlatformImage(
		&role,
		clst.vmImage,
		mediaLink,
		"")
	vmutils.ConfigureForLinux(&role, name, clst.username, clst.userPassword)
	vmutils.ConfigureWithPublicSSH(&role)

	role.ConfigurationSets[0].CustomData =
		base64.StdEncoding.EncodeToString([]byte(cloudConfig))

	operationID, err := clst.vmClient.CreateDeployment(
		role,
		name,
		virtualmachine.CreateDeploymentOptions{})
	if err != nil {
		clst.instanceDel(name)
		return err
	}

	// Block the operation.
	if err := clst.azureClient.WaitForOperation(operationID, nil); err != nil {
		clst.instanceDel(name)
		return err
	}

	return nil
}

// Delete one Azure instance by name (blocking).
func (clst *azureCluster) instanceDel(name string) error {
	operationID, err := clst.hsClient.DeleteHostedService(name, true)
	if err != nil {
		return err
	}

	// Block the operation.
	if err := clst.azureClient.WaitForOperation(operationID, nil); err != nil {
		return err
	}

	return nil
}

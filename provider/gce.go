package provider

////// ADD YOUR SSH KEY:
//
// 1) In the Google Developer Console navigate to:
//    Metadata > SSH Keys
//
////// SET UP API ACCESS:
//
// 1) In the Google Developer Console navigate to:
//    Permissions > Service accounts
//
// 2) Create or use an existing Service Account
//
// 3) For your Service Account, create and save a key as "~/.gce/di.json"
//
// 4) In the Google Developer Console navigate to:
//    Permissions > Permissions
//
// 5) If the Service Account is not already, assign it the "Editor" role.
//    You select the account by email.

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/NetSys/di/db"
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"

	log "github.com/Sirupsen/logrus"
)

const computeBaseURL string = "https://www.googleapis.com/compute/v1/projects"
const (
	// These are the various types of Operations that the GCE API returns
	local = iota
	global
)

var gAuthClient *http.Client    // the oAuth client
var gceService *compute.Service // gce service

type gceCluster struct {
	projID   string // gce project ID
	zone     string // gce zone
	imgURL   string // gce url to the VM image
	machType string // gce machine type
	baseURL  string // gce project specific url prefix

	ns          string // cluster namespace
	cloudConfig string
	id          int        // the id of the cluster, used externally
	aclTrigger  db.Trigger // for watching the acls
}

// Create a GCE cluster.
//
// Clusters are differentiated (namespace) by setting the description and
// filtering off of that.
//
// XXX: A lot of the fields are hardcoded.
func (clst *gceCluster) Start(conn db.Conn, clusterID int, namespace string, keys []string) error {
	if namespace == "" {
		panic("newGCE(): namespace CANNOT be empty")
	}

	err := gceInit()
	if err != nil {
		return err
	}

	clst.projID = "declarative-infrastructure"
	clst.zone = "us-central1-a"
	clst.machType = "f1-micro"
	clst.id = clusterID
	clst.ns = namespace
	clst.cloudConfig = cloudConfigCoreOS(keys)
	clst.aclTrigger = conn.TriggerTick(60, db.ClusterTable)
	clst.imgURL = fmt.Sprintf(
		"%s/%s",
		computeBaseURL,
		"coreos-cloud/global/images/coreos-beta-899-3-0-v20160115")
	clst.baseURL = fmt.Sprintf("%s/%s", computeBaseURL, clst.projID)

	err = clst.netInit()
	if err != nil {
		return err
	}

	err = clst.fwInit()
	if err != nil {
		return err
	}

	go clst.watchACLs(conn, clusterID)
	return nil
}

// Get a list of machines from the cluster
//
// XXX: This doesn't use the instance group listing functionality because
// listing that way doesn't get you information about the instances
func (clst *gceCluster) Get() ([]Machine, error) {
	list, err := gceService.Instances.List(clst.projID, clst.zone).
		Filter(fmt.Sprintf("description eq %s", clst.ns)).Do()
	if err != nil {
		return nil, err
	}
	var mList []Machine
	for _, item := range list.Items {
		// XXX: This make some iffy assumptions about NetworkInterfaces
		mList = append(mList, Machine{
			ID:        item.Name,
			PublicIP:  item.NetworkInterfaces[0].AccessConfigs[0].NatIP,
			PrivateIP: item.NetworkInterfaces[0].NetworkIP,
		})
	}
	return mList, nil
}

// Boots instances, it is a blocking call.
//
// XXX: currently ignores cloudConfig
// XXX: should probably have a better clean up routine if an error is encountered
func (clst *gceCluster) Boot(count int) error {
	if count < 0 {
		return errors.New("count must be >= 0")
	}
	var ops []*compute.Operation
	var urls []*compute.InstanceReference
	for i := 0; i < count; i++ {
		name := "di-" + uuid.NewV4().String()
		op, err := clst.instanceNew(name, clst.cloudConfig)
		if err != nil {
			return err
		}
		ops = append(ops, op)
		urls = append(urls, &compute.InstanceReference{
			Instance: op.TargetLink,
		})
	}
	err := clst.wait(ops, local)
	if err != nil {
		return err
	}
	return nil
}

// Deletes the instances, it is a blocking call.
//
// If an error occurs while deleting, it will finish the ones that have
// successfully started before returning.
//
// XXX: should probably have a better clean up routine if an error is encountered
func (clst *gceCluster) Stop(ids []string) error {
	var ops []*compute.Operation
	for _, id := range ids {
		op, err := clst.instanceDel(id)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"id":    id,
			}).Error("Failed to delete instance.")
			continue
		}
		ops = append(ops, op)
	}
	err := clst.wait(ops, local)
	if err != nil {
		return err
	}
	return nil
}

// Disconnect
func (clst *gceCluster) Disconnect() {
	panic("disconnect(): unimplemented! Check the comments on what this should actually do")
	// should cancel the ACL watch
	// should delete the instances
	// should delete the network
}

// Blocking wait with a hardcoded timeout.
//
// Waits on operations, the type of which is indicated by 'domain'. All
// operations must be of the same 'domain'
//
// XXX: maybe not hardcode timeout, and retry interval
func (clst *gceCluster) wait(ops []*compute.Operation, domain int) error {
	if len(ops) == 0 {
		return nil
	}

	after := time.After(3 * time.Minute)
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()

	var op *compute.Operation
	var err error
	for {
		select {
		case <-after:
			return fmt.Errorf("wait(): timeout")
		case <-tick.C:
			for len(ops) > 0 {
				switch {
				case domain == local:
					op, err = gceService.ZoneOperations.
						Get(clst.projID, clst.zone, ops[0].Name).Do()
				case domain == global:
					op, err = gceService.GlobalOperations.
						Get(clst.projID, ops[0].Name).Do()
				}
				if err != nil {
					return err
				}
				if op.Status != "DONE" {
					break
				}
				ops = append(ops[:0], ops[1:]...)
			}
			if len(ops) == 0 {
				return nil
			}
		}
	}
}

// Get a GCE instance.
func (clst *gceCluster) instanceGet(name string) (*compute.Instance, error) {
	ist, err := gceService.Instances.
		Get(clst.projID, clst.zone, name).Do()
	return ist, err
}

// Create new GCE instance.
//
// Does not check if the operation succeeds.
//
// XXX: all kinds of hardcoded junk in here
// XXX: currently only defines the bare minimum
func (clst *gceCluster) instanceNew(name string, cloudConfig string) (*compute.Operation, error) {
	instance := &compute.Instance{
		Name:        name,
		Description: clst.ns,
		MachineType: fmt.Sprintf("%s/zones/%s/machineTypes/%s",
			clst.baseURL,
			clst.zone,
			clst.machType),
		Disks: []*compute.AttachedDisk{
			{
				Boot:       true,
				AutoDelete: true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: clst.imgURL,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: fmt.Sprintf("%s/global/networks/%s",
					clst.baseURL,
					clst.ns),
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					// There is a generic startup script method and a
					// CoreOS specific way of startup scripting
					//
					//Key:   "startup-script", //XXX This is the GENERIC way
					Key:   "user-data", //XXX This is CoreOS SPECIFIC
					Value: &cloudConfig,
				},
			},
		},
	}

	op, err := gceService.Instances.
		Insert(clst.projID, clst.zone, instance).Do()
	if err != nil {
		return nil, err
	}
	return op, nil
}

// Delete a GCE instance.
//
// Does not check if the operation succeeds
func (clst *gceCluster) instanceDel(name string) (*compute.Operation, error) {
	op, err := gceService.Instances.Delete(clst.projID, clst.zone, name).Do()
	return op, err
}

func (clst *gceCluster) updateSecurityGroups(acls []string) error {
	op, err := clst.firewallPatch(clst.ns, acls)
	if err != nil {
		return err
	}
	err = clst.wait([]*compute.Operation{op}, global)
	if err != nil {
		return err
	}
	return nil
}

// Creates the network for the cluster.
func (clst *gceCluster) networkNew(name string) (*compute.Operation, error) {
	network := &compute.Network{
		Name: name,
	}

	op, err := gceService.Networks.Insert(clst.projID, network).Do()
	return op, err
}

func (clst *gceCluster) networkExists(name string) (bool, error) {
	list, err := gceService.Networks.List(clst.projID).Do()
	if err != nil {
		return false, err
	}
	for _, val := range list.Items {
		if val.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Creates the firewall for the cluster.
//
// XXX: Assumes there is only one network
func (clst *gceCluster) firewallNew(name string) (*compute.Operation, error) {
	firewall := &compute.Firewall{
		Name: name,
		Network: fmt.Sprintf("%s/global/networks/%s",
			clst.baseURL,
			clst.ns),
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"0-65535"},
			},
			{
				IPProtocol: "udp",
				Ports:      []string{"0-65535"},
			},
			{
				IPProtocol: "icmp",
			},
		},
		// XXX: This is just a dummy address to allow the rule to be created
		SourceRanges: []string{"127.0.0.1/32"},
	}

	op, err := gceService.Firewalls.Insert(clst.projID, firewall).Do()
	return op, err
}

func (clst *gceCluster) firewallExists(name string) (bool, error) {
	list, err := gceService.Firewalls.List(clst.projID).Do()
	if err != nil {
		return false, err
	}
	for _, val := range list.Items {
		if val.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Updates the firewall using PATCH semantics.
//
// The IP addresses must be in CIDR notation.
// XXX: Assumes there is only one network
// XXX: Assumes the firewall only needs to adjust the IP addrs affected
func (clst *gceCluster) firewallPatch(name string, ips []string) (*compute.Operation, error) {
	firewall := &compute.Firewall{
		Name: name,
		Network: fmt.Sprintf("%s/global/networks/%s",
			clst.baseURL,
			clst.ns),
		SourceRanges: ips,
	}

	op, err := gceService.Firewalls.Patch(clst.projID, name, firewall).Do()
	return op, err
}

func (clst *gceCluster) watchACLs(conn db.Conn, clusterID int) {
	for range clst.aclTrigger.C {
		var acls []string
		conn.Transact(func(view db.Database) error {
			clusters := view.SelectFromCluster(func(c db.Cluster) bool {
				return c.ID == clusterID
			})

			if len(clusters) == 0 {
				log.Warn("Undefined cluster")
				return nil
			} else if len(clusters) > 1 {
				panic("Duplicate Clusters")
			}

			acls = clusters[0].AdminACL
			return nil
		})

		clst.updateSecurityGroups(acls)
	}
}

// Initialize GCE.
//
// Authenication and the client are things that are re-used across clusters.
//
// Idempotent, can call multiple times but will only initialize once.
//
// XXX: ^but should this be the case? maybe we can just have the user call it?
func gceInit() error {
	if gAuthClient == nil {
		log.Debug("GCE initializing...")
		keyfile := filepath.Join(
			os.Getenv("HOME"),
			".gce",
			"di.json")
		err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyfile)
		if err != nil {
			return err
		}
		srv, err := newComputeService(context.Background())
		if err != nil {
			return err
		}
		gceService = srv
	} else {
		log.Debug("GCE already initialized! Skipping...")
	}
	log.Debug("GCE initialize success")
	return nil
}

func newComputeService(ctx context.Context) (*compute.Service, error) {
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, err
	}
	computeService, err := compute.New(client)
	if err != nil {
		return nil, err
	}
	return computeService, nil
}

// Initializes the network for the cluster
//
// XXX: Currently assumes that each cluster is entirely behind 1 network
func (clst *gceCluster) netInit() error {
	exists, err := clst.networkExists(clst.ns)
	if err != nil {
		return err
	}

	if exists {
		log.Debug("Network already exists")
		return nil
	}

	log.Debug("Creating network")
	op, err := clst.networkNew(clst.ns)
	if err != nil {
		return err
	}

	err = clst.wait([]*compute.Operation{op}, global)
	if err != nil {
		return err
	}
	return nil
}

// Initializes the firewall for the cluster
//
// XXX: Currently assumes that each cluster is entirely behind 1 network
// XXX: Currently only manipulates 1 firewall rule
func (clst *gceCluster) fwInit() error {
	exists, err := clst.firewallExists(clst.ns)
	if err != nil {
		return err
	}
	if exists {
		log.Debug("Firewall already exists")
		return nil
	}
	log.Debug("Creating firewall")
	op, err := clst.firewallNew(clst.ns)
	if err != nil {
		return err
	}

	err = clst.wait([]*compute.Operation{op}, global)
	if err != nil {
		return err
	}
	return nil
}

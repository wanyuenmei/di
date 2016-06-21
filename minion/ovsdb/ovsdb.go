package ovsdb

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	ovs "github.com/socketplane/libovsdb"
)

// The Ovsdb interface allows interaction with a locally running instance of the Open
// vSwitchd database.
type Ovsdb interface {
	Close()
	ListSwitches() ([]string, error)
	CreateSwitch(lswitch string) error
	DeleteSwitch(lswitch string) error
	ListPorts(lswitch string) ([]LPort, error)
	CreatePort(lswitch, name, mac, ip string) error
	DeletePort(lswitch, name string) error
	ListACLs(lswitch string) ([]Acl, error)
	CreateACL(lswitch string, dir string, priority int, match string, action string,
		doLog bool) error
	DeleteACL(lswitch string, dir string, priority int, match string) error
	DeleteOFPort(bridge, name string) error
	GetOFPortNo(name string) (int, error)
	CreateOFPort(bridge, name string) error
	ListOFPorts(bridge string) ([]string, error)
	GetDefaultOFInterface(port string) (Row, error)
	GetOFInterfaceType(iface Row) (string, error)
	GetOFInterfacePeer(iface Row) (string, error)
	GetOFInterfaceAttachedMAC(iface Row) (string, error)
	GetOFInterfaceIfaceID(iface Row) (string, error)
	SetOFInterfacePeer(name, peer string) error
	SetOFInterfaceAttachedMAC(name, mac string) error
	SetOFInterfaceIfaceID(name, ifaceID string) error
	SetOFInterfaceType(name, ifaceType string) error
	SetBridgeMac(lswitch, mac string) error
}

// Client is a connection to the ovsdb-server database.
type Client struct {
	*ovs.OvsdbClient
}

// LPort is a logical port in OVN.
type LPort struct {
	Name      string
	Addresses []string
}

// Acl is a firewall rule in OVN.
type Acl struct {
	uuid ovs.UUID
	Core AclCore
	Log  bool
}

// AclCore is the actual ACL rule that will be matched, without various OVSDB metadata
// found in Acl.
type AclCore struct {
	Priority  int
	Direction string
	Match     string
	Action    string
}

// ExistError is returned as a pointer (*ExistError) when what was searched
// for does not exist
type ExistError struct {
	msg string
}

// Row is an ovsdb row
type Row map[string]interface{}
type condition interface{}
type mutation interface{}

// Open creates a new Ovsdb connection.
// It's stored in a variable so we can mock it out for the unit tests.
var Open = func() (Ovsdb, error) {
	client, err := ovs.Connect("127.0.0.1", 6640)
	return Client{client}, err
}

// Close destroys an Ovsdb connection created by Open.
func (ovsdb Client) Close() {
	ovsdb.Disconnect()
}

// 'value' is normally a string, except under special conditions, upon which
// this function should recognize them and deal with it appropriately
func newCondition(column, function string, value interface{}) condition {
	if column == "_uuid" {
		switch t := value.(type) {
		case ovs.UUID:
			return ovs.NewCondition(column, function, value)
		case string:
			return ovs.NewCondition(column, function,
				ovs.UUID{GoUUID: value.(string)})
		default:
			log.WithFields(log.Fields{
				"value": value,
				"type":  t,
			}).Error("invalid type for value in condition")
			return nil
		}
	}
	return ovs.NewCondition(column, function, value)
}

// This does not cover all cases, they should just be added as needed
func newMutation(column, mutator string, value interface{}) mutation {
	switch typedValue := value.(type) {
	case ovs.UUID:
		uuidSlice := []ovs.UUID{typedValue}
		mutateValue, err := ovs.NewOvsSet(uuidSlice)
		if err != nil {
			// An error can only occur if the input is not a slice
			panic("newMutation(): an impossible error occurred")
		}
		return ovs.NewMutation(column, mutator, mutateValue)
	default:
		var mutateValue interface{}
		var err error
		switch reflect.ValueOf(typedValue).Kind() {
		case reflect.Slice:
			mutateValue, err = ovs.NewOvsSet(typedValue)
		case reflect.Map:
			mutateValue, err = ovs.NewOvsMap(typedValue)
		default:
			panic(fmt.Sprintf("unhandled value in mutation: value %s, type %s",
				value, typedValue))
		}
		if err != nil {
			return err
		}
		return ovs.NewMutation(column, mutator, mutateValue)
	}
}

func (ovsdb Client) selectRows(db string, table string,
	conds ...condition) ([]Row, error) {
	var anonymousConds []interface{}
	for _, c := range conds {
		anonymousConds = append(anonymousConds, c)
	}
	selectOp := ovs.Operation{
		Op:    "select",
		Table: table,
		Where: anonymousConds,
	}
	results, err := ovsdb.Transact(db, selectOp)
	if err != nil {
		return nil, err
	}
	var typedRows []Row
	for _, r := range results[0].Rows {
		typedRows = append(typedRows, r)
	}
	return typedRows, nil
}

func (ovsdb Client) getFromMapInRow(row Row, column, key string) (
	interface{}, error) {
	val, ok := row[column]
	if !ok {
		msg := fmt.Sprintf("column %s not found in row", column)
		return nil, &ExistError{msg}
	}
	goMap, err := ovsStringMapToMap(val)
	if err != nil {
		return nil, err
	}

	val, ok = goMap[key]
	if !ok {
		msg := fmt.Sprintf("key %s not found in column %s", key, column)
		return "", &ExistError{msg}
	}
	return val, nil
}

func ovsStringMapToMap(oMap interface{}) (map[string]string, error) {
	var ret = make(map[string]string)
	wrap, ok := oMap.([]interface{})
	if !ok {
		return nil, errors.New("ovs map outermost layer invalid")
	}
	if wrap[0] != "map" {
		return nil, errors.New("ovs map invalid identifier")
	}

	brokenMap, ok := wrap[1].([]interface{})
	if !ok {
		return nil, errors.New("ovs map content invalid")
	}
	for _, kvPair := range brokenMap {
		kvSlice, ok := kvPair.([]interface{})
		if !ok {
			return nil, errors.New("ovs map block must be a slice")
		}
		key, ok := kvSlice[0].(string)
		if !ok {
			return nil, errors.New("ovs map key must be string")
		}
		val, ok := kvSlice[1].(string)
		if !ok {
			return nil, errors.New("ovs map value must be string")
		}
		ret[key] = val
	}
	return ret, nil
}

func ovsStringSetToSlice(oSet interface{}) []string {
	var ret []string
	if t, ok := oSet.([]interface{}); ok && t[0] == "set" {
		for _, v := range t[1].([]interface{}) {
			ret = append(ret, v.(string))
		}
	} else {
		ret = append(ret, oSet.(string))
	}
	return ret
}

func ovsUUIDSetToSlice(oSet interface{}) []ovs.UUID {
	var ret []ovs.UUID
	if t, ok := oSet.([]interface{}); ok && t[0] == "set" {
		for _, v := range t[1].([]interface{}) {
			ret = append(ret, ovs.UUID{
				GoUUID: v.([]interface{})[1].(string),
			})
		}
	} else {
		ret = append(ret, ovs.UUID{
			GoUUID: oSet.([]interface{})[1].(string),
		})
	}
	return ret
}

func errorCheck(results []ovs.OperationResult, expectedResponses int,
	expectedCount int) error {
	totalCount := 0
	if len(results) < expectedResponses {
		return errors.New("mismatched responses and operations")
	}
	for i, result := range results {
		if result.Error != "" {
			return fmt.Errorf("[%d] %s: %s", i, result.Error, result.Details)
		}
		totalCount += result.Count
	}
	if totalCount != expectedCount {
		return errors.New("unexpected number of rows altered")
	}
	return nil
}

// ListSwitches queries the database for the logical switches in OVN.
func (ovsdb Client) ListSwitches() ([]string, error) {
	var switches []string
	results, err := ovsdb.selectRows("OVN_Northbound", "Logical_Switch",
		newCondition("_uuid", "!=", "_"))
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		// Only return names, because they are effectively UUIDs as OVSDB
		// enforces their uniqueness.
		switches = append(switches, result["name"].(string))
	}
	return switches, nil
}

// CreateSwitch creates a new logical switch in OVN.
func (ovsdb Client) CreateSwitch(lswitch string) error {
	check, err := ovsdb.selectRows("OVN_Northbound", "Logical_Switch",
		newCondition("name", "==", lswitch))
	if err != nil {
		return err
	}
	if len(check) > 0 {
		return &ExistError{
			fmt.Sprintf("logical switch %s already exists", lswitch),
		}
	}

	bridge := make(map[string]interface{})
	bridge["name"] = lswitch

	insertOp := ovs.Operation{
		Op:    "insert",
		Table: "Logical_Switch",
		Row:   bridge,
	}

	results, err := ovsdb.Transact("OVN_Northbound", insertOp)
	if err != nil {
		return errors.New("transaction error")
	}
	return errorCheck(results, 1, 2)
}

// DeleteSwitch removes a logical switch from OVN.
func (ovsdb Client) DeleteSwitch(lswitch string) error {
	deleteOp := ovs.Operation{
		Op:    "delete",
		Table: "Logical_Switch",
		Where: []interface{}{
			newCondition("name", "==", lswitch),
		},
	}
	results, err := ovsdb.Transact("OVN_Northbound", deleteOp)
	if err != nil {
		return err
	}
	return errorCheck(results, 1, 1)
}

// ListPorts lists the logical ports in OVN.
func (ovsdb Client) ListPorts(lswitch string) ([]LPort, error) {
	var lports []LPort
	results, err := ovsdb.selectRows("OVN_Northbound", "Logical_Switch",
		newCondition("name", "==", lswitch))
	if err != nil {
		return nil, err
	}
	if len(results) <= 0 {
		return lports, nil
	}
	ports := ovsUUIDSetToSlice(results[0]["ports"])
	for _, result := range ports {
		results, err = ovsdb.selectRows("OVN_Northbound", "Logical_Port",
			newCondition("_uuid", "==", result.GoUUID))
		if err != nil {
			return nil, err
		}
		if len(results) <= 0 {
			continue
		}
		lport := results[0]
		port := LPort{
			Name:      lport["name"].(string),
			Addresses: ovsStringSetToSlice(results[0]["addresses"]),
		}
		lports = append(lports, port)
	}
	return lports, nil
}

// CreatePort creates a new logical port in OVN.
func (ovsdb Client) CreatePort(lswitch, name, mac, ip string) error {
	// OVN Uses name an index into the Logical_Port table so, we need to check
	// no port called name already exists. This isn't strictly necessary, but it
	// makes our lives easier.
	rows, err := ovsdb.selectRows("OVN_Northbound", "Logical_Port",
		newCondition("name", "==", name))
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return &ExistError{fmt.Sprintf("port %s already exists", name)}
	}

	port := make(map[string]interface{})
	port["name"] = name
	addrs, err := ovs.NewOvsSet([]string{fmt.Sprintf("%s %s", mac, ip)})
	if err != nil {
		return err
	}
	port["addresses"] = addrs

	insertOp := ovs.Operation{
		Op:       "insert",
		Table:    "Logical_Port",
		Row:      port,
		UUIDName: "dilportadd",
	}

	mutateOp := ovs.Operation{
		Op:    "mutate",
		Table: "Logical_Switch",
		Mutations: []interface{}{
			newMutation("ports", "insert", ovs.UUID{GoUUID: "dilportadd"}),
		},
		Where: []interface{}{newCondition("name", "==", lswitch)},
	}

	results, err := ovsdb.Transact("OVN_Northbound", insertOp, mutateOp)
	if err != nil {
		return errors.New("transaction error")
	}
	return errorCheck(results, 2, 1)
}

// DeletePort removes a logical port from OVN.
func (ovsdb Client) DeletePort(lswitch, name string) error {
	rows, err := ovsdb.selectRows("OVN_Northbound", "Logical_Port",
		newCondition("name", "==", name))
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	uuid := ovs.UUID{GoUUID: rows[0]["_uuid"].([]interface{})[1].(string)}

	deleteOp := ovs.Operation{
		Op:    "delete",
		Table: "Logical_Port",
		Where: []interface{}{newCondition("_uuid", "==", uuid)},
	}

	mutateOp := ovs.Operation{
		Op:        "mutate",
		Table:     "Logical_Switch",
		Mutations: []interface{}{newMutation("ports", "delete", uuid)},
		Where:     []interface{}{newCondition("name", "==", lswitch)},
	}

	results, err := ovsdb.Transact("OVN_Northbound", deleteOp, mutateOp)
	if err != nil {
		return errors.New("transaction error")
	}
	return errorCheck(results, 2, 2)
}

// ListACLs lists the access control rules in OVN.
func (ovsdb Client) ListACLs(lswitch string) ([]Acl, error) {
	var acls []Acl
	results, err := ovsdb.selectRows("OVN_Northbound", "Logical_Switch",
		newCondition("name", "==", lswitch))
	if err != nil {
		return nil, err
	}
	if len(results) <= 0 {
		return acls, nil
	}
	aids := ovsUUIDSetToSlice(results[0]["acls"])
	for _, aid := range aids {
		aid := aid.GoUUID
		results, err := ovsdb.selectRows("OVN_Northbound", "ACL",
			newCondition("_uuid", "==", aid))
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			acl := Acl{
				uuid: ovs.UUID{GoUUID: result["_uuid"].([]interface{})[1].(string)},
				Core: AclCore{
					Priority:  int(result["priority"].(float64)),
					Direction: result["direction"].(string),
					Match:     result["match"].(string),
					Action:    result["action"].(string),
				},
				Log: result["log"].(bool),
			}
			acls = append(acls, acl)
		}
	}
	return acls, nil
}

// CreateACL creates an access control rule in OVN.
//
// The parameters to the CreatACL() and DeleteACL() functions correspond to columns
// of the ACL table of the OVN database in the following ways:
//
// dir corresponds to the "direction" column and, unless wildcarded, must be
// either "from-lport" or "to-lport"
//
// priority corresponds to the "priority" column and, unless wildcarded, must be
// in [1,32767]
//
// match corresponds to the "match" column and, unless wildcarded or empty, must
// be a valid OpenFlow expression
//
// action corresponds to the "action" column and must be one of the following
// values in {"allow", "allow-related", "drop", "reject"}
//
// doLog corresponds to the "log" column
//
// dir and match may be wildcarded by passing the value "*". priority may also
// be wildcarded by passing a value less than 0
func (ovsdb Client) CreateACL(lswitch string, dir string, priority int, match string,
	action string, doLog bool) error {
	acl := make(map[string]interface{})
	if dir != "*" {
		acl["direction"] = dir
	}
	if match != "*" {
		acl["match"] = match
	}
	acl["priority"] = int(math.Max(0.0, float64(priority)))
	acl["action"] = action
	acl["log"] = doLog

	insertOp := ovs.Operation{
		Op:       "insert",
		Table:    "ACL",
		Row:      acl,
		UUIDName: "diacladd",
	}

	mutateOp := ovs.Operation{
		Op:    "mutate",
		Table: "Logical_Switch",
		Mutations: []interface{}{
			newMutation("acls", "insert", ovs.UUID{GoUUID: "diacladd"}),
		},
		Where: []interface{}{newCondition("name", "==", lswitch)},
	}

	results, err := ovsdb.Transact("OVN_Northbound", insertOp, mutateOp)
	if err != nil {
		return errors.New("transaction error")
	}
	return errorCheck(results, 2, 1)
}

// DeleteACL removes an access control rule from OVN.
func (ovsdb Client) DeleteACL(lswitch string, dir string, priority int,
	match string) error {
	acls, err := ovsdb.ListACLs(lswitch)
	if err != nil {
		return err
	}
	for _, acl := range acls {
		if dir != "*" && acl.Core.Direction != dir {
			continue
		}
		if match != "*" && acl.Core.Match != match {
			continue
		}
		if priority >= 0 && acl.Core.Priority != priority {
			continue
		}

		deleteOp := ovs.Operation{
			Op:    "delete",
			Table: "ACL",
			Where: []interface{}{newCondition("_uuid", "==", acl.uuid)},
		}

		mutateOp := ovs.Operation{
			Op:    "mutate",
			Table: "Logical_Switch",
			Mutations: []interface{}{
				newMutation("acls", "delete", acl.uuid),
			},
			Where: []interface{}{newCondition("name", "==", lswitch)},
		}

		results, err := ovsdb.Transact("OVN_Northbound", deleteOp, mutateOp)
		if err != nil {
			return errors.New("transaction error")
		}
		check := errorCheck(results, 2, 2)
		if check != nil {
			return check
		}
	}
	return nil
}

func (ovsdb Client) getOFGeneric(table, name string) ([]Row, error) {
	results, err := ovsdb.selectRows("Open_vSwitch", table,
		newCondition("name", "==", name))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, &ExistError{
			fmt.Sprintf("no %s with name %s", strings.ToLower(table), name),
		}
	}

	return results, nil
}

// DeleteOFPort deletes an openflow port with the corresponding bridge and
// port names.
func (ovsdb Client) DeleteOFPort(bridge, name string) error {
	rows, err := ovsdb.selectRows("Open_vSwitch", "Port",
		newCondition("name", "==", name))
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	deleteOp := ovs.Operation{
		Op:    "delete",
		Table: "Port",
		Where: []interface{}{newCondition("name", "==", name)},
	}

	uuid := ovs.UUID{GoUUID: rows[0]["_uuid"].([]interface{})[1].(string)}
	mutateOp := ovs.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{newMutation("ports", "delete", uuid)},
		Where:     []interface{}{newCondition("name", "==", bridge)},
	}

	results, err := ovsdb.Transact("Open_vSwitch", deleteOp, mutateOp)
	if err != nil {
		return errors.New("transaction error")
	}
	return errorCheck(results, 2, 2)
}

// GetOFPortNo retrieves the OpenFlow port number of 'name' from OVS.
//
// Returns an error of type *ExistError if the port does not exist
func (ovsdb Client) GetOFPortNo(name string) (int, error) {
	// It takes some time for the OF port to get up, so we give
	// it a few chances, before we return an error.
	for i := 0; i < 3; i++ {
		results, err := ovsdb.getOFGeneric("Interface", name)
		if err != nil {
			return 0, err
		}

		if port, ok := results[0]["ofport"].(float64); ok {
			return int(port), nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return 0, &ExistError{fmt.Sprintf("interface %s has no openflow port", name)}
}

// CreateOFPort creates an openflow port on specified bridge
//
// A port cannot be created without an interface, that is why the "default"
// interface (one with the same name as the port) is created along with it.
func (ovsdb Client) CreateOFPort(bridge, name string) error {
	var ops []ovs.Operation

	ops = append(ops, ovs.Operation{
		Op:       "insert",
		Table:    "Interface",
		Row:      Row{"name": name},
		UUIDName: "diifaceadd",
	})

	ifaces, err := ovs.NewOvsSet([]ovs.UUID{{GoUUID: "diifaceadd"}})
	if err != nil {
		return err
	}

	ops = append(ops, ovs.Operation{
		Op:       "insert",
		Table:    "Port",
		Row:      Row{"name": name, "interfaces": ifaces},
		UUIDName: "diportadd",
	})

	ops = append(ops, ovs.Operation{
		Op:    "mutate",
		Table: "Bridge",
		Mutations: []interface{}{
			newMutation("ports", "insert", ovs.UUID{GoUUID: "diportadd"}),
		},
		Where: []interface{}{newCondition("name", "==", bridge)},
	})

	results, err := ovsdb.Transact("Open_vSwitch", ops...)
	if err != nil {
		return fmt.Errorf(
			"transaction error creating openflow port %s within %s: %s",
			name, bridge, err)
	}
	if err := errorCheck(results, 2, 1); err != nil {
		return fmt.Errorf("error creating openflow port %s within %s: %s",
			name, bridge, err)
	}
	return nil
}

// ListOFPorts lists all openflow ports on specified bridge
func (ovsdb Client) ListOFPorts(bridge string) ([]string, error) {
	var ports []string
	results, err := ovsdb.selectRows("Open_vSwitch", "Bridge",
		newCondition("name", "==", bridge))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []string{}, nil
	}
	portUUIDs := ovsUUIDSetToSlice(results[0]["ports"])
	for _, uuid := range portUUIDs {
		results, err = ovsdb.selectRows("Open_vSwitch", "Port",
			newCondition("_uuid", "==", uuid.GoUUID))
		if err != nil {
			return nil, err
		}
		ports = append(ports, results[0]["name"].(string))
	}
	return ports, nil
}

// GetDefaultOFInterface gets the default interface of the specified port,
// which is the interface with the same name
func (ovsdb Client) GetDefaultOFInterface(port string) (Row, error) {
	results, err := ovsdb.selectRows("Open_vSwitch", "Port",
		newCondition("name", "==", port))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("could not find openflow port %s", port)
	}
	ifaceUUIDs := ovsUUIDSetToSlice(results[0]["interfaces"])
	for _, uuid := range ifaceUUIDs {
		results, err = ovsdb.selectRows("Open_vSwitch", "Interface",
			newCondition("_uuid", "==", uuid.GoUUID))
		if err != nil {
			return nil, err
		}
		row := results[0]
		if row["name"].(string) == port {
			return row, nil
		}
	}
	err = fmt.Errorf("could not find default openflow interface for port %s",
		port)
	return nil, err
}

// GetOFInterfaceType gets the type of the interface given a Row
func (ovsdb Client) GetOFInterfaceType(iface Row) (string, error) {
	val, ok := iface["type"]
	if !ok {
		return "", &ExistError{"column type not found in interface row"}
	}
	return val.(string), nil
}

// GetOFInterfacePeer gets the peer of the interface given a Row
func (ovsdb Client) GetOFInterfacePeer(iface Row) (string, error) {
	val, err := ovsdb.getFromMapInRow(iface, "options", "peer")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetOFInterfaceAttachedMAC gets the attached-mac of the interface given a Row
func (ovsdb Client) GetOFInterfaceAttachedMAC(iface Row) (string, error) {
	val, err := ovsdb.getFromMapInRow(iface, "external_ids", "attached-mac")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetOFInterfaceIfaceID gets the iface-id of the interface given a Row
func (ovsdb Client) GetOFInterfaceIfaceID(iface Row) (string, error) {
	val, err := ovsdb.getFromMapInRow(iface, "external_ids", "iface-id")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

func (ovsdb Client) addToOFInterface(name string, mut mutation) error {
	mutateOp := ovs.Operation{
		Op:        "mutate",
		Table:     "Interface",
		Mutations: []interface{}{mut},
		Where:     []interface{}{newCondition("name", "==", name)},
	}

	results, err := ovsdb.Transact("Open_vSwitch", mutateOp)
	if err != nil {
		return fmt.Errorf("transaction error inserting into interface %s: %s",
			name, err)
	}
	return errorCheck(results, 1, 1)
}

func (ovsdb Client) updateOFInterface(name string, update Row) error {
	updateOp := ovs.Operation{
		Op:    "update",
		Table: "Interface",
		Where: []interface{}{newCondition("name", "==", name)},
		Row:   update,
	}

	results, err := ovsdb.Transact("Open_vSwitch", updateOp)
	if err != nil {
		return fmt.Errorf("transaction error updating interface %s: %s",
			name, err)
	}
	return errorCheck(results, 1, 1)
}

// SetOFInterfacePeer sets the peer of the interface
func (ovsdb Client) SetOFInterfacePeer(name, peer string) error {
	mut := newMutation("options", "insert", map[string]string{"peer": peer})
	if err := ovsdb.addToOFInterface(name, mut); err != nil {
		return fmt.Errorf("error setting interface %s to peer %s: %s",
			name, peer, err)
	}
	return nil
}

// SetBridgeMac sets the MAC address of the bridge
func (ovsdb Client) SetBridgeMac(lswitch, mac string) error {
	mut := newMutation("other_config", "insert", map[string]string{"hwaddr": mac})
	mutateOp := ovs.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mut},
		Where:     []interface{}{newCondition("name", "==", lswitch)},
	}

	results, err := ovsdb.Transact("Open_vSwitch", mutateOp)
	if err != nil {
		return fmt.Errorf("transaction error changing MAC of bridge %s: %s",
			lswitch, err)
	}
	return errorCheck(results, 1, 1)
}

// SetOFInterfaceAttachedMAC sets the attached-mac of the interface
func (ovsdb Client) SetOFInterfaceAttachedMAC(name, mac string) error {
	mut := newMutation("external_ids", "insert",
		map[string]string{"attached-mac": mac})
	if err := ovsdb.addToOFInterface(name, mut); err != nil {
		return fmt.Errorf("error setting interface %s to attached-mac %s: %s",
			name, mac, err)
	}
	return nil
}

// SetOFInterfaceIfaceID sets the iface-id of the interface
func (ovsdb Client) SetOFInterfaceIfaceID(name, ifaceID string) error {
	mut := newMutation("external_ids", "insert",
		map[string]string{"iface-id": ifaceID})
	if err := ovsdb.addToOFInterface(name, mut); err != nil {
		return fmt.Errorf("error setting interface %s to iface-id %s: %s",
			name, ifaceID, err)
	}
	return nil
}

// SetOFInterfaceType sets the type of the interface
func (ovsdb Client) SetOFInterfaceType(name, ifaceType string) error {
	if err := ovsdb.updateOFInterface(name, Row{"type": ifaceType}); err != nil {
		return fmt.Errorf("error setting interface %s to type %s: %s",
			name, ifaceType, err)
	}
	return nil
}

// Error returns the error message
func (e *ExistError) Error() string {
	return e.msg
}

// IsExist checks if the error is of type *ExistError
func IsExist(e error) bool {
	_, ok := e.(*ExistError)
	return ok
}

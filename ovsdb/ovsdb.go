package ovsdb

import (
	"errors"
	"fmt"
	"math"

	"github.com/socketplane/libovsdb"
)

type Ovsdb struct {
	*libovsdb.OvsdbClient
}

type LPort struct {
	Name      string
	Addresses []string
}

type Acl struct {
	uuid      libovsdb.UUID
	Priority  int
	Direction string
	Match     string
	Action    string
	Log       bool
}

func Open() (Ovsdb, error) {
	client, err := libovsdb.Connect("127.0.0.1", 6640)
	return Ovsdb{client}, err
}

func (ovsdb Ovsdb) Close() {
	ovsdb.Disconnect()
}

/* Helpers */
func (ovsdb Ovsdb) selectRows(db string, table string, field string, args []string,
	op string) []map[string]interface{} {
	var cond []interface{}
	for _, arg := range args {
		if field == "_uuid" {
			cond = libovsdb.NewCondition(field, op, libovsdb.UUID{arg})
		} else {
			cond = libovsdb.NewCondition(field, op, arg)
		}
		selectOp := libovsdb.Operation{
			Op:    "select",
			Table: table,
			Where: []interface{}{cond},
		}
		ops := []libovsdb.Operation{selectOp}
		results, _ := ovsdb.Transact(db, ops...)
		if len(results) == 1 {
			return results[0].Rows
		}
	}
	return make([]map[string]interface{}, 0)
}

func rwMutateOp(table string, column string, op string, condCol string,
	condOp string, condVal interface{}, uuidname string) libovsdb.Operation {
	mutateUuid := []libovsdb.UUID{{uuidname}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUuid)
	mutation := libovsdb.NewMutation(column, op, mutateSet)
	condition := libovsdb.NewCondition(condCol, condOp, condVal)
	return libovsdb.Operation{
		Op:        "mutate",
		Table:     table,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
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

func ovsUUIDSetToSlice(oSet interface{}) []libovsdb.UUID {
	var ret []libovsdb.UUID
	if t, ok := oSet.([]interface{}); ok && t[0] == "set" {
		for _, v := range t[1].([]interface{}) {
			ret = append(ret, libovsdb.UUID{
				v.([]interface{})[1].(string),
			})
		}
	} else {
		ret = append(ret, libovsdb.UUID{
			oSet.([]interface{})[1].(string),
		})
	}
	return ret
}

func errorCheck(results []libovsdb.OperationResult, expectedResponses int,
	expectedCount int) error {
	totalCount := 0
	if len(results) < expectedResponses {
		return errors.New("Mismatched numbers of responses and opeartions.")
	}
	for _, result := range results {
		if result.Error != "" {
			return errors.New(result.Error + ": " + result.Details)
		}
		totalCount += result.Count
	}
	if totalCount != expectedCount {
		return errors.New("Unexpected number of rows altered.")
	}
	return nil
}

/* LSwitch Queries */
func (ovsdb Ovsdb) ListSwitches() ([]string, error) {
	var switches []string
	results := ovsdb.selectRows("OVN_Northbound", "Logical_Switch", "_uuid",
		[]string{"_"}, "!=")
	for _, result := range results {
		/* Only return names, because they are effectively UUIDs as OVSDB
		 * enforces their uniqueness. */
		switches = append(switches, result["name"].(string))
	}
	return switches, nil
}

func (ovsdb Ovsdb) CreateSwitch(lswitch string) error {
	check := ovsdb.selectRows("OVN_Northbound", "Logical_Switch", "name",
		[]string{lswitch}, "==")
	if len(check) > 0 {
		return errors.New(fmt.Sprintf("Logical switch %s already exists", lswitch))
	}

	bridge := make(map[string]interface{})
	bridge["name"] = lswitch

	insertOp := libovsdb.Operation{
		Op:    "insert",
		Table: "Logical_Switch",
		Row:   bridge,
	}

	ops := []libovsdb.Operation{insertOp}
	results, err := ovsdb.Transact("OVN_Northbound", ops...)
	if err != nil {
		return errors.New("OVSDB Transaction error.")
	}
	return errorCheck(results, 1, 2)
}

func (ovsdb Ovsdb) DeleteSwitch(lswitch string) error {
	deleteOp := libovsdb.Operation{
		Op:    "delete",
		Table: "Logical_Switch",
		Where: []interface{}{
			libovsdb.NewCondition("name", "==", lswitch),
		},
	}
	ops := []libovsdb.Operation{deleteOp}
	results, err := ovsdb.Transact("OVN_Northbound", ops...)
	if err != nil {
		return err
	}
	return errorCheck(results, 1, 1)
}

/* LPort Queries */
func (ovsdb Ovsdb) ListPorts(lswitch string) ([]LPort, error) {
	var lports []LPort
	results := ovsdb.selectRows("OVN_Northbound", "Logical_Switch",
		"name", []string{lswitch}, "==")
	if len(results) <= 0 {
		return lports, nil
	}
	ports := ovsUUIDSetToSlice(results[0]["ports"])
	for _, result := range ports {
		pid := result.GoUuid
		results = ovsdb.selectRows("OVN_Northbound", "Logical_Port",
			"_uuid", []string{pid}, "==")
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

func (ovsdb Ovsdb) CreatePort(lswitch, name, mac, ip string) error {
	/* OVN Uses name an index into the Logical_Port table so, we need to check
	 * no port called name already exists. This isn't strictly necessary, but it
	 * makes our lives easier. */
	rows := ovsdb.selectRows("OVN_Northbound", "Logical_Port", "name",
		[]string{name}, "==")
	if len(rows) > 0 {
		return errors.New(fmt.Sprintf("Port %s already exists", name))
	}

	port := make(map[string]interface{})
	port["name"] = name
	addrs, err := libovsdb.NewOvsSet([]string{fmt.Sprintf("%s %s", mac, ip)})
	if err != nil {
		return err
	}
	port["addresses"] = addrs

	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "Logical_Port",
		Row:      port,
		UUIDName: "dilportadd",
	}

	mutateOp := rwMutateOp("Logical_Switch", "ports", "insert", "name", "==",
		lswitch, "dilportadd")

	ops := []libovsdb.Operation{insertOp, mutateOp}
	results, err := ovsdb.Transact("OVN_Northbound", ops...)
	if err != nil {
		return errors.New("OVSDB Transaction error.")
	}
	return errorCheck(results, 2, 1)
}

func (ovsdb Ovsdb) DeletePort(lswitch, name string) error {
	rows := ovsdb.selectRows("OVN_Northbound", "Logical_Port", "name",
		[]string{name}, "==")
	if len(rows) == 0 {
		return nil
	}
	uuid := libovsdb.UUID{rows[0]["_uuid"].([]interface{})[1].(string)}

	deleteOp := libovsdb.Operation{
		Op:    "delete",
		Table: "Logical_Port",
		Where: []interface{}{
			libovsdb.NewCondition("_uuid", "==", uuid),
		},
	}

	mutateOp := rwMutateOp("Logical_Switch", "ports", "delete", "name", "==",
		lswitch, uuid.GoUuid)

	ops := []libovsdb.Operation{deleteOp, mutateOp}
	results, err := ovsdb.Transact("OVN_Northbound", ops...)
	if err != nil {
		return errors.New("OVSDB Transaction error.")
	}
	return errorCheck(results, 2, 2)
}

/* ACL Queries */
func (ovsdb Ovsdb) ListACLs(lswitch string) ([]Acl, error) {
	var acls []Acl
	results := ovsdb.selectRows("OVN_Northbound", "Logical_Switch", "name",
		[]string{lswitch}, "==")
	if len(results) <= 0 {
		return acls, nil
	}
	aids := ovsUUIDSetToSlice(results[0]["acls"])
	for _, aid := range aids {
		aid := aid.GoUuid
		results := ovsdb.selectRows("OVN_Northbound", "ACL", "_uuid",
			[]string{aid}, "==")
		for _, result := range results {
			acl := Acl{
				uuid:      libovsdb.UUID{result["_uuid"].([]interface{})[1].(string)},
				Priority:  int(result["priority"].(float64)),
				Direction: result["direction"].(string),
				Match:     result["match"].(string),
				Action:    result["action"].(string),
				Log:       result["log"].(bool),
			}
			acls = append(acls, acl)
		}
	}
	return acls, nil
}

/* The parameters to the CreatACL() and DeleteACL() functions correspond to columns
 * of the ACL table of the OVN database in the following ways:
 *
 * dir corresponds to the "direction" column and, unless wildcarded, must be
 * either "from-lport" or "to-lport"
 *
 * priority corresponds to the "priority" column and, unless wildcarded, must be
 * in [1,32767]
 *
 * match corresponds to the "match" column and, unless wildcarded or empty, must
 * be a valid OpenFlow expression
 *
 * action corresponds to the "action" column and must be one of the following
 * values in {"allow", "allow-related", "drop", "reject"}
 *
 * doLog corresponds to the "log" column
 *
 * dir and match may be wildcarded by passing the value "*". priority may also
 * be wildcarded by passing a value less than 0
 */

func (ovsdb Ovsdb) CreateACL(lswitch string, dir string, priority int, match string,
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

	insertOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "ACL",
		Row:      acl,
		UUIDName: "diacladd",
	}

	mutateOp := rwMutateOp("Logical_Switch", "acls", "insert", "name", "==",
		lswitch, "diacladd")

	ops := []libovsdb.Operation{insertOp, mutateOp}
	results, err := ovsdb.Transact("OVN_Northbound", ops...)
	if err != nil {
		return errors.New("OVSDB Transaction error.")
	}
	return errorCheck(results, 2, 1)
}

func (ovsdb Ovsdb) DeleteACL(lswitch string, dir string, priority int, match string) error {
	acls, err := ovsdb.ListACLs(lswitch)
	if err != nil {
		return err
	}
	for _, acl := range acls {
		if dir != "*" && acl.Direction != dir {
			continue
		}
		if match != "*" && acl.Match != match {
			continue
		}
		if priority >= 0 && acl.Priority != priority {
			continue
		}

		deleteOp := libovsdb.Operation{
			Op:    "delete",
			Table: "ACL",
			Where: []interface{}{
				libovsdb.NewCondition("_uuid", "==", acl.uuid),
			},
		}

		mutateOp := rwMutateOp("Logical_Switch", "acls", "delete", "name", "==",
			lswitch, acl.uuid.GoUuid)

		ops := []libovsdb.Operation{deleteOp, mutateOp}
		results, err := ovsdb.Transact("OVN_Northbound", ops...)
		if err != nil {
			return errors.New("OVSDB Transaction error.")
		}
		check := errorCheck(results, 2, 2)
		if check != nil {
			return check
		}
	}
	return nil
}

/* OVS utilities */
func (ovsdb Ovsdb) GetOFPort(name string) (int, error) {
	results := ovsdb.selectRows("Open_vSwitch", "Interface", "name", []string{name},
		"==")
	if len(results) == 0 {
		return 0, errors.New(fmt.Sprintf("No interfaces with name %s", name))
	}

	port, ok := results[0]["ofport"].(float64)
	if !ok {
		return 0, fmt.Errorf("no openflow port")
	}

	return int(port), nil
}

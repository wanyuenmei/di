package network

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func delVeth(config netdev) error {
	if err := linkDelete("", config.name); err != nil {
		return err
	}
	return nil
}

func addVeth(config netdev) error {
	tmpPeer := tempVethPairName(config.name)
	// Create veth pair
	err := ipExec("", "link add %s type veth peer name %s",
		config.name, tmpPeer)
	if err != nil {
		return fmt.Errorf("error adding veth %s with peer %s: %s",
			config.name, tmpPeer, err)
	}

	if err := modVeth(netdev{name: config.name}, config); err != nil {
		return err
	}
	return nil
}

func modVeth(current netdev, target netdev) error {
	if current.name != target.name {
		panic("unmatched veths, cannot modify")
	}

	var err error
	tmpPeer := tempVethPairName(target.name)

	if ok, err := linkExists("", tmpPeer); err != nil {
		return err
	} else if ok {
		// Move the internal veth inside the container.
		err = ipExec("", "link set %s netns %s", tmpPeer, target.peerNS)
		if err != nil {
			return fmt.Errorf("error moving veth %s inside %s: %s",
				tmpPeer, target.peerNS, err)
		}
	}

	if exists, err := linkExists(target.peerNS, tmpPeer); err != nil {
		return err
	} else if exists && tmpPeer != innerVeth {
		// Rename the internal veth to the final name
		err = ipExec(target.peerNS, "link set dev %s name %s",
			tmpPeer, innerVeth)
		if err != nil {
			return fmt.Errorf("error renaming %s to %s in namespace %s: %s",
				target.name, innerVeth, target.peerNS, err)
		}
	}

	if current.peerMTU != target.peerMTU {
		err = ipExec(target.peerNS, "link set dev %s mtu %d",
			innerVeth, innerMTU)
		if err != nil {
			return fmt.Errorf("error setting veth %s inside %s to mtu %d: %s",
				innerVeth, target.peerNS, innerMTU, err)
		}
	}

	if current.up != target.up {
		var state string
		if target.up {
			state = "up"
		} else {
			state = "down"
		}
		if err = ipExec("", "link set %s %s", target.name, state); err != nil {
			return fmt.Errorf("error bringing veth %s %s: %s",
				target.name, state, err)
		}
		err = ipExec(target.peerNS, "link set %s %s", innerVeth, state)
		if err != nil {
			return fmt.Errorf("error bringing veth %s inside %s %s: %s",
				innerVeth, target.peerNS, state, err)
		}
	}
	return nil
}

func getLinkMTU(namespace, name string) (int, error) {
	res, err := linkQuery(namespace, name, "mtu")
	if err != nil {
		return 0, err
	}
	mtu, err := strconv.Atoi(res)
	if err != nil {
		return 0, err
	}
	return mtu, nil
}

func linkExists(namespace, name string) (bool, error) {
	cmd := fmt.Sprintf("ip link show %s", name)
	if namespace != "" {
		cmd = fmt.Sprintf("ip netns exec %s %s", namespace, cmd)
	}
	stdout, _, err := shVerbose(cmd)
	// If err is of type *ExitError then that means it has a non-zero exit
	// code which we are okay with
	if _, ok := err.(*exec.ExitError); !ok && err != nil {
		err = fmt.Errorf("error checking if link %s exists in %s: %s",
			name, namespaceName(namespace), err)
		return false, err
	}
	if string(stdout) == "" {
		return false, nil
	}
	return true, nil
}

// Interprets the empty string as the "root" namespace
func linkDelete(namespace, name string) error {
	if err := ipExec(namespace, "link delete %s", name); err != nil {
		return fmt.Errorf("error deleting link %s in %s: %s",
			name, namespaceName(namespace), err)
	}
	return nil
}

// Query the link for information
// Interprets the empty string as the "root" namespace
func linkQuery(namespace, name, query string) (string, error) {
	// The output for `ip link show` gives a field name followed
	// by the value. We take advantage of this fact while parsing.

	var cmd string

	stdout, _, err := ipExecVerbose(namespace, "link show %s", name)
	if err != nil {
		return "", fmt.Errorf("query command failed: %s: %s", cmd, err)
	}

	splitStr := strings.Fields(string(stdout))
	for i := 0; i < len(splitStr)-1; i++ {
		if splitStr[i] == query {
			return splitStr[i+1], nil
		}
	}

	err = fmt.Errorf("could not find link %s in %s",
		name, namespaceName(namespace))
	return "", err
}

// Lists all veths in the root namespace
func listVeths() ([]string, error) {
	var veths []string

	stdout, _, err := ipExecVerbose("", "link show type veth")
	if err != nil {
		return nil, fmt.Errorf("failed to list veths: %s", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	skipRE := regexp.MustCompile("^\\s+.*")
	vethRE := regexp.MustCompile("^\\d+: (\\w+)@.*")
	for scanner.Scan() {
		line := scanner.Text()
		if skipRE.FindStringIndex(line) != nil {
			// Skip if the line begins with whitespace
			continue
		}
		match := vethRE.FindStringSubmatch(line)
		if match == nil || len(match) != 2 {
			return nil, errors.New("list of veths is not parsing properly")
		}
		veths = append(veths, match[1])
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error while getting veths: %s", err)
	}
	return veths, nil
}

func listIP(namespace, dev string) ([]string, error) {
	var ips []string

	stdout, _, err := ipExecVerbose(namespace, "addr list dev %s", dev)
	if err != nil {
		return nil, fmt.Errorf("failed to list ip addresses in %s: %s",
			namespaceName(namespace), err)
	}

	re, _ := regexp.Compile(`(?:inet|inet6) (\S+)`)
	for _, v := range re.FindAllSubmatch(stdout, -1) {
		ips = append(ips, string(v[1]))
	}

	return ips, nil
}

func addIP(namespace, ip, dev string) error {
	err := ipExec(namespace, "addr add %s dev %s", ip, dev)
	if err != nil {
		return fmt.Errorf("failed to add ip %s to %s in %s: %s",
			ip, dev, namespaceName(namespace), err)
	}
	return nil
}

func delIP(namespace, ip, dev string) error {
	err := ipExec(namespace, "addr del %s dev %s", ip, dev)
	if err != nil {
		return fmt.Errorf("failed to delete ip %s in %s: %s",
			ip, namespaceName(namespace), err)
	}
	return nil
}

func getMac(namespace, dev string) (string, error) {
	return linkQuery(namespace, dev, "link/ether")
}

func setMac(namespace, dev, mac string) error {
	err := ipExec(namespace, "link set dev %s address %s", dev, mac)
	if err != nil {
		return fmt.Errorf("failed to set mac %s for %s in %s: %s",
			mac, dev, namespaceName(namespace), err)
	}
	return nil
}

func upLink(namespace, dev string) error {
	up, err := linkIsUp(namespace, dev)
	if up || err != nil {
		return err
	}

	if err = ipExec(namespace, "link set dev %s up", dev); err != nil {
		return fmt.Errorf("failed to set %s up in %s: %s",
			dev, namespaceName(namespace), err)
	}

	return nil
}

func linkIsUp(namespace, dev string) (bool, error) {
	stdout, _, err := ipExecVerbose(namespace, "link show %s", dev)
	if err != nil {
		return false, fmt.Errorf("failed to show %s: %s", dev, err)
	}

	pattern := fmt.Sprintf("^\\d+:\\s%s:\\s<.*,UP.*>.*", dev)
	stateRE := regexp.MustCompile(pattern)
	return stateRE.MatchString(string(stdout)), nil
}

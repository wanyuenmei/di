package config

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "reflect"
    "time"

    "github.com/op/go-logging"

    "github.com/NetSys/di/util"
)

type Config struct {
    Namespace string

    RedCount int
    BlueCount int
    WorkerCount int         /* Number of worker VMs */
    MasterCount int         /* Number of master VMs */
    Region string           /* AWS availability zone */

    AdminACL []string
    SSHAuthorizedKeys []string
}

var log = logging.MustGetLogger("config")


/* Convert 'cfg' its string representation. */
func (cfg Config) String() string {
    str := fmt.Sprintf(
        "{\n\tNamespace: %s,\n\tMasterCount: %d\n\tWorkerCount: %d,\n\tRegion: %s\n}",
        cfg.Namespace, cfg.MasterCount, cfg.WorkerCount, cfg.Region)
    return str
}

func parseConfig(config_path string) *Config {
    var config Config

    config_file, err := ioutil.ReadFile(config_path)
    if err != nil {
        log.Warning("Error reading config")
        log.Warning(err.Error())
        return nil
    }

    err = json.Unmarshal(config_file, &config)
    if err != nil {
        log.Warning("Malformed config")
        log.Warning(err.Error())
        return nil
    }

    for i, acl := range config.AdminACL {
        if acl == "local" {
            ip, err := util.MyIp()
            if err != nil {
                log.Warning("Failed to get local IP address. Skipping ACL: %s",
                            err)
            } else {
                config.AdminACL[i] = ip + "/32"
            }
        }
    }

    /* XXX: There's research in this somewhere.  How do we validate inputs into
    * the policy?  What do we do with a policy that's wrong?  Also below, we
    * want someone to be able to say "limit the number of instances for cost
    * reasons" ... look at what's going on in amp for example.  100k in a month
    * is crazy. */

    return &config
}

func watchConfigForUpdates(config_path string, config_chan chan Config) {
    var old_config *Config

    old_config = nil

    for {
        new_config := parseConfig(config_path)
        if new_config != nil && (old_config == nil ||
        !reflect.DeepEqual(*old_config, *new_config)) {
            config_chan <- *new_config
            old_config = new_config
        }
        time.Sleep(2 * time.Second)
    }
}

func MasterCloudConfig(cfg Config, token string) string {
    return cloudConfig(cfg, true, token, "localhost")
}

func WorkerCloudConfig(cfg Config, token, master_ip string) string {
    return cloudConfig(cfg, false, token, master_ip)
}

func cloudConfig(cfg Config, master bool, token, master_ip string) string {
    cloud_config := "#cloud-config\n\n"

    if len(cfg.SSHAuthorizedKeys) > 0 {
        cloud_config += "ssh_authorized_keys:\n"
        for _, key := range cfg.SSHAuthorizedKeys {
            cloud_config += fmt.Sprintf("    - \"%s\"\n", key)
        }
    }

    cloud_config += `
coreos:
    etcd2:`

    if master {
        cloud_config += fmt.Sprintf(`
        discovery: %s
        advertise-client-urls: http://$private_ipv4:2379,http://$private_ipv4:4001
        initial-advertise-peer-urls: http://$private_ipv4:2380
        listen-client-urls: http://$private_ipv4:2379,http://127.0.0.1:2379
        listen-peer-urls: http://$private_ipv4:2380
        `, token)
    } else {
        cloud_config += fmt.Sprintf(`
        proxy: on
        discovery: %s
        listen-client-urls: http://127.0.0.1:4001,http://127.0.0.1:2379
        `, token)
    }

    cloud_config += `
    units:
        - name: etcd2.service
          command: start
        - name: fleet.service
          command: start
        - name: docker.service
          command: start
          content: |
            [Unit]
            Description=Docker
            After=etcd2.service
            Requires=etcd2.service

            [Service]
            ExecStartPre=/usr/bin/mkdir -p /opt
            ExecStartPre=/usr/bin/chmod 777 /opt
            ExecStartPre=/usr/bin/wget \
                https://get.docker.com/builds/Linux/x86_64/docker-1.9.0 \
                -O /opt/docker
            ExecStartPre=/usr/bin/chmod a+x /opt/docker
            ExecStart=/opt/docker daemon --cluster-store=etcd://127.0.0.1:4001
        - name: di-minion.service
          command: start
          content: |
            [Unit]
            Description=DI Minion
            After=docker.service
            Requires=docker.service
            After=systemd-networkd.service

            [Service]
            ExecStart=/opt/docker run -d --privileged --net=host \
                      --name=minion ethanjjackson/di-minion`

    if master {
        cloud_config += `
        - name: ovn.service
          command: start
          content: |
            [Unit]
            Description=Open vSwitch virtual network
            After=docker.service

            [Service]
            Restart=always
            ExecStartPre=/opt/docker pull melvinw/ubuntu-ovn
            ExecStart=/opt/docker run -itd \
                --privileged --net=host --name=ovn \
                -v /etc/docker:/etc/docker:rw \
                -v /var/run/docker.sock:/var/run/docker.sock:rw \
                melvinw/ubuntu-ovn
            ExecStartPost=/usr/bin/sleep 5
            ExecStartPost=/opt/docker start ovn
            ExecStartPost=/opt/docker exec ovn \
                 mkdir -p /usr/local/var/log/openvswitch \
                /usr/local/var/lib/openvswitch \
                /usr/local/var/lib/openvswitch/pki \
                /usr/local/var/run/openvswitch \
                /usr/local/etc/openvswitch
            ExecStartPost=/opt/docker exec ovn \
                ovsdb-tool create /usr/local/etc/openvswitch/conf.db \
                /usr/local/share/openvswitch/vswitch.ovsschema
            ExecStartPost=/opt/docker exec ovn ovsdb-server \
                --remote=punix:/usr/local/var/run/openvswitch/db.sock \
                --remote=db:Open_vSwitch,Open_vSwitch,manager_options \
                --log-file=/usr/local/var/log/openvswitch/ovsdb-server.log \
                --pidfile --detach
            ExecStartPost=/opt/docker exec ovn ovs-appctl -t ovsdb-server \
                ovsdb-server/add-remote ptcp:6640
            ExecStartPost=/opt/docker exec ovn \
                /usr/local/share/openvswitch/scripts/ovn-ctl start_northd
            ExecStartPost=/opt/docker exec ovn \
                ovn-nbctl --db=unix:/usr/local/var/run/openvswitch/db.sock \
                lswitch-add di_net`

    } else {
        cloud_config += `
        - name: ovs.service
          command: start
          content: |
            [Unit]
            Description=Open vSwitch
            After=docker.service
            Requires=docker.service

            [Service]
            Restart=always
            ExecStartPre=/sbin/modprobe openvswitch
            ExecStartPre=/opt/docker pull melvinw/ubuntu-ovn
            ExecStart=/opt/docker run -itd \
                --privileged --net=host --name=ovn \
                -v /etc/docker:/etc/docker:rw \
                -v /var/run/docker.sock:/var/run/docker.sock:rw \
                melvinw/ubuntu-ovn
            ExecStartPost=/usr/bin/sleep 5
            ExecStartPost=/opt/docker start ovn
            ExecStartPost=/opt/docker exec ovn \
                mkdir -p /usr/local/var/log/openvswitch \
                /usr/local/var/lib/openvswitch \
                /usr/local/var/lib/openvswitch/pki \
                /usr/local/var/run/openvswitch \
                /usr/local/etc/openvswitch
            ExecStartPost=/opt/docker exec ovn \
                ovsdb-tool create /usr/local/etc/openvswitch/conf.db \
                /usr/local/share/openvswitch/vswitch.ovsschema
            ExecStartPost=/opt/docker exec ovn ovsdb-server \
                --remote=punix:/usr/local/var/run/openvswitch/db.sock \
                --remote=db:Open_vSwitch,Open_vSwitch,manager_options \
                --log-file=/usr/local/var/log/openvswitch/ovsdb-server.log \
                --pidfile --detach
            ExecStartPost=/opt/docker exec ovn \
                ovs-vsctl --no-wait init
            ExecStartPost=/opt/docker exec ovn \
                ovs-vswitchd --pidfile --detach \
                --log-file=/usr/local/var/log/openvswitch/ovs-vswitchd.log

        - name: ovn.service
          command: start
          content: |
            [Unit]
            Description=Open vSwitch virtual network
            After=ovs.service

            [Service]
            Restart=always
            ExecStartPre=/opt/docker exec ovn ovs-vsctl set Open_vSwitch . \
                external_ids:ovn-remote="tcp:%s:6640" \
                external_ids:ovn-encap-ip=$public_ipv4 external_ids:ovn-encap-type="geneve"
            ExecStart=/opt/docker exec ovn \
                /usr/local/share/openvswitch/scripts/ovn-ctl start_controller
            ExecStartPost=/opt/docker exec ovn \
                /opt/ovn-docker/ovn-docker-overlay-driver --detach`
    }

    return cloud_config
}

func WatchConfig(config_path string) chan Config {
    config_chan := make(chan Config)
    go watchConfigForUpdates(config_path, config_chan)
    return config_chan
}

package config

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "reflect"
    "time"

    "github.com/op/go-logging"
)

type Config struct {
    Namespace string

    RedCount int
    BlueCount int
    HostCount int           /* Number of VMs */
    Region string           /* AWS availability zone */

    AdminACL []string

    /* Path to cloud config in the json file.  Contents of the cloud config
    * when used outside of this module. */
    CloudConfig string
}

var log = logging.MustGetLogger("config")


/* Convert 'cfg' its string representation. */
func (cfg Config) String() string {
    str := fmt.Sprintf(
        "{\n\tNamespace: %s,\n\tHostCount: %d,\n\tRegion: %s\n}",
        cfg.Namespace, cfg.HostCount, cfg.Region)
    return str
}

func getMyIp () string {
    resp, err := http.Get("http://checkip.amazonaws.com/")
    if err != nil {
        panic(err)
    }

    defer resp.Body.Close()
    body_byte, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        panic(err)
    }

    body := string(body_byte)
    return body[:len(body) - 1]
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

    cfg, err := ioutil.ReadFile(config.CloudConfig)
    if err != nil {
        log.Warning("Error reading cloud config")
        log.Warning(err.Error())
        return nil
    }
    config.CloudConfig = string(cfg)

    for i, acl := range config.AdminACL {
        if acl == "local" {
            config.AdminACL[i] = getMyIp() + "/32"
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

func WatchConfig(config_path string) chan Config {
    config_chan := make(chan Config)
    go watchConfigForUpdates(config_path, config_chan)
    return config_chan
}

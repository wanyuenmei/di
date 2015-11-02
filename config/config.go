package config

import (
    "encoding/json"
    "io/ioutil"

    "gopkg.in/fsnotify.v1"
    "github.com/op/go-logging"
    "github.com/NetSys/di/cluster"
)

type di_config struct {
    RedCount int
    BlueCount int
    HostCount int           /* Number of VMs */
    CloudConfig string      /* Path to cloud config */
    Region string           /* AWS availability zone */
}

var log = logging.MustGetLogger("config")

func parseConfig(config_path string) *cluster.Config {
    var temp_config di_config
    var config cluster.Config

    config_file, err := ioutil.ReadFile(config_path)
    if err != nil {
        log.Warning("Error reading config")
        log.Warning(err.Error())
        return nil
    }

    err = json.Unmarshal(config_file, &temp_config)
    if err != nil {
        log.Warning("Malformed config")
        log.Warning(err.Error())
        return nil
    }

    /* XXX: There's research in this somewhere.  How do we validate inputs into
    * the policy?  What do we do with a policy that's wrong?  Also below, we
    * want someone to be able to say "limit the number of instances for cost
    * reasons" ... look at what's going on in amp for example.  100k in a month
    * is crazy. */
    config.InstanceCount = temp_config.HostCount
    config.Region = temp_config.Region
    config.CloudConfig, err = ioutil.ReadFile(temp_config.CloudConfig)
    if err != nil {
        log.Warning("Error reading cloud config")
        log.Warning(err.Error())
        return nil
    }

    return &config
}

func watchConfigForUpdates(config_path string,
                           config_chan chan cluster.Config) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        panic(err)
    }
    defer watcher.Close()

    err = watcher.Add(config_path)
    if err != nil {
        panic(err)
    }

    /* Read initial config */
    new_config := parseConfig(config_path)
    if new_config != nil {
        config_chan <- *new_config
    }

    for {
        select {
            case e := <-watcher.Events:
                new_config := parseConfig(e.Name)
                if new_config != nil {
                    config_chan <- *new_config
                }

                /* XXX: Some editors (e.g. vim) trigger a rename event, even
                 * if the filename doesn't actually change. This results in the
                 * old listener becoming stale. If there's a cleaner way to do
                 * this, let's replace this. */
                if e.Op == fsnotify.Rename {
                    watcher.Remove(e.Name)
                    watcher.Add(config_path)
                }
            case err := <-watcher.Errors:
                panic(err)
            default:
                continue
        }
    }
}

func WatchConfig(config_path string) chan cluster.Config {
    config_chan := make(chan cluster.Config)
    go watchConfigForUpdates(config_path, config_chan)
    return config_chan
}

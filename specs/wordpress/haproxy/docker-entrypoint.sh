#!/bin/sh
set -e

# Wait until servers are reachable
#
# Takes in a comma separated list of hostnames
waitServers() {
    serverList=$1

    savedIFS=$IFS
    IFS=','
    for server in $serverList; do
        echo "trying to reach $server"
        until ping -q -c1 "$server" > /dev/null 2>&1; do
            sleep 1
        done
        echo "successfully reached $server"
    done
    IFS=$savedIFS
}

# Generate a web server entry to go into the haproxy.cfg
genServerLine() {
    serverID=$1
    serverHostname=$2

    echo "    server $serverID ${serverHostname}:80 check"
}

# Inserts a line in the designated server list area
insertServerLine() {
    targetFile=$1
    inputLine=$2

    footerLine="### END SERVERLIST ###"
    sed -i -e "s/$footerLine/${inputLine}\n$footerLine/" "$targetFile"
}

hostnameToID() {
    inputHostname=$1

    # This assumes hostnames are of form "<stuff>.di" and removes ".di"
    id=$(echo "$inputHostname" | sed -e 's/...$//')
    echo "$id"
}

# Adds server entries into designated config file
#
# Takes in a comma separated list of hostnames
addServersToConfig() {
    configFile="$1"
    serverList="$2"

    savedIFS=$IFS
    IFS=','
    for server in $serverList; do
        serverID=$(hostnameToID "$server")
        serverLine=$(genServerLine "$serverID" "$server")
        insertServerLine "$configFile" "$serverLine"
    done
    IFS=$savedIFS
}


### Begin Main Script ###


ConfigFile="/usr/local/etc/haproxy/haproxy.cfg"

ServerList=$1
shift

# first arg is `-f` or `--some-option`
if [ "${1#-}" != "$1" ]; then
	set -- haproxy "$@"
fi

if [ "$1" = 'haproxy' ]; then
	# if the user wants "haproxy", let's use "haproxy-systemd-wrapper" instead so we can have proper reloadability implemented by upstream
	shift # "haproxy"
	set -- "$(which haproxy-systemd-wrapper)" -p /run/haproxy.pid "$@"
fi

addServersToConfig "$ConfigFile" "$ServerList"
waitServers "$ServerList"

exec "$@"

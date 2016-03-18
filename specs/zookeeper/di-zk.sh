#! /bin/sh

# Wait for eth0 to show up.
until ip link show eth0
do
    sleep 1
done

until grep zoo /etc/hosts
do
    sleep 1
done

MYID=$1

FILE=/opt/zookeeper/conf/zoo.cfg
DATA=/tmp/zookeeper

echo $MYID > $DATA/myid

cat << EOF > $FILE
tickTime=2000
dataDir=$DATA
clientPort=2181
initLimit=5
syncLimit=2
EOF

id=1
for i in `echo $2 | tr "," "\n"`
do
    echo server.$id=$i\:2888\:3888 >> $FILE
    id=$((id+1))
done

cat $FILE
/opt/zookeeper/bin/zkServer.sh start-foreground

#! /bin/sh

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

echo "trying to resolve $(hostname)"
until hostname -f > /dev/null 2>&1; do
    sleep 1
done
echo "successfully resolved $(hostname)"

/opt/zookeeper/bin/zkServer.sh start-foreground

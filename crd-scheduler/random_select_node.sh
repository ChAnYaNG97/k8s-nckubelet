#!/bin/bash
SERVER='localhost:8001'
NODES=($(kubectl --server $SERVER get nodes -o json | jq '.items[].metadata.name' | tr -d '"'))        
echo ${NODES[@]}
NUMNODES=${#NODES[@]}
echo $NUMNODES

while [ "$CHOSEN" = "master" -o "$CHOSEN" = "" ];
do
    CHOSEN=${NODES[$[ $RANDOM % $NUMNODES ]]}
    echo $CHOSEN
done

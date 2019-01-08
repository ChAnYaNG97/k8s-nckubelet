#!/bin/bash
SERVER='localhost:8001'
CRD_PLURAL='ncapps'
CRD_SINGULAR='ncapp'
while true;
do
    for CRDNAME in $(kubectl --server $SERVER get $CRD_PLURAL -o json | jq '.items[] | select(.spec.nodeName == null) | .metadata.name' | tr -d '"')
    do
        echo $CRDNAME
        NODES=($(kubectl --server $SERVER get nodes -o json | jq '.items[].metadata.name' | tr -d '"'))        
        echo ${NODES[@]}
        NUMNODES=${#NODES[@]}
        echo $NUMNODES
        CHOSEN=""
        while [ "$CHOSEN" = "master" -o "$CHOSEN" = "" ];
        do
            CHOSEN=${NODES[$[ $RANDOM % $NUMNODES ]]}
            echo $CHOSEN
        done
        kubectl patch $CRD_SINGULAR $CRDNAME --type merge --patch $'spec:\n nodeName: '$CHOSEN''
    done

    sleep 1

done
     

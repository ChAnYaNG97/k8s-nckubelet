#!/bin/bash
SERVER='localhost:8001'
CRDTYPE='myapps'
for CRDNAME in $(kubectl --server $SERVER get $CRDTYPE -o json | jq '.items[] | select(.spec.nodeName == null) | .metadata.name' | tr -d '"')
do
    echo $CRDNAME
done

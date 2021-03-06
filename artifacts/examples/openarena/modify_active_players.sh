#!/bin/bash

kubectl create -f dedicatedgameservercollection.yaml

# demo scale out

# get DGS names
dgs=`kubectl get dgs -l DedicatedGameServerCollectionName=openarena | cut -d ' ' -f 1 | sed 1,1d`
# update DGS.Spec.ActivePlayers
kubectl patch dgs $dgs -p '[{ "op": "replace", "path": "/status/activePlayers", "value": 5 },]' --type='json'

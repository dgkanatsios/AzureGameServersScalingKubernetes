#!/bin/bash

kubectl create -f dedicatedgameservercollection.yaml

# demo scale out

# get DGS names
dgs=`kubectl get dgs -l DedicatedGameServerCollectionName=openarena-collection-example | cut -d ' ' -f 1 | sed 1,1d`
# update DGS.Spec.ActivePlayers
kubectl patch dgs $dgs -p '[{ "op": "replace", "path": "/spec/activePlayers", "value": 9 },]' --type='json'
# update DGS.Labels[ActivePlayers]
kubectl label dgs $dgs ActivePlayers=9 --overwrite

# demo scale in
# get DGS names - again
dgs=`kubectl get dgs -l DedicatedGameServerCollectionName=openarena-collection-example | cut -d ' ' -f 1 | sed 1,1d`
# update DGS.Spec.ActivePlayers
kubectl patch dgs $dgs -p '[{ "op": "replace", "path": "/spec/activePlayers", "value": 5 },]' --type='json'
# update DGS.Labels[ActivePlayers]
kubectl label dgs $dgs ActivePlayers=5 --overwrite

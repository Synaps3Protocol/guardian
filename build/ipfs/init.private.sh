
#!/bin/bash

echo "Running ipfs in ${IPFS_PATH}"

if [ ! -e ${IPFS_PATH}/config ]; then
       echo "Initializing IPFS repo at ${IPFS_PATH}"
       ipfs init --empty-repo
fi

# shellcheck disable=SC2006
ipfs config Addresses.API '/ip4/127.0.0.1/tcp/5001'
ipfs config Addresses.Gateway '/ip4/127.0.0.1/tcp/8080'
ipfs bootstrap rm all

ipfs config --json Import.CidVersion '1'
ipfs config --json Experimental.FilestoreEnabled true
ipfs config --json Experimental.UrlstoreEnabled false

ipfs config Swarm.Transports.Network.Websocket --json true
ipfs config Swarm.Transports.Network.WebTransport --json false
ipfs config Swarm.Transports.Network.WebRTCDirect --json false
ipfs config Swarm.ConnMgr.LowWater 30 --json
ipfs config Swarm.ConnMgr.HighWater 50 --json

ipfs config Addresses.Swarm '[
       "/ip4/0.0.0.0/tcp/4001",
       "/ip6/::/tcp/4001",
       "/ip4/0.0.0.0/tcp/0/ws",
       "/ip4/0.0.0.0/udp/4001/quic-v1",
       "/ip6/::/udp/4001/quic-v1"
]' --json

ipfs config Swarm.AddrFilters '[
       "/ip4/100.64.0.0/ipcidr/10",
       "/ip4/169.254.0.0/ipcidr/16",
       "/ip4/198.18.0.0/ipcidr/15",
       "/ip4/198.51.100.0/ipcidr/24",
       "/ip4/203.0.113.0/ipcidr/24",
       "/ip4/240.0.0.0/ipcidr/4",
       "/ip6/100::/ipcidr/64",
       "/ip6/2001:2::/ipcidr/48",
       "/ip6/2001:db8::/ipcidr/32",
       "/ip6/fc00::/ipcidr/7",
       "/ip6/fe80::/ipcidr/10"
]' --json


# force the use of s3 datastore
if [ "$IPFS_DATASTORE" = "s3" ]; then
       echo "Using s3 datastore"
       ipfs config Datastore.Spec.mounts "[
              {
                     \"child\": {
                            \"bucket\": \"$IPFS_S3_BUCKET\",
                            \"region\": \"$IPFS_S3_REGION\",
                            \"rootDirectory\": \"\",
                            \"accessKey\": \"\",
                            \"secretKey\":\"\",
                            \"type\": \"s3ds\"
                     },
                     \"mountpoint\": \"/blocks\",
                     \"prefix\": \"s3.datastore\",
                     \"type\": \"measure\"
              },
              {
                     \"child\": {
                            \"compression\": \"none\",
                            \"path\": \"datastore\",
                            \"type\": \"levelds\"
                     },
                     \"mountpoint\": \"/\",
                     \"prefix\": \"leveldb.datastore\",
                     \"type\": \"measure\"
              }
       ]" --json
       
       echo "{\"mounts\":[{\"bucket\":\"$IPFS_S3_BUCKET\",\"mountpoint\":\"/blocks\",\"region\":\"$IPFS_S3_REGION\",\"rootDirectory\":\"\"},{\"mountpoint\":\"/\",\"path\":\"datastore\",\"type\":\"levelds\"}],\"type\":\"mount\"}" > ${IPFS_PATH}/datastore_spec
       
fi

echo "Running ipfs in server mode"
ipfs config profile apply server
ipfs config AutoNAT.ServiceMode "disabled"
ipfs config Gateway.DeserializedResponses true --bool
ipfs config Gateway.RootRedirect "" 
ipfs config Gateway.NoFetch true --bool
ipfs config Gateway.NoDNSLink false --bool
ipfs config Gateway.PublicGateways '{}' --json
ipfs config Gateway.DeserializedResponses true --bool
# increase bit array to avoid collisions..
ipfs config Datastore.BloomFilterSize "1048576" --json

# required in private network
ipfs config Routing.Type "dht"
ipfs config Datastore.GCPeriod "144h"
ipfs config Datastore.StorageMax "3000GB"
ipfs config Datastore.StorageGCWatermark 99 --json
ipfs config Pubsub.Router "gossipsub"
ipfs config --json Swarm.DisableBandwidthMetrics false

# add the swarm key from env
(
    echo -e '/key/swarm/psk/1.0.0/\n/base16/'; 
    echo "$SWARM_KEY" | tr -d '\n '; echo '' 
) > ${IPFS_PATH}/swarm.key
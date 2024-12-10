#!/bin/bash
(
    echo -e '/key/swarm/psk/1.0.0/\n/base16/'; 
    echo "/n$SWARM_KEY"
) > ${IPFS_PATH}/swarm.key


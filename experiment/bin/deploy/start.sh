#!/usr/bin/env bash
./pkill.sh

start(){
    SERVER_ADDR=(`cat public_ips.txt`)
    SERVER_COUNT=${#SERVER_ADDR[@]}
    for (( j=1; j<=$1; j++))
    do
      INDEX=$(( (j-1) % SERVER_COUNT ))
      ssh -t $2@${SERVER_ADDR[INDEX]} "cd bamboo ; nohup ./run.sh ${j}"
      sleep 0.1
      echo replica ${j} is launched!
    done
}

USERNAME="root"
# MAXPEERNUM=(`wc -l public_ips.txt | awk '{ print $1 }'`)
MAXPEERNUM=60

# update config.json to replicas
start $MAXPEERNUM $USERNAME

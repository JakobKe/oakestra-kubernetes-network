#!/bin/bash

echo -e "Starting script for building and transferring the Oakestra CNI executable to all Kubernetes nodes\n"


echo "Building..."
GOOS=linux GOARCH=amd64 go build -o oakestra 


scp ./oakestra oakestra-env:/home/ubuntu/temp/oakestra
echo "Transferring..."

for node in {1..2}; do
    for cluster in {1..2}; do
        ssh oakestra-env "ssh kubernetes-${cluster}-${node} 'mkdir -p temp' >/dev/null 2>&1"
        ssh oakestra-env "scp /home/ubuntu/temp/oakestra kubernetes-${cluster}-${node}:/home/ubuntu/temp/"
        ssh oakestra-env "ssh kubernetes-${cluster}-${node} 'sudo mv /home/ubuntu/temp/oakestra /opt/cni/bin/'"
    done
done

echo "Oakestra CNI executable successfully transferred to Kubernetes cluster nodes."

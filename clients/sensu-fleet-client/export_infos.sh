#!/bin/sh

machineID=`cat /etc/machine-id`
hostname=`hostname`
osVersion=`cat /etc/os-release | grep VERSION | head -n 1 | cut -d'=' -f2`

etcdctl set /${NAMESPACE:-"machines"}/$machineID/hostname $hostname
etcdctl set /${NAMESPACE:-"machines"}/$machineID/version $osVersion

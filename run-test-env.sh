#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

echo "Script directory: $SCRIPT_DIR"

sambaDir=$SCRIPT_DIR/test/samba

run() {
    prepare_test_directory
    run_samba
}

prepare_test_directory() {
    mkdir -p $sambaDir
}

run_samba() {
    docker run -it \
        --name samba  \
        -p 139:139 \
        -p 445:445 \
        -v "$sambaDir:/mount" \
        -d dperson/samba \
        -p -n -s "public;/mount;yes;no;yes;all" -u "samba;password"
}

stop() {
    stop_samba
}

stop_samba() {
    docker stop samba
    docker rm samba
}

case "$1" in
    run)
        run
        exit 0
        ;;
    stop)
        stop
        exit 0
        ;;
    *)
        echo "Usage: $0 {run|stop}"
        exit 1
esac
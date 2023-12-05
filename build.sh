#!/usr/bin/env bash

set -e

BashDir=$(cd "$(dirname $BASH_SOURCE)" && pwd)
eval $(cat "$BashDir/script/conf.sh")
if [[ "$Command" == "" ]];then
    Command="$0"
fi

function help(){
    echo "build script"
    echo
    echo "Usage:"
    echo "  $0 [flags]"
    echo "  $0 [command]"
    echo
    echo "Available Commands:"
    echo "  help              help for $0"
    echo "  clear             clear output"
    echo "  go                go build helper"
    echo "  run               run service"
    echo "  pack              pack release"
    echo "  docker            docker build helper"
    echo
    echo "Flags:"
    echo "  -h, --help          help for $0"
}

case "$1" in
    help|-h|--help)
        help
    ;;
    run)
        cd "$BashDir/bin"
        if [[ "$BACKEND_URL" == "" ]];then
            echo "export BACKEND_URL=your_websocket_url"
            exit 1
        fi
        if [[ "$BACKEND_PASSWORD" == "" ]];then
            echo "export BACKEND_PASSWORD=your_password"
            exit 1
        fi
        ./cb -s ipv4.txt -p "$BACKEND_PASSWORD" -r "/api/v1/ws:$BACKEND_URL" -m 180 -m 280 --min 50 --max 60 -c 5
        # ./cb
        exit $?
    ;;
    clear)
        shift
        export Command="$0 clear"
        "$BashDir/script/clear.sh" "$@"
    ;;
    pack)
        shift
        export Command="$0 pack"
        "$BashDir/script/pack.sh" "$@"
    ;;
    go)
        shift
        export Command="$0 go"
        "$BashDir/script/go.sh" "$@"
    ;;
    docker)
        shift
        export Command="$0 docker"
        "$BashDir/script/docker.sh" "$@"
    ;;
    *)
        if [[ "$1" == "" ]];then
            help
        elif [[ "$1" == -* ]];then
            echo Error: unknown flag "$1" for "$0"
            echo "Run '$0 --help' for usage."
        else
            echo Error: unknown command "$1" for "$0"
            echo "Run '$0 --help' for usage."
        fi        
        exit 1
    ;;
esac
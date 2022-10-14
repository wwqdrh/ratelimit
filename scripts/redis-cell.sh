#! /bin/bash

# !!本脚本基于docker-swarm环境，若基于原生docker请进行适当修改
# not work yet

#######color code########
red="31m"      
green="32m"  
yellow="33m" 
blue="36m"
fuchsia="35m"

function color_echo(){
    echo -e "\033[$1${@:2}\033[0m"
}

function base() {
    docker pull redis:6-alpine
}

function install() {
    local file_name="redis-cell-v0.3.0-x86_64-unknown-linux-gnu.tar.gz"
    local temp_path=`mktemp -d`

    curl -L https://github.com/brandur/redis-cell/releases/download/v0.3.0/redis-cell-v0.3.0-x86_64-unknown-linux-gnu.tar.gz -o $file_name
    [[ $? != 0 ]] && { color_echo $yellow "\n下载失败!"; rm -rf $temp_path $file_name; exit 1; }

    tar -C $temp_path -xzf redis-cell-v0.3.0-x86_64-unknown-linux-gnu.tar.gz
    [[ $? != 0 ]] && { color_echo $yellow "\n解压失败!"; rm -rf $temp_path $file_name; exit 1; }

    docker service create --name redis-cell --mount source=redis-conf,target=/etc/redis redis:6-alpine

    cat > $temp_path/redis.conf <<EOF
loadmodule /etc/redis/libredis_cell.so
EOF

    docker cp $temp_path/libredis_cell.so `docker ps | grep redis-cell | awk '{print $1}'`:/etc/redis
    docker cp $temp_path/libredis_cell.d `docker ps | grep redis-cell | awk '{print $1}'`:/etc/redis
    docker cp $temp_path/redis.conf `docker ps | grep redis-cell | awk '{print $1}'`:/etc/redis

    # docker service update --args 'redis-server /etc/redis/conf' redis-cell
    docker service rm redis-cell
    docker service create --name redis-cell --mount source=redis-conf,target=/etc/redis redis:6-alpine redis-server /etc/redis/redis.conf

    rm -rf $temp_path $file_name
}

function main() {
    base
    install
}
main
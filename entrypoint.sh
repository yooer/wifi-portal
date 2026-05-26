#!/bin/sh

set -e

# 替换配置文件中的环境变量占位符
envsubst < /app/config.yaml > /app/config.yaml.tmp && mv /app/config.yaml.tmp /app/config.yaml

# 启动应用
exec ./wifi
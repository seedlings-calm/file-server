#! /bin/bash
set -e

# 创建 两个策略
echo "⏳  创建 apiUser-policy,sysUser-policy 两个策略"

mc admin policy create local apiUser-policy /scripts/apiUser-policy.json
mc admin policy create local sysUser-policy /scripts/sysUser-policy.json

echo "⏳  创建 apiUser,sysUser 两个用户"

mc admin user add local apiUser apiUser123456
mc admin user add local sysUser sysUser123456

echo "⏳  为 apiUser,sysUser 用户分别设置 apiUser-policy,sysUser-policy 两个策略"

mc admin policy attach local apiUser-policy --user apiUser
mc admin policy attach local sysUser-policy --user sysUser

echo "✅  Init Policy and user complete."

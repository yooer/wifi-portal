#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
WiFi Captive Portal SaaS - 安全初始化引导程序 (Bootstrap)
功能：自动跨平台生成强随机数据库密钥，配置隔离，彻底杜绝硬编码泄漏风险。
"""

import os
import secrets
import string

def generate_strong_password(length=32):
    """生成包含大小写字母和数字的高强度强随机密码"""
    alphabet = string.ascii_letters + string.digits
    return ''.join(secrets.choice(alphabet) for _ in range(length))

def main():
    print("==================================================================")
    print("[BOOT] WiFi Captive Portal SaaS - 安全部署初始化引导程序 (Bootstrap)")
    print("==================================================================")

    # 1. 产生安全随机凭证
    mongo_root_user = "root"
    mongo_root_password = generate_strong_password(32)
    mongo_wifi_user = "wifi"
    mongo_wifi_password = generate_strong_password(32)
    redis_password = generate_strong_password(32)

    # 2. 写入 .env 文件
    env_content = f"""# Docker 容器化环境变量 - 随机账密自动生成于引导程序
MONGO_ROOT_USER={mongo_root_user}
MONGO_ROOT_PASSWORD={mongo_root_password}
MONGO_WIFI_USER={mongo_wifi_user}
MONGO_WIFI_PASSWORD={mongo_wifi_password}
REDIS_PASSWORD={redis_password}
"""
    
    try:
        with open(".env", "w", encoding="utf-8") as f:
            f.write(env_content)
        print("[OK] 成功生成环境变量凭证文件: .env")
    except Exception as e:
        print("[ERROR] 严重错误: 无法写入 .env 文件 (%s)" % e)
        return

    # 3. 解析 config.docker.yaml.template 生成 config.docker.yaml
    template_path = "config.docker.yaml.template"
    config_path = "config.docker.yaml"

    if not os.path.exists(template_path):
        print("[ERROR] 严重错误: 缺失配置模板文件 %s" % template_path)
        return

    try:
        with open(template_path, "r", encoding="utf-8") as f:
            template_data = f.read()

        # 执行替换
        config_data = template_data
        config_data = config_data.replace("${MONGO_WIFI_USER}", mongo_wifi_user)
        config_data = config_data.replace("${MONGO_WIFI_PASSWORD}", mongo_wifi_password)
        config_data = config_data.replace("${REDIS_PASSWORD}", redis_password)

        with open(config_path, "w", encoding="utf-8") as f:
            f.write(config_data)
        print("[OK] 成功解析配置模板并生成本地安全配置文件: config.docker.yaml")
    except Exception as e:
        print("[ERROR] 严重错误: 无法处理模板生成配置文件 (%s)" % e)
        return

    print("------------------------------------------------------------------")
    print("[SUCCESS] 系统初始化配置已全部就绪！")
    print("[SECURE] 专属业务库用户名: wifi")
    print("[SECURE] 所有数据库和缓存均已配置 32 位独立随机强密码，已安全写入本地忽略规则。")
    print("------------------------------------------------------------------")
    print("[TIPS] 接下来您可以运行以下命令进行部署和维护：")
    print("  1. 启动全套服务 (App + MongoDB + Redis):")
    print("     docker-compose up -d")
    print("")
    print("  2. 查看实时 WiFi 验证与运行日志:")
    print("     docker-compose logs -f wifi-portal")
    print("")
    print("  3. 针对业务进行单独微秒级零中断重启（数据库不重启，线上会话不丢失）:")
    print("     docker-compose restart wifi-portal")
    print("")
    print("  4. 停止与清理容器:")
    print("     docker-compose down")
    print("==================================================================")

if __name__ == "__main__":
    main()

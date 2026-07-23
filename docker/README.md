# Docker 本地开发指南

## 本地 HTTPS 证书路径

本地服务入口：
```text
https://localhost:80/

```

关于 Web 证书请放在下面：

```text
docker/certs/localhost.pem
docker/certs/localhost-key.pem

```

---

## 环境配置

### Windows + WSL 环境

> **注意**：mkcert 运行在哪个环境下，那么这个环境下的浏览器才会信任这个证书，比如你在 Windows 上安装了 mkcert 并生成了证书，那么 Windows 上的 Chrome 才会信任这个证书，而 WSL 上的浏览器则不会信任。

1. 先安装 mkcert

2. **安装本地 CA 到 Windows 信任库**：
```powershell
mkcert -install
```

3. **创建证书目录**（WSL 执行）：
```bash
mkdir -p docker/certs
```


4. 获取项目的 Windows 路径（WSL 执行并复制输出内容）：
```bash
wslpath -w "$(pwd)"
```

5. 生成证书（Windows 下执行，替换 `<repo-windows-path>` 为对应路径）：
```powershell
mkcert `
  -cert-file "<repo-windows-path>\docker\certs\localhost.pem" `
  -key-file "<repo-windows-path>\docker\certs\localhost-key.pem" `
  localhost 127.0.0.1 ::1
```
---

### macOS / Linux 环境

在宿主机中执行：

```bash
mkdir -p docker/certs
mkcert \
  -cert-file docker/certs/localhost.pem \
  -key-file docker/certs/localhost-key.pem \
  localhost 127.0.0.1 ::1
```


## 启动服务

注意如果修改了镜像相关的配置，要加上 `--build` 参数重新构建镜像。

仅启动基础服务（只包含 Redis + MySQL）方便调试服务器：

```bash
docker compose -f docker/compose.infra.yaml up -d

```

启动完整环境（包含数据库和服务器）：

```bash
docker compose -f docker/compose.infra.yaml -f docker/compose.app.yaml up --build

```
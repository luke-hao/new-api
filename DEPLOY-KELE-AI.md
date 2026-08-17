# 可乐AI本地部署

## 前置条件

当前源码已改为本地构建镜像，运行前需要先安装 Docker Desktop for Windows，并确保 PowerShell 中可以执行：

```powershell
docker --version
docker compose version
```

如果 `docker` 命令不可用，请安装 Docker Desktop、启动它，并重新打开 PowerShell。

## 启动

```powershell
Set-Location 'D:\Users\yfww\Desktop\new-api\new-api-main'
docker compose up -d --build
```

访问：

```text
http://localhost:3000
```

## 验证

```powershell
(Invoke-RestMethod http://localhost:3000/api/status).data | Select-Object system_name, logo
```

期望结果：

```text
system_name : 可乐AI
logo        : /logo.png
```

## 如果旧数据库覆盖了品牌

如果之前已经启动过旧版本，数据库里的 `options` 可能覆盖源码默认值。执行：

```powershell
docker compose exec postgres psql -U root -d new-api -c "INSERT INTO options (key, value) VALUES ('SystemName', '可乐AI') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;"
docker compose exec postgres psql -U root -d new-api -c "INSERT INTO options (key, value) VALUES ('Logo', '/logo.png') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;"
docker compose restart new-api
```

然后清理浏览器中 `localhost:3000` 的 localStorage 后刷新页面。

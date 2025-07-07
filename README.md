# proxy-subscription

proxy-subscription 是一个为 NekoRay 提供的网页配置界面，让用户能够通过浏览器轻松管理 NekoRay 的配置。本项目采用前后端分离架构，前端使用 Vue.js，后端使用 Go 语言开发。

## 功能特点

- 通过网页界面管理多个订阅配置
- 自动更新订阅节点信息
- 导入导出订阅配置
- 支持多种 NekoRay 配置文件格式
- 响应式设计，支持移动端和桌面端
- RESTful API 接口，便于二次开发

## 安装与部署


### 从源码构建

#### 前提条件

- 已安装 Node.js (v16+)
- 已安装 Go (v1.20+)
- 已安装 Git

#### 构建步骤

1. 克隆仓库

```bash
git clone https://github.com/bernard-xu/proxy-subscription.git
cd proxy-subscription
```

2. 在 Windows 上构建

```bash
.\build.bat
```

3. 在 Linux/macOS 上构建

```bash
chmod +x ./build.sh
./build.sh
```

构建完成后，可执行文件将位于 `dist` 目录中。

## 配置说明

### 环境变量

| 环境变量 | 描述 | 默认值 |
|---------|------|-------|
| `PORT` | 服务监听端口 | `8080` |
| `DATA_DIR` | 数据存储目录 | `./data` |
| `GIN_MODE` | Gin 框架运行模式 | `release` |

### 数据目录

应用数据存储在 `data` 目录中，包括：

- 数据库文件：`nekoray-config.db`
- 配置文件：`config.json`
- 日志文件：`logs/`

## 使用方法

1. 启动应用后，访问 `http://localhost:8080` 打开 Web 界面
2. 添加订阅：点击「添加订阅」按钮，输入订阅地址和相关信息
3. 刷新订阅：点击订阅列表中的「刷新」按钮更新节点信息
4. 导出配置：点击「导出」按钮将配置导出为 NekoRay 可用的格式

### 自定义主机和端口

可以在启动二进制文件时通过命令行参数指定主机地址和端口：

```bash
# Windows
proxy-subscription.exe --host 0.0.0.0 --port 8000

# Linux/macOS
./proxy-subscription --host 0.0.0.0 --port 8000
```

参数说明：
- `--host`: 指定服务器监听的主机地址，默认为 `localhost`。使用 `0.0.0.0` 可以监听所有网络接口
- `--port`: 指定服务器监听的端口，默认为 `8080`

## API 文档

### 订阅管理 API

- `GET /api/subscriptions` - 获取所有订阅
- `POST /api/subscriptions` - 添加新订阅
- `PUT /api/subscriptions/:id` - 更新订阅
- `DELETE /api/subscriptions/:id` - 删除订阅
- `POST /api/subscriptions/:id/refresh` - 刷新订阅

## 常见问题

### Q: 如何更新到最新版本？

**二进制方式**：下载最新版本并替换原有可执行文件。

### Q: 数据如何备份？

只需备份 `data` 目录即可保存所有配置和数据。

## 贡献指南

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件
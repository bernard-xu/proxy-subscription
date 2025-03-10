# 前端构建阶段
FROM node:20-alpine AS frontend-builder
WORKDIR /app
# 只复制package文件以利用缓存
COPY frontend/package*.json ./frontend/
RUN cd frontend && npm install
# 复制前端源代码并构建
COPY frontend/ ./frontend/
RUN cd frontend && npm run build

# Go构建阶段
FROM golang:1.23-alpine AS go-builder
WORKDIR /app
# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist/
# 先复制go.mod和go.sum以利用缓存
COPY backend/go.mod backend/go.sum ./
RUN go mod download
# 复制后端源代码
COPY backend/ ./
# 使用静态编译，禁用CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o proxy-subscription .

# 最终运行阶段 - 使用更小的基础镜像
FROM alpine:3.19
# 安装CA证书以支持HTTPS
RUN apk --no-cache add ca-certificates tzdata
# 设置工作目录
WORKDIR /app
# 创建数据目录并设置权限
RUN mkdir -p /app/data && chmod 755 /app/data
# 只复制二进制文件
COPY --from=go-builder /app/proxy-subscription /app/
# 设置环境变量
ENV GIN_MODE=release
# 声明数据卷
VOLUME ["/app/data"]
# 暴露端口
EXPOSE 8080
# 启动应用
CMD ["/app/proxy-subscription"]
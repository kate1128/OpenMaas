# Nginx 选型与用法

**版本要求：** ≥ 1.25（含 HTTP/3 QUIC 支持）  
**角色：** 反向代理、TLS 终止、入口限流、静态文件服务

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| 高并发 | 事件驱动架构，单进程支持数万并发连接 |
| TLS 终止 | 统一管理证书，内网服务无需独立 TLS |
| 限流 | `limit_req_zone` 实现 IP/Key 级别限流，gateway-service 的第一道防线 |
| 健康检查 | `upstream` 模块支持主动/被动健康检查 |
| HTTP/2 + HTTP/3 | 减少连接建立开销，降低 TTFB |

---

## 二、架构角色

```
Internet
    │ HTTPS :443 / HTTP/3 :443
    ▼
Nginx（2副本，L4 LB 前置）
    ├─ /api/v1/        → gateway-service:8080
    ├─ /console/       → console-frontend (静态)
    ├─ /admin/         → admin-frontend (静态)
    └─ /docs/          → docs-site (静态)
```

---

## 三、核心配置（nginx.conf）

```nginx
user nginx;
worker_processes auto;
worker_rlimit_nofile 65535;

events {
    worker_connections 4096;
    use epoll;
    multi_accept on;
}

http {
    # 基础安全头
    add_header X-Frame-Options           "DENY"            always;
    add_header X-Content-Type-Options    "nosniff"         always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-XSS-Protection          "1; mode=block"   always;
    server_tokens off;

    # 限流区（IP 级别）
    limit_req_zone $binary_remote_addr zone=ip_limit:20m rate=100r/s;
    # 限流区（API Key 级别，从 Header 提取）
    limit_req_zone $http_authorization zone=key_limit:50m rate=500r/s;

    # 上游服务
    upstream gateway_service {
        least_conn;
        server gateway-service-0:8080 max_fails=3 fail_timeout=10s;
        server gateway-service-1:8080 max_fails=3 fail_timeout=10s;
        keepalive 32;
    }

    server {
        listen 443 ssl;
        listen 443 quic reuseport;   # HTTP/3
        http2 on;
        server_name api.maas.example.com;

        ssl_certificate     /etc/nginx/certs/tls.crt;
        ssl_certificate_key /etc/nginx/certs/tls.key;
        ssl_protocols       TLSv1.2 TLSv1.3;
        ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
        ssl_session_cache   shared:SSL:10m;
        ssl_session_timeout 10m;

        # API 反向代理
        location /api/ {
            limit_req zone=ip_limit burst=200 nodelay;
            limit_req zone=key_limit burst=1000 nodelay;

            proxy_pass         http://gateway_service;
            proxy_http_version 1.1;
            proxy_set_header   Connection      "";
            proxy_set_header   Host            $host;
            proxy_set_header   X-Real-IP       $remote_addr;
            proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header   X-Request-ID    $request_id;

            # SSE 流式响应支持
            proxy_buffering    off;
            proxy_cache        off;
            proxy_read_timeout 300s;

            # 健康检查路径不限流
            location /api/health {
                limit_req off;
                proxy_pass http://gateway_service;
            }
        }

        # 静态资源
        location /console/ {
            root /usr/share/nginx/html;
            try_files $uri $uri/ /console/index.html;
            expires 1d;
            add_header Cache-Control "public, immutable";
        }
    }

    # HTTP 强制跳转 HTTPS
    server {
        listen 80;
        return 301 https://$host$request_uri;
    }
}
```

---

## 四、SSE（Server-Sent Events）代理要点

流式 LLM 响应必须关闭缓冲，否则客户端会等到响应结束才收到数据：

```nginx
location /api/v1/chat/ {
    proxy_pass          http://gateway_service;
    proxy_buffering     off;        # 关闭响应缓冲
    proxy_cache         off;
    proxy_read_timeout  300s;       # 流式响应最长 5 分钟
    chunked_transfer_encoding on;
    add_header          X-Accel-Buffering "no";
}
```

---

## 五、限流状态码规范

| 场景 | HTTP 状态码 | 响应体 |
|------|------------|-------|
| IP 超出限流 | 429 | `{"code":"RATE_LIMITED","message":"Too Many Requests"}` |
| API Key 超出配额 | 429 | `{"code":"QUOTA_EXCEEDED","message":"API key quota exceeded"}` |
| 服务不可用 | 503 | `{"code":"SERVICE_UNAVAILABLE","message":"..."}` |

---

## 六、证书管理

生产环境使用 cert-manager（K8s）自动签发/续期 Let's Encrypt 证书：

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: maas-api-tls
spec:
  secretName: maas-api-tls-secret
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - api.maas.example.com
```

---

## 七、监控指标（nginx-prometheus-exporter）

| 指标 | 告警阈值 |
|------|---------|
| `nginx_http_requests_total` 增速 | 突增 5x 触发 P1 |
| `nginx_connections_active` | > 8000（单实例）触发 P1 |
| `nginx_http_4xx_rate` | > 10% 触发 P1 |
| `nginx_http_5xx_rate` | > 1% 触发 P0 |

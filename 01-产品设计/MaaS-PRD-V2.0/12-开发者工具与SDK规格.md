# MaaS平台 PRD V2.0 —— 12 开发者工具与SDK规格

**文档版本：** V2.0.0（框架草稿）  
**编写日期：** 2026年05月22日  
**文档状态：** 框架草稿（待详细规格补充，V2.1 完整化）  
**机密等级：** 内部保密  
**所属模块：** 开发者生态（Developer Ecosystem）  
**对应 GAP：** GAP-11（CLI工具）、GAP-26（CLI工具）、GAP-27（Terraform Provider / Helm Chart）  
**关联文档：** `00-总纲与导航.md` §8 开发者生态、`09-Console控制台功能规格.md`

---

## 目录

1. [设计原则与背景](#第1章-设计原则与背景)
2. [OpenAPI 规范](#第2章-openapi-规范)
3. [多语言 SDK](#第3章-多语言-sdk)
4. [CLI 工具（maas-cli）](#第4章-cli-工具maas-cli)
5. [本地调试代理（maas-proxy）](#第5章-本地调试代理maas-proxy)
6. [Terraform Provider](#第6章-terraform-provider)
7. [Helm Chart](#第7章-helm-chart)
8. [开发者门户规格](#第8章-开发者门户规格)
9. [验收标准](#第9章-验收标准)

---

## 第1章 设计原则与背景

### 1.1 为什么开发者工具是竞争关键

企业 AI 平台的竞争，在技术采购环节已不仅仅是功能与价格的比拼，开发者体验（Developer Experience，DX）已成为越来越关键的采购权重维度。以下两类情形最为典型：

**情形一：工程师先用再推**  
企业内部的 AI 基础设施选型，通常由一线工程师完成 POC，再向管理层推荐。如果 MaaS 平台对工程师不友好——没有 CLI、没有本地调试、SDK 文档残缺——则工程师倾向于推荐 LiteLLM（有完整 CLI）或 Portkey（有本地代理），而非 MaaS。

**情形二：Infrastructure as Code 是企业 IT 标配**  
中大型企业的 IT 治理中，"配置即代码"（Configuration as Code）已是标准实践。如果 MaaS 平台的租户配置、路由策略、预算管理只能通过 Console 手动操作，无法通过 Terraform 管理、无法纳入 GitOps 流程，则在企业 IT 团队的平台评估中将显著失分。

### 1.2 V2.0 开发者工具交付范围

| 工具 | V2.0 状态 | V2.1 目标 |
|------|----------|----------|
| OpenAPI 规范 | ✅ 完整发布（随 API 服务同步生成） | 自动化接口测试集成 |
| Python SDK | ✅ P0，与 MVP 同步交付 | 异步支持、流式增强 |
| Node.js SDK | ✅ P0，与 MVP 同步交付 | Bun/Deno 适配 |
| Go SDK | ⚠️ P1，Beta 阶段交付 | 并发池管理 |
| Java SDK | 🔄 P2，GA 阶段交付 | Spring Boot Starter |
| maas-cli | ⚠️ P1，Beta 阶段发布 v0.1 | 插件系统 |
| maas-proxy（本地调试代理） | ✅ P0，与 MVP 同步交付 | UI 面板 |
| Terraform Provider | 🔄 P2，GA 阶段发布 | 完整资源覆盖 |
| Helm Chart | 🔄 P2，私有化交付配套 | ArgoCD Application 模板 |

---

## 第2章 OpenAPI 规范

### 2.1 规范标准与生成方式

MaaS 平台 API 遵循 **OpenAPI 3.1.0** 规范，采用"代码即文档"（Code-First）策略：API 注解直接生成 OpenAPI YAML，避免文档与实现脱节。

**规范文件发布地址**：
- 生产环境：`https://api.maas-platform.com/openapi.yaml`
- 沙箱环境：`https://sandbox.maas-platform.com/openapi.yaml`
- 私有化部署：`https://<your-domain>/openapi.yaml`

### 2.2 API 分组结构

| Tag 分组 | 说明 | 认证要求 |
|---------|------|---------|
| `inference` | LLM 推理调用（OpenAI兼容 + 原生） | API Key |
| `models` | 模型目录查询 | API Key |
| `usage` | Token 用量与成本查询 | API Key |
| `traces` | Trace 查询与 Span 上报 | API Key |
| `budgets` | 预算查询（只读） | API Key |
| `admin/tenants` | 租户管理 | JWT（平台管理员） |
| `admin/models` | 模型目录管理 | JWT（Model Curator） |
| `admin/routing` | 路由策略管理 | JWT（Tenant Admin+） |
| `admin/compliance` | 合规策略管理 | JWT（Security Officer+） |
| `webhooks` | Webhook 订阅管理 | JWT |

### 2.3 OpenAI 兼容性要求

推理接口必须完整兼容 OpenAI API v1，包含以下端点：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | Chat 补全（支持流式） |
| `/v1/completions` | POST | 文本补全（兼容老版本） |
| `/v1/embeddings` | POST | Embedding 向量化 |
| `/v1/models` | GET | 列出可用模型 |
| `/v1/models/{model}` | GET | 查询单个模型信息 |
| `/v1/images/generations` | POST | 图像生成（依赖供应商支持） |
| `/v1/audio/transcriptions` | POST | 语音转文字 |

**兼容性验证**：使用 OpenAI Python SDK 连接 MaaS 网关时，仅需修改 `base_url` 和 `api_key`，无需其他代码变更。此项为 P0 验收项。

---

## 第3章 多语言 SDK

### 3.1 SDK 设计原则

**原则一：OpenAI 兼容层优先**  
SDK 第一层封装是 OpenAI 兼容调用，确保已使用 OpenAI SDK 的项目可以最小代价迁移（仅替换 base_url 和 api_key）。

**原则二：MaaS 增强层可选**  
SDK 第二层封装是 MaaS 专有增强能力（Trace 上报、Agent Run 追踪、预算查询、路由策略管理），以独立模块提供，不强制引入。

**原则三：零魔改依赖**  
SDK 不 monkey-patch 第三方库（如不修改 httpx、requests 的全局行为），确保与项目中其他 HTTP 客户端无冲突。

### 3.2 Python SDK

**包名**：`maas-platform`  
**发布渠道**：PyPI  
**最低 Python 版本**：3.9+  
**依赖**：`httpx`（>=0.24）、`pydantic`（>=2.0）

**核心接口示例**：

```python
from maas import MaaSClient, TraceClient

# OpenAI 兼容调用（最小迁移成本）
from openai import OpenAI
client = OpenAI(
    api_key="proj_your_key",
    base_url="https://api.maas-platform.com/v1"
)
response = client.chat.completions.create(
    model="maas:gpt-4o",
    messages=[{"role": "user", "content": "你好"}]
)

# MaaS 原生客户端（含增强能力）
maas = MaaSClient(api_key="proj_your_key")

# 查询 Token 用量
usage = maas.usage.get(start="2026-05-01", end="2026-05-21", group_by="model")

# 查询预算余额
budget = maas.budgets.get_remaining(project_id="proj_xxx")

# Agent Trace 上报
with maas.trace.agent_run(session_id="sess_abc") as run:
    with run.step("检索知识库", span_type="RAG_RETRIEVAL") as step:
        docs = kb.search(query)
        step.set_retrieval_meta(doc_count=len(docs))
```

**流式响应示例**：

```python
stream = client.chat.completions.create(
    model="maas:gpt-4o",
    messages=[{"role": "user", "content": "写一首诗"}],
    stream=True
)
for chunk in stream:
    print(chunk.choices[0].delta.content, end="", flush=True)
```

### 3.3 Node.js / TypeScript SDK

**包名**：`@maas-platform/sdk`  
**发布渠道**：npm  
**最低 Node.js 版本**：18+  
**支持**：CommonJS + ESM + TypeScript 类型声明

**核心接口示例**：

```typescript
import MaaS from '@maas-platform/sdk';

const maas = new MaaS({ apiKey: 'proj_your_key' });

// OpenAI 兼容调用
const response = await maas.chat.completions.create({
  model: 'maas:gpt-4o',
  messages: [{ role: 'user', content: '你好' }],
});

// 流式调用
const stream = await maas.chat.completions.create({
  model: 'maas:gpt-4o',
  messages: [{ role: 'user', content: '写一首诗' }],
  stream: true,
});
for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? '');
}

// 查询用量
const usage = await maas.usage.list({ startDate: '2026-05-01', groupBy: 'model' });
```

### 3.4 Go SDK

**包路径**：`github.com/maas-platform/maas-go`  
**发布渠道**：Go Modules  
**最低 Go 版本**：1.21+

**核心接口示例**：

```go
import "github.com/maas-platform/maas-go"

client := maas.NewClient("proj_your_key",
    maas.WithBaseURL("https://api.maas-platform.com/v1"),
)

resp, err := client.Chat.Completions.Create(ctx, &maas.ChatCompletionRequest{
    Model: "maas:gpt-4o",
    Messages: []maas.Message{
        {Role: "user", Content: "你好"},
    },
})
```

---

## 第4章 CLI 工具（maas-cli）

### 4.1 CLI 定位

`maas-cli` 是面向平台管理员和开发者的命令行工具，支持：
- 模型、路由策略、API Key、预算等资源的增删改查
- Trace 日志实时查看（`tail -f` 风格）
- 配置文件管理（多 Profile 支持，方便在多环境间切换）
- 与 CI/CD 管道集成（非交互模式 + JSON 输出）

### 4.2 安装方式

```bash
# macOS（Homebrew）
brew install maas-platform/tap/maas-cli

# Linux（Shell 安装脚本）
curl -fsSL https://cli.maas-platform.com/install.sh | sh

# Windows（Scoop）
scoop bucket add maas https://github.com/maas-platform/scoop-bucket
scoop install maas-cli

# 通用（Go Install）
go install github.com/maas-platform/maas-cli@latest
```

### 4.3 核心命令集

#### 认证与配置

```bash
# 登录（交互式，打开浏览器完成 SSO 或输入 API Key）
maas auth login

# 查看当前登录状态
maas auth status

# 配置多环境 Profile
maas config set --profile prod --api-key proj_xxx --base-url https://api.maas-platform.com
maas config set --profile staging --api-key proj_yyy --base-url https://staging.maas-platform.com

# 切换 Profile
maas config use prod
```

#### 模型管理

```bash
# 列出所有可用模型（支持过滤）
maas models list
maas models list --provider openai --capability vision
maas models list --status active --output json

# 查看模型详情
maas models get maas:gpt-4o

# 对比两个模型（输出能力/价格/合规差异表格）
maas models diff maas:gpt-4o maas:claude-3-5-sonnet
```

#### API Key 管理

```bash
# 列出项目 Key
maas keys list --project proj_xxx

# 创建新 Key
maas keys create --project proj_xxx --name "CI/CD Key" --scope "inference:read"

# 吊销 Key（需确认）
maas keys revoke key_yyy --confirm

# 批量吊销（危险操作，需输入项目名称确认）
maas keys revoke-all --project proj_xxx
```

#### 路由策略

```bash
# 列出路由策略
maas routing policies list --scope tenant

# 查看策略详情
maas routing policies get policy_id

# 应用策略配置文件（GitOps 场景）
maas routing policies apply -f routing-policy.yaml

# 仿真策略（用历史流量测试新策略，返回预期效果报告）
maas routing policies simulate policy_id --traffic-window 7d
```

#### Trace 实时查看

```bash
# 实时流式查看调用日志（类似 tail -f）
maas trace tail

# 按项目过滤
maas trace tail --project proj_xxx

# 只看失败请求
maas trace tail --status error

# 查询历史 Trace
maas trace get trace_id

# 查询 Session 详情
maas trace session sess_id
```

#### 预算与成本

```bash
# 查看预算余额
maas budget status --project proj_xxx

# 查看成本概览（按模型分解）
maas cost summary --period 2026-05 --group-by model

# 导出账单（CSV）
maas billing export --period 2026-05 --format csv --output ./invoice-may.csv
```

### 4.4 CI/CD 集成模式

在 CI/CD 管道中，`maas-cli` 支持以下非交互模式：

```bash
# 通过环境变量传入认证（不写入配置文件）
export MAAS_API_KEY=proj_xxx
export MAAS_BASE_URL=https://api.maas-platform.com

# JSON 输出模式（方便 jq 解析）
maas models list --output json | jq '.[] | select(.status=="active") | .model_id'

# 静默模式（仅返回退出码，0=成功，非0=失败）
maas keys revoke key_yyy --confirm --quiet
echo $?  # 0 表示吊销成功
```

---

## 第5章 本地调试代理（maas-proxy）

### 5.1 定位

`maas-proxy` 是一个本地运行的轻量级代理服务，解决开发者在本地开发阶段的两类痛点：

**痛点一：本地代码需要指向生产/测试环境**  
开发者在本地测试时，需要在代码中硬编码 `base_url` 或配置 `.env` 文件，且每次切换环境都要修改。`maas-proxy` 提供统一的本地端点 `http://localhost:8080`，代理到当前激活的 Profile 所指向的环境。

**痛点二：调用链路不透明**  
开发者在本地无法直接看到路由决策过程（选了哪个后端、为什么选它）、Token 消耗、延迟明细。`maas-proxy` 在本地提供一个轻量级 Trace 面板，实时展示每次调用的完整链路信息。

### 5.2 启动方式

```bash
# 启动代理（监听 localhost:8080，转发到当前 Profile 的环境）
maas proxy start

# 指定端口
maas proxy start --port 8081

# 带 UI 面板启动（自动在浏览器打开 http://localhost:8090）
maas proxy start --ui

# 后台运行
maas proxy start --daemon
maas proxy stop
```

### 5.3 代理功能规格

| 功能 | 说明 |
|------|------|
| 请求转发 | 完整转发 HTTP 请求（含 Headers、Body、流式响应） |
| 本地 Trace 记录 | 每次请求写入本地 SQLite，保留最近 1000 条 |
| 路由解释展示 | 在响应 Header 中追加 `X-MaaS-Route-Debug` 字段，包含路由决策 JSON |
| 延迟注入（测试用）| `--latency 200ms` 模拟额外延迟，测试 fallback 场景 |
| 离线模式 | `--mock` 模式下不发起真实请求，返回预配置的 mock 响应，供纯离线开发使用 |
| 本地 UI 面板 | 展示最近调用列表、Trace 详情、Token 消耗统计（类似 Wireshark 的 HTTP 分析） |

---

## 第6章 Terraform Provider

### 6.1 Provider 信息

**Provider 名称**：`maas-platform/maas`  
**Terraform Registry**：`registry.terraform.io/maas-platform/maas`  
**最低 Terraform 版本**：1.5+

### 6.2 支持的资源类型（V2.0 目标范围）

| 资源类型 | Terraform 资源名 | 说明 |
|---------|----------------|------|
| 项目 | `maas_project` | 创建/更新/删除项目 |
| API Key | `maas_api_key` | 创建/更新 API Key（不支持读取明文，敏感值通过 sensitive 标记） |
| 路由策略 | `maas_routing_policy` | 路由策略 CRUD（支持 YAML 内联或文件引用） |
| 预算 | `maas_budget` | 项目级预算设置 |
| 成员角色绑定 | `maas_member_role` | 将用户绑定到项目角色 |
| 合规策略 | `maas_compliance_policy` | 合规策略 CRUD |

### 6.3 使用示例

```hcl
terraform {
  required_providers {
    maas = {
      source  = "maas-platform/maas"
      version = "~> 1.0"
    }
  }
}

provider "maas" {
  api_key  = var.maas_api_key
  base_url = "https://api.maas-platform.com"
}

# 创建项目
resource "maas_project" "analytics" {
  name        = "数据分析团队"
  description = "BI 和数据分析业务线 AI 调用项目"
  tenant_id   = var.tenant_id
}

# 创建项目 API Key
resource "maas_api_key" "analytics_key" {
  project_id = maas_project.analytics.id
  name       = "Analytics Pipeline Key"
  scopes     = ["inference:create", "usage:read"]
}

# 配置月度预算
resource "maas_budget" "analytics_budget" {
  project_id    = maas_project.analytics.id
  period        = "monthly"
  limit_usd     = 500
  alert_at_pct  = 80
  hard_stop     = true
}

# 路由策略
resource "maas_routing_policy" "analytics_routing" {
  project_id   = maas_project.analytics.id
  name         = "成本优先策略"
  policy_type  = "MULTI_OBJECTIVE"
  weights = {
    quality   = 0.20
    cost      = 0.50
    latency   = 0.15
    stability = 0.15
  }
}

# 输出 Key ID（注意：API Key 明文不会输出，通过 Console 或 CLI 获取）
output "analytics_key_id" {
  value = maas_api_key.analytics_key.id
}
```

---

## 第7章 Helm Chart

### 7.1 Chart 信息

**Chart 名称**：`maas-platform`  
**Helm Repository**：`https://charts.maas-platform.com`  
**适用场景**：私有化部署、混合云场景（详细规格见 `08-私有化交付与混合云规格.md`）

### 7.2 Chart 结构概览

```
maas-platform/
├── Chart.yaml
├── values.yaml          # 可覆盖的配置项
├── values.schema.json   # 配置项 JSON Schema 验证
├── templates/
│   ├── gateway/         # API 网关 Deployment + Service + HPA
│   ├── routing-engine/  # 路由引擎 Deployment
│   ├── console/         # Console 前端 Deployment + Ingress
│   ├── billing/         # 计费服务 Deployment
│   ├── trace/           # Trace 采集服务 Deployment
│   ├── postgres/        # PostgreSQL StatefulSet（可选，也可外接）
│   ├── redis/           # Redis StatefulSet（可选，也可外接）
│   └── _helpers.tpl     # 公共模板函数
└── charts/              # 子 Chart 依赖（PostgreSQL、Redis）
```

### 7.3 关键配置项（values.yaml 节选）

```yaml
global:
  imageRegistry: ""          # 私有镜像仓库地址（离线部署必填）
  storageClass: "standard"   # PVC 存储类

gateway:
  replicas: 3
  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "2Gi"
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70

database:
  external: false             # true = 使用外部 PostgreSQL
  host: ""                    # external=true 时必填
  port: 5432
  name: "maas"
  username: "maas"
  passwordSecret: "maas-db-secret"  # K8s Secret 名称

compliance:
  dataResidency:
    enabled: false
    region: "cn-north"        # 强制所有请求路由到指定地域
  zeroDataRetention:
    enabled: false            # 启用后不持久化 Prompt/Response 原文
```

---

## 第8章 开发者门户规格

### 8.1 开发者门户（Developer Portal）

**地址**：`https://developers.maas-platform.com`（公有云）/ `https://<your-domain>/docs`（私有化）

**核心功能**：

| 模块 | 内容 |
|------|------|
| 快速开始 | 5分钟接入指南（含代码复制即用的示例） |
| API 参考 | 交互式 OpenAPI 文档（Swagger UI / Redoc），支持在线调试 |
| SDK 文档 | Python / Node.js / Go SDK 完整 API 文档，含代码示例 |
| CLI 参考 | `maas-cli` 完整命令手册，含参数说明和示例 |
| 迁移指南 | 从 OpenAI SDK / LiteLLM / One API 迁移到 MaaS 的步骤说明 |
| 变更日志 | API / SDK / CLI 版本变更历史，含 Breaking Change 标注 |
| 社区与支持 | 企业客户工单入口、GitHub Issues（开源 SDK）、Discord 社区 |

### 8.2 交互式 Playground（开发者门户内嵌）

开发者门户内嵌轻量级 Playground，无需登录 Console 即可体验（使用沙箱 Token，有调用次数限制）：
- 选择模型（仅展示公开可用模型）
- 输入 Prompt 并发送
- 查看响应和路由解释（简化版）
- 查看本次调用的 Token 消耗

---

## 第9章 验收标准

| # | 验收项 | 验收口径 |
|---|--------|---------|
| DT-01 | OpenAI SDK 兼容性 | 使用官方 OpenAI Python SDK，仅修改 `base_url` 和 `api_key`，发起 Chat Completion 请求成功，响应格式与 OpenAI 原始响应一致 |
| DT-02 | Python SDK 安装 | `pip install maas-platform` 成功安装，`import maas` 无报错，Python 3.9/3.10/3.11/3.12 四个版本均通过 |
| DT-03 | Node.js SDK 安装 | `npm install @maas-platform/sdk` 成功安装，TypeScript 项目中类型推导正常，Node.js 18/20/22 三个版本均通过 |
| DT-04 | CLI 安装（macOS） | `brew install maas-platform/tap/maas-cli` 成功，`maas --version` 输出版本号 |
| DT-05 | CLI 模型列表 | `maas models list --output json` 返回有效 JSON 数组，字段包含 `model_id`、`display_name`、`status` |
| DT-06 | CLI Trace 实时查看 | `maas trace tail` 启动后，发起一次 API 调用，在 ≤ 3 秒内看到该调用的 Trace 记录输出 |
| DT-07 | 本地代理启动 | `maas proxy start` 启动后，向 `http://localhost:8080/v1/chat/completions` 发起请求，代理成功转发并返回响应 |
| DT-08 | Terraform Provider | 使用示例 HCL 配置执行 `terraform apply`，成功在 MaaS 平台创建项目、API Key 和预算，`terraform destroy` 成功清理 |
| DT-09 | Helm Chart 部署 | 在 Kubernetes 1.28+ 集群执行 `helm install maas maas-platform/maas-platform`，所有 Pod 进入 Running 状态，`/health` 端点返回 200 |
| DT-10 | 开发者门户 Playground | 无需登录访问开发者门户，在 Playground 中发送请求，≤ 5 秒内收到响应，Token 消耗正确展示 |

---

*本文档为 V2.0 框架草稿，第 4-7 章的详细规格（参数表、错误码、资源字段完整定义）将在 V2.1 迭代中补充完整。*  
*下一文档：（无，本文档为最后一号子文档）*

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Nature

OpenMaas is a **pure documentation and prototyping repository** for an enterprise MaaS (Model as a Service) platform — a multi-tenant, multi-vendor AI model aggregation and operations control plane. There is no executable source code, build system, test suite, or CI/CD. All content is in **Chinese (Simplified)**, except this file and some code snippets.

The only non-markdown files are the 4 HTML prototypes in `04-原型设计/` (Tailwind CSS + Font Awesome, single-file, no build step — open directly in a browser).

## Directory Map

```
01-产品设计/
  └── MaaS-PRD-V2.0/           ← 12 authoritative PRD specs (V2.0, ~200K words total)
      00-总纲与导航.md           Architecture diagram, capability roadmap (MVP→Beta→GA→Enterprise)
      01-产品定位与用户角色体系.md  19-role RBAC, approval workflow state machine, SSO/SCIM, 5 API key types
      02-模型目录与供应商治理规格.md 3-layer model (ProviderModel→VendorBackend→LogicalModel), 50+ field model card
      03-路由策略与容灾降级规格.md  4D scoring, 5 policy types, fallback chains, simulation engine, A/B linkage
      04-LLMOps观测与请求Trace规格.md 44-field Trace, Session view, cost attribution, anomaly clustering
      05-Prompt实验与模型评测中心规格.md Prompt versioning, A/B testing, LLM-as-Judge, quality gate, Pareto analysis
      06-计费成本与FinOps规格.md   3-priority metering, 4-layer budget, anomaly detection, saving advisor
      07-合规安全与审计规格.md     L0-L4 data classification, Guardrails, ZDR, BYOK, audit hash chain
      08-私有化交付与混合云规格.md   SaaS/Dedicated/Appliance, hybrid cloud, licensing, domestic adaptation
      09-Console控制台功能规格.md   13 chapters of tenant-side console UI spec, URL scheme, permission matrix
      10-Admin平台管理后台规格.md   15 modules of admin backend UI spec, role-menu matrix

02-竞品分析/                    28 competitive analysis reports (cloud vendors + open-source gateways)
03-供应商分析/                   AI model supplier evaluations (OpenAI, Anthropic, DeepSeek, Qwen, Llama...)
04-原型设计/
  ├── console-frontend-prototype.html   Tenant dev console (6600+ lines, 25 sections)
  ├── admin-frontend-prototype.html     Platform admin backend (4800+ lines, 17 sections)
  ├── public-site-prototype.html        Public-facing site
  ├── docs-site-example.html            Documentation site example
  ├── console-原型设计线稿.md           Console wireframe notes
  ├── admin-原型设计线稿.md             Admin wireframe notes
  └── 共享规范文档.md                   Shared design tokens

05-开发设计/
  ├── 01-后端设计/
  │   ├── 架构全景图.md                  Architecture overview with 12 Mermaid diagrams
  │   ├── 技术架构设计文档.md            Technical architecture (V2.0, aligned with PRD V2.0)
  │   ├── 系统设计文档(HLD).md          High-Level Design (V2.0)
  │   ├── API接口设计文档.md            API design — OpenAI/Anthropic/Gemini multi-protocol
  │   ├── 数据库设计文档.md              Database design — 6 schemas, ClickHouse OLAP
  │   ├── 代码规范文档.md               Coding standards (Go + Python + TypeScript)
  │   ├── 需求追溯矩阵.md               Requirements traceability matrix
  │   ├── SLA服务等级协议.md             SLA targets per service
  │   ├── API版本管理策略.md             API versioning strategy
  │   ├── 厂商信息导入模板.md            Vendor import template (5-sheet Excel)
  │   └── 微服务设计/                   10 microservice detailed design docs
  │       ├── 00-微服务设计审核报告(V2.0).md
  │       ├── 01-gateway-service详细设计.md       Go, 11-step middleware chain, multi-protocol
  │       ├── 02-routing-service详细设计.md       Go, 4D scoring, fallback L1-L5, A/B + quality gate linkage
  │       ├── 03-model-catalog-service详细设计.md  Go, 3-layer model, 50+ fields, vendor governance
  │       ├── 04-adapter-service详细设计.md        Python 3.12 + LiteLLM, Prompt Cache, semantic cache
  │       ├── 05-billing-service详细设计.md        Go, 3 anomaly detection algorithms, saving advisor
  │       ├── 06-llmops-trace-service详细设计.md   Go, 44-field Trace, ClickHouse, Session view
  │       ├── 07-prompt-eval-service详细设计.md    Go+Python, A/B test, LLM-as-Judge, quality gate
  │       ├── 08-auth-service详细设计.md           Go, 19-role RBAC, break-glass, approval 12-state
  │       ├── 09-compliance-service详细设计.md     Go+Python, Guardrails, ZDR, KMS, audit hash chain
  │       └── 10-notification-service详细设计.md   Go, multi-channel, dedup, escalation
  └── 02-前端设计/                  Frontend architecture for Console + Admin

06-产品运维/                     Security, compliance, DR, runbooks, capacity planning, deployment topology
07-中间件/                       Middleware deep-dives (PostgreSQL, Redis, Kafka, ClickHouse, Qdrant, MinIO...)
08-源码走读/                     Source walkthroughs of open-source competitors
  ├── litellm-源码走读分析报告.md      LiteLLM mainstream (Python SDK + Proxy)
  ├── bifrost-源码走读分析报告.md      Bifrost (Go AI Gateway, plugin architecture)
  └── routero-develop源码走读调研报告.md LiteLLM fork analysis
09-cookbook/                     Developer docs, user manual, SDK guide
```

## Architecture — 10 Microservices

```
                    ┌──────────────────────────────────────────┐
                    │         gateway-service (Go, :8080)       │
                    │  11-step middleware chain, multi-protocol │
                    │  OpenAI / Anthropic / Gemini / WebSocket  │
                    └──────┬──────┬──────┬──────┬──────────────┘
                           │gRPC  │gRPC  │gRPC  │gRPC
              ┌────────────┘      │      │      └────────────┐
              ▼                   ▼      ▼                   ▼
    ┌─────────────────┐  ┌────────────┐ ┌──────────────┐ ┌──────────────┐
    │ routing-service  │  │billing-svc │ │compliance-svc│ │  auth-svc    │
    │ (Go, :9001)      │  │(Go, :9070) │ │(Go+Py,:9090) │ │(Go, :9010)   │
    │ 4D scoring       │  │metering    │ │Guardrails    │ │19-role RBAC  │
    │ L1-L5 fallback   │  │anomaly det │ │ZDR + KMS     │ │Break-glass   │
    └────────┬─────────┘  └────────────┘ └──────────────┘ └──────────────┘
             │ RouteDecision{backend_id, fallback_chain}
             ▼
    ┌────────────────────────────────────────────────────────┐
    │          adapter-service (Python 3.12, :8000/:9002)    │
    │          LiteLLM — 100+ provider protocol translation   │
    │          KeyPool + CircuitBreaker + RetryEngine + Cache │
    └────────────────────────────────────────────────────────┘
             │                    │                     │
    ┌────────┘     ┌──────────────┘     ┌──────────────┘
    ▼              ▼                    ▼
  OpenAI      Anthropic      Gemini / DeepSeek / 通义千问 / 100+

  Supporting services (Kafka consumers & REST APIs):
  ┌─────────────────┐ ┌──────────────────┐ ┌──────────────────────┐
  │model-catalog-svc│ │llmops-trace-svc │ │prompt-eval-service   │
  │(Go, :8082:9030) │ │(Go, :8085)       │ │(Go+Py, :8086)        │
  │3-layer model    │ │44-field Trace    │ │A/B + LLM-as-Judge    │
  │vendor governance│ │ClickHouse        │ │quality gate          │
  └─────────────────┘ └──────────────────┘ └──────────────────────┘
  ┌──────────────────────┐
  │notification-service  │
  │(Go, :8089)           │
  │multi-channel + dedup │
  └──────────────────────┘
```

| # | Service | Language | HTTP | gRPC | Key Responsibility |
|---|---------|----------|------|------|-------------------|
| 1 | gateway-service | Go 1.22 | :8080 | — | Single external entry, 11-step middleware, multi-protocol |
| 2 | routing-service | Go 1.22 | — | :9001 | 4D scoring, 5 policy types, fallback L1-L5 |
| 3 | adapter-service | Python 3.12 | :8000 | :9002 | LiteLLM protocol translation, KeyPool, semantic cache |
| 4 | model-catalog-service | Go 1.22 | :8082 | :9030 | 3-layer model, vendor governance, marketplace |
| 5 | billing-service | Go 1.22 | :8084 | :9070 | Metering, 4-layer budget, anomaly detection |
| 6 | llmops-trace-service | Go 1.22 | :8085 | — | 44-field Trace, ClickHouse, Session view |
| 7 | prompt-eval-service | Go+Python | :8086 | — | A/B experiments, LLM-as-Judge, quality gate |
| 8 | auth-service | Go 1.22 | :8087 | :9010 | 19-role RBAC, SSO/SCIM, break-glass, approval workflow |
| 9 | compliance-service | Go+Python | :8088 | :9090 | Guardrails, ZDR, KMS, audit hash chain |
| 10 | notification-service | Go 1.22 | :8089 | — | Multi-channel notification, dedup, escalation |

Key 7 Kafka topics: `maas.api.requests` (12 partitions) | `maas.routing.decisions` | `maas.model.events` | `maas.billing.alerts` | `maas.anomaly.alerts` | `maas.compliance.events` | `maas.auth.events`

## Document Cross-Reference Map

When updating a document, check these related files for consistency:

| If editing... | Also check... |
|--------------|---------------|
| PRD §01 (roles/auth) | `08-auth-service详细设计.md`, `API接口设计文档.md` §2 |
| PRD §02 (model catalog) | `03-model-catalog-service详细设计.md`, `数据库设计文档.md` §3.3 |
| PRD §03 (routing) | `02-routing-service详细设计.md`, `技术架构设计文档.md` §4 |
| PRD §04 (LLMOps) | `06-llmops-trace-service详细设计.md` |
| PRD §05 (eval) | `07-prompt-eval-service详细设计.md` |
| PRD §06 (billing) | `05-billing-service详细设计.md`, `数据库设计文档.md` §3.4 |
| PRD §07 (compliance) | `09-compliance-service详细设计.md` |
| PRD §08 (private deploy) | `系统设计文档(HLD).md` §6-7 |
| PRD §09 (Console) | `console-frontend-prototype.html`, `console-原型设计线稿.md` |
| PRD §10 (Admin) | `admin-frontend-prototype.html`, `admin-原型设计线稿.md` |
| Microservice design doc | `架构全景图.md`, `技术架构设计文档.md`, `数据库设计文档.md` |
| Prototype HTML | Corresponding PRD §09/§10 + wireframe `.md` |

## Prototype Conventions

The HTML prototypes in `04-原型设计/` follow these patterns:

- **Tech**: Single-file HTML with Tailwind CSS CDN + Font Awesome 6.4 CDN — no build step
- **Navigation**: Sidebar with `data-target` attributes → JavaScript `navigate()` function toggles `.section.active`
- **Top-level nav items** (`.nav-item[data-target]`) and **sub-items** (`.nav-sub-item[data-target]`) both handled
- **Admin layout**: Sidebar + top header bar (`#headerTitle` set by JS) + `<main class="flex-1 overflow-y-auto p-6">` for scrollable content
- **Console layout**: Sidebar + main content with inner `max-w-5xl mx-auto px-6 py-6` container per section
- **Modals**: `.modal-mask` divs with `id`, toggled via `openModal(id)` / `closeModal(id)` JS helpers
- **Toast**: `showToast(msg)` helper, auto-dismisses after 1.8s
- **Inner tabs**: Buttons with `data-section` + `data-tab` attributes, `.inner-tab-panel` content divs
- **Section pattern**: `<div id="page-name" class="section">` — the `section` class sets `display:none`, `.active` sets `display:block`
- **NavTitles map**: JS `const navTitles = { targetId: '页面标题', ... }` — must include entries for every `data-target`
- When adding a new page, you must update: (1) sidebar nav item, (2) section div inside `<main>`, (3) navTitles map, (4) navigation JS if top-level item

## Key Terminology

| Chinese | English | Context |
|---------|---------|---------|
| 模型 | Model | LLM model (logical_model in DB) |
| 供应商/厂商 | Vendor/Provider | AI model supplier (OpenAI, Anthropic...) |
| 后端/后端实例 | Backend/VendorBackend | Specific deployment instance of a provider model |
| 租户 | Tenant | Enterprise customer organization |
| 项目 | Project | Tenant's workspace, contains API Keys + routing policies |
| 逻辑模型 | Logical Model | Tenant-facing model with capability tags (maas:gpt-4o) |
| 路由策略 | Routing Policy | 30+ field policy object, 5 types (COST_OPTIMAL/PERFORMANCE/FIXED/WEIGHTED/CANARY) |
| 四维评分 | 4D Scoring | W1×cost + W2×latency + W3×capability + W4×health |
| Fallback 链 | Fallback Chain | L1(main)→L2(same vendor)→L3(cross vendor, cap≥0.8)→L4(cap≥0.6)→L5(cache) |
| 预扣-核销 | Pre-deduct-Settle | Two-phase billing: estimate before request, settle after response |
| 数据分级 | Data Classification | L0(public)→L4(top secret), controls routing and storage |
| Guardrails | Guardrails | Content safety: keyword + PII regex + NER + semantic classifier + injection detection |
| ZDR | Zero Data Retention | Two modes: Metadata-Only / True Zero Retention |
| KMS/BYOK | Customer-managed keys | AWS KMS / Aliyun KMS / Vault for customer-controlled encryption |
| 审批工作流 | Approval Workflow | 12-state state machine, 3 risk tiers (P0/P1/P2), dual-approval for P0 |
| Break Glass | Break Glass | Emergency temporary privilege escalation with TTL + full audit |
| 熔断器 | Circuit Breaker | CLOSED→OPEN→HALF_OPEN 3-state pattern, per-backend |
| 语义缓存 | Semantic Cache | Embedding similarity cache (cosine≥0.92, bge-m3 model, Qdrant/Redis Vector) |

## Common Task Patterns

### Adding a new page to a prototype
1. Add nav item in sidebar (`.nav-item` with `data-target="page-id"`)
2. Add section div inside `<main>`: `<div id="page-id" class="section">...</div>`
3. Add entry to `navTitles` JS object: `'page-id':'页面标题'`
4. If top-level `.nav-item`, the generic handler will pick it up automatically
5. Match existing visual patterns: KPI cards use `grid grid-cols-4`, tables use `bg-white rounded-xl border overflow-hidden`

### Updating a microservice design doc
1. Check PRD V2.0 for the authoritative spec first
2. Check `架构全景图.md` and `技术架构设计文档.md` for cross-service consistency
3. Verify port numbers, gRPC service names, Kafka topic names against the canonical list
4. Update version number and changelog in the doc header

### Adding a new PRD section reference
1. PRD docs live in `01-产品设计/MaaS-PRD-V2.0/`
2. Each PRD doc has a standard header with version, status, and cross-references
3. Cross-reference format: `PRD §XX Y.Y` (e.g., `PRD §06 1.2.3`)
4. When a microservice doc references a PRD section, use the full path

### Cross-document consistency checks
- Service port numbers must match across `技术架构设计文档.md`, `架构全景图.md`, and each `*-service详细设计.md`
- Kafka topic names: 7 canonical topics with defined partition counts and replication factors
- Role names: underscore format (`platform_owner`, `tenant_admin`, `project_developer`) — no colons
- API Key types: 5 prefixes (`mk-prod-`, `mk-dev-`, `mk-ci-`, `mk-pg-`, `mk-sub-`)
- Model lifecycle states: `draft → testing → canary → active → paused → deprecated → retired`
- Approval states: 12-state machine defined in PRD §01 7.2

## Version and Update Tracking

All design documents have a header block with version, date, and base PRD reference. Current state:
- **PRD version**: V2.0 (12 specs, dated 2026-05-21)
- **Design doc version**: V2.0 (dated 2026-05-25, aligned with PRD V2.0)
- **Prototype version**: Updated 2026-05-25 (approval center + finetune added)
- **Source walkthroughs**: Updated 2026-05-25 (LiteLLM + Bifrost added)
- **Architecture overview**: Created 2026-05-25 (`架构全景图.md`, 12 Mermaid diagrams)

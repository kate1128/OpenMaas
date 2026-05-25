# AGENTS.md — OpenMaas

**Pure documentation repo.** No executable code, no build/test/lint/deploy tooling, no CI/CD.

## Structure

| Directory | Contents |
|---|---|
| `01-产品设计/` | Product design docs (PRD, user stories, req matrix, project plan, QA) |
| `02-竞品分析/` | Competitive analysis — 28 products (cloud vendors, open-source gateways, observability platforms) |
| `03-供应商分析/` | AI model supplier analysis (OpenAI, Anthropic, DeepSeek, Qwen, Llama, etc.) |
| `04-原型设计/` | HTML prototypes (admin/console/docs/public) + design guidelines |
| `05-开发设计/01-后端设计/` | Backend architecture: HLD, API spec, DB design, coding standards, 10 microservice designs |
| `05-开发设计/02-前端设计/` | Frontend design for console, admin, docs site |
| `06-产品运维/` | Operations: security, compliance, DR, runbook, capacity planning, deployment topology |
| `07-中间件/` | Middleware selection deep-dives (11 services: PostgreSQL, Redis, Kafka, ClickHouse, Qdrant, MinIO, Prometheus+Grafana, OTel+Jaeger, Vector, Vault, Nginx) |
| `08-源码走读/` | Code review of a LiteLLM fork (routero-develop) |
| `09-cookbook/` | Developer docs, user manual, SDK guide, product docs |

## Tech Stack (documented target architecture)

Go 1.22+ / Python 3.11+, React 18+ + Ant Design Pro, K8s 1.28+, Istio optional. Middleware stack in `07-中间件/00-中间件选型总览.md`.

## Notes

- All documentation is **Chinese**.
- `.gitignore` is a Go template placeholder — no Go code actually exists in this repo.
- The only non-markdown files are 4 HTML prototypes in `04-原型设计/`.
- This repo documents system design only — no implementation is present.

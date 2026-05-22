# HashiCorp Vault 选型与用法

**版本要求：** ≥ 1.16  
**角色：** 密钥管理、动态凭证、Transit 加密即服务

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| 动态凭证 | 为每个服务实例生成短生命周期数据库/Kafka 凭证，避免静态密码 |
| 租约自动轮转 | 凭证到期前自动续约，服务无感知 |
| Transit Engine | 加密即服务，compliance-service 用于审计日志哈希加密 |
| KMS 集成 | 可对接 AWS KMS / 阿里云 KMS，私有部署用 Vault 自身 |
| 审计日志 | 所有 Vault 操作记录，满足等保 2.0 操作审计要求 |

---

## 二、Secret Engine 规划

| Engine | 挂载路径 | 用途 |
|--------|---------|------|
| KV v2 | `secret/maas/` | 静态配置（第三方 API Key、OAuth 凭证） |
| Database | `database/maas/` | PostgreSQL 动态凭证 |
| PKI | `pki/maas/` | 内部服务 mTLS 证书签发 |
| Transit | `transit/maas/` | 数据加密/解密（审计日志哈希） |
| AWS | `aws/maas/` | 动态 AWS IAM 凭证（访问 S3/KMS） |

---

## 三、动态数据库凭证（PostgreSQL）

```hcl
# 配置 Database Engine
vault write database/config/maas-pg \
    plugin_name=postgresql-database-plugin \
    allowed_roles="maas-*" \
    connection_url="postgresql://{{username}}:{{password}}@pg-primary:5432/postgres" \
    username="vault_admin" \
    password="..."

# 创建角色（billing-service 专用，只读 billing_ledger）
vault write database/roles/maas-billing-service \
    db_name=maas-pg \
    creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}'; GRANT SELECT, INSERT ON billing_ledger TO \"{{name}}\";" \
    default_ttl="1h" \
    max_ttl="24h"
```

**Go 侧读取动态凭证（vault-sdk）：**

```go
import vaultapi "github.com/hashicorp/vault/api"

client, _ := vaultapi.NewClient(vaultapi.DefaultConfig())
client.SetToken(os.Getenv("VAULT_TOKEN"))  // 或 K8s Auth

secret, _ := client.Logical().Read("database/creds/maas-billing-service")
username := secret.Data["username"].(string)
password := secret.Data["password"].(string)
```

---

## 四、Transit Engine（加密即服务）

```go
// compliance-service 加密审计日志哈希
func encryptHash(hash string) (string, error) {
    encoded := base64.StdEncoding.EncodeToString([]byte(hash))
    secret, err := vaultClient.Logical().Write(
        "transit/maas/encrypt/audit-log-key",
        map[string]interface{}{"plaintext": encoded},
    )
    if err != nil {
        return "", err
    }
    return secret.Data["ciphertext"].(string), nil
}

// 解密
func decryptHash(ciphertext string) (string, error) {
    secret, err := vaultClient.Logical().Write(
        "transit/maas/decrypt/audit-log-key",
        map[string]interface{}{"ciphertext": ciphertext},
    )
    decoded, _ := base64.StdEncoding.DecodeString(secret.Data["plaintext"].(string))
    return string(decoded), err
}
```

---

## 五、Kubernetes Auth（推荐生产方式）

```hcl
# 无需静态 VAULT_TOKEN，Pod 用 K8s ServiceAccount 换取 Vault Token
vault auth enable kubernetes

vault write auth/kubernetes/config \
    kubernetes_host="https://kubernetes.default.svc" \
    kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt

vault write auth/kubernetes/role/billing-service \
    bound_service_account_names=billing-service \
    bound_service_account_namespaces=maas \
    policies=maas-billing-policy \
    ttl=1h
```

---

## 六、KV 静态密钥存取规范

```bash
# 写入
vault kv put secret/maas/openai api_key="sk-xxx"

# 读取（Go）
secret, _ := client.KVv2("secret").Get(ctx, "maas/openai")
apiKey := secret.Data["api_key"].(string)
```

> **禁止**将密钥写入环境变量、配置文件、代码仓库。所有密钥统一通过 Vault 读取。

---

## 七、部署规格

| 参数 | 值 |
|------|---|
| 模式 | HA 模式（3 节点 Raft 存储） |
| 端口 | 8200（API + UI） |
| 自动解封 | Vault Auto-Unseal with AWS KMS 或 阿里云 KMS |
| 备份 | 每日 `vault operator raft snapshot save` |

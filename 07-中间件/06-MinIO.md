# MinIO 选型与用法

**版本要求：** ≥ RELEASE.2024-01-01（S3 API 兼容）  
**角色：** 对象存储（冷数据归档、评测数据集、合规报告）

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| S3 兼容 API | 代码层面与 AWS S3 完全兼容，公有云可无缝切换 |
| 私有化部署 | 支持完全离线部署，满足数据合规要求 |
| 纠删码 | 4+2 纠删码，2 个节点故障不丢数据 |
| 生命周期策略 | 自动转换存储类或过期删除 |
| 服务端加密 | SSE-S3 / SSE-KMS 支持，与 Vault 集成 |

---

## 二、Bucket 规划

| Bucket | 用途 | 加密 | 生命周期 |
|--------|------|------|---------|
| `maas-trace-archive` | llmops-trace-service 冷归档（Parquet） | SSE-KMS | 180天后删除 |
| `maas-eval-datasets` | prompt-eval-service 评测数据集 | SSE-S3 | 永久保留 |
| `maas-compliance-reports` | compliance-service 合规报告 | SSE-KMS | 7年（等保要求） |
| `maas-audit-logs` | 审计日志归档 | SSE-KMS | 7年 |
| `maas-model-artifacts` | 模型文件、微调产物 | SSE-S3 | 永久保留 |

---

## 三、Go SDK 使用（minio-go）

```go
import "github.com/minio/minio-go/v7"

client, _ := minio.New("minio:9000", &minio.Options{
    Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
    Secure: true,
})

// 上传文件
client.PutObject(ctx, "maas-eval-datasets",
    fmt.Sprintf("datasets/%s/%s.jsonl", tenantID, datasetID),
    reader, fileSize,
    minio.PutObjectOptions{
        ContentType: "application/jsonl",
        UserMetadata: map[string]string{
            "x-maas-tenant-id": tenantID,
        },
    },
)

// 生成预签名下载 URL（1小时有效）
url, _ := client.PresignedGetObject(ctx, "maas-eval-datasets",
    objectName, time.Hour, nil)
```

---

## 四、生命周期配置

```xml
<!-- 通过 mc 或 API 设置 Bucket 生命周期 -->
<LifecycleConfiguration>
  <Rule>
    <ID>trace-archive-expiry</ID>
    <Filter><Prefix>traces/</Prefix></Filter>
    <Status>Enabled</Status>
    <Expiration><Days>180</Days></Expiration>
  </Rule>
  <Rule>
    <ID>compliance-transition</ID>
    <Filter><Prefix>compliance/</Prefix></Filter>
    <Status>Enabled</Status>
    <Transition>
      <Days>90</Days>
      <StorageClass>GLACIER</StorageClass>
    </Transition>
  </Rule>
</LifecycleConfiguration>
```

---

## 五、SSE-KMS 与 Vault 集成

```bash
# MinIO 配置使用外部 KMS（Vault Transit Engine）
MINIO_KMS_KES_ENDPOINT=https://kes:7373
MINIO_KMS_KES_KEY_NAME=maas-minio-key
MINIO_KMS_KES_CERT_FILE=/etc/kes/client.crt
MINIO_KMS_KES_KEY_FILE=/etc/kes/client.key
```

---

## 六、部署规格

| 参数 | 值 |
|------|---|
| 节点数 | 4（生产纠删码模式） |
| 每节点磁盘 | 4 × HDD 各 4TB |
| API 端口 | 9000 |
| Console 端口 | 9001 |
| 网络带宽 | ≥ 10Gbps（节点间） |

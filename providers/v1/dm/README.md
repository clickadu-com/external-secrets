# Провайдер Domain Manager (DM)

Провайдер **Domain Manager (DM)** интегрирует управление SSL-сертификатами Clickadu в Kubernetes через External Secrets Operator.

## Настройка Store

Для использования провайдера необходимо создать `SecretStore` или `ClusterSecretStore` с указанием URL API и токена доступа.

### Создание Secret с токеном
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dm-token
type: Opaque
stringData:
  token: "ваш_api_токен"
```

### SecretStore
```yaml
apiVersion: external-secrets.io/v1
kind: SecretStore
metadata:
  name: dm-store
spec:
  provider:
    dm:
      url: "https://dm.clickadu.com" # URL вашего Domain Manager
      auth:
        secretRef:
          token:
            name: dm-token
            key: token
```

---

## Использование в ExternalSecret

### 1. Режим Чтения (`data`)
Используется для получения полей **существующих** сертификатов.

**Внимание**: Поле `property` является **обязательным**.

| Поле `property` | Описание |
|-------|----------|
| `bundle` | Fullchain (сертификат + CA) |
| `cert` | Только тело сертификата |
| `ca` | Только CA (root/intermediate) |
| `key` | Приватный ключ |

#### Примеры (data):
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: dm-certs
spec:
  secretStoreRef:
    name: dm-store
    kind: SecretStore
  target:
    name: my-tls-secret
  data:
    # Получение Fullchain по имени (RSA)
    - secretKey: tls.crt
      remoteRef:
        key: rsa/name/example.com
        property: bundle

    # Получение ключа по имени (RSA)
    - secretKey: tls.key
      remoteRef:
        key: rsa/name/example.com
        property: key

    # Получение по ID сертификата (ECDSA)
    - secretKey: cert-only.pem
      remoteRef:
        key: ecdsa/id/12345
        property: cert
```

### 2. Режим Авто-выпуска (`dataFrom`)
Используется для **автоматического создания** сертификатов, если они еще не существуют. Провайдер сам найдет домен и закажет сертификат.

- **key**: `rsa/name/<domain>` или `ecdsa/name/<domain>`
- **property**: список SAN (субдоменов) через запятую.
- **Результат**: всегда создает два ключа в K8s Secret: `bundle` и `key`.

#### Примеры (dataFrom):
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: dm-auto-tls
spec:
  secretStoreRef:
    name: dm-store
    kind: SecretStore
  target:
    name: auto-tls-secret
    template:
      type: kubernetes.io/tls
      data:
        tls.crt: "{{ .bundle }}"
        tls.key: "{{ .key }}"
  dataFrom:
    # Если сертификата для my-site.com нет — он будет создан.
    # Будут добавлены SAN: www.my-site.com и api.my-site.com
    - extract:
        key: rsa/name/my-site.com
        property: "www,api"
```

---

## Форматы ключей (Key)
* `rsa/name/<host>` — Поиск или Создание RSA сертификата.
* `ecdsa/name/<host>` — Поиск или Создание ECDSA сертификата.
* `rsa/id/<id>` — Поиск по ID (только чтение).
* `ecdsa/id/<id>` — Поиск по ID (только чтение).

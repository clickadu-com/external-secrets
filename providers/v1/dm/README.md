# Провайдер Domain Manager (DM)

Провайдер **Domain Manager (DM)** позволяет интегрировать систему управления SSL-сертификатами Clickadu в Kubernetes через External Secrets Operator (ESO).

## Возможности
*   **Получение существующих сертификатов**: Чтение по имени или ID.
*   **Автоматический выпуск**: Создание новых сертификатов прямо из ESO, если они отсутствуют в DM.
*   **Гибкая конфигурация**: Настройка провайдера (`providerName`), типа движка (`providerType`) и данных субъекта (`subject`).

## Настройка SecretStore

Для работы провайдера требуется URL API и токен доступа.

### 1. Создание Secret с токеном
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dm-token
type: Opaque
stringData:
  token: "ваш_api_токен"
```

### 2. Конфигурация SecretStore
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

### Параметры выпуска сертификата (`generator`)

Если сертификат не найден в базе DM по доменному имени, провайдер попытается создать его, используя параметры из поля `generator`.

| Поле | JSON-тег | Тип | Описание |
|------|----------|-----|----------|
| `ProviderName` | `providerName` | `string` | Имя провайдера в DM (напр. `LE_PROD`, `ZERO_SSL`) |
| `ProviderType` | `providerType` | `string` | Тип движка (`LETSENCRYPT`, `ZEROSSL`, `SELF`) |
| `Sync` | `sync` | `bool` | Синхронизировать сертификат немедленно (по умолчанию `true`) |
| `DNSNames` | `dnsNames` | `[]string` | Список доп. имен (SAN) |
| `IPAddresses` | `ipAddresses` | `[]string` | Список IP адресов |
| `Subject` | `subject` | `object` | Данные субъекта (Country, Organization и др.) |

### 1. Получение отдельных полей (`data`)

Используйте поле `property` для выбора конкретной части сертификата.

| Поле `property` | Описание |
|-----------------|----------|
| `bundle` | Fullchain (сертификат + CA) |
| `cert` | Только тело сертификата |
| `ca` | Только CA (root/intermediate) |
| `key` | Приватный ключ |

#### Пример:
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: example-com-tls
spec:
  secretStoreRef:
    name: dm-store
  data:
    - secretKey: tls.crt
      remoteRef:
        key: rsa/name/example.com
        property: bundle
        generator:
          providerName: "LE_PROD"
          providerType: "LETSENCRYPT"
          dnsNames: ["www", "api"]
```

### 2. Автоматический выпуск всех полей (`dataFrom`)

В режиме `dataFrom` (через `extract`) провайдер по умолчанию возвращает карту с двумя ключами: `bundle` и `key`. Если указано поле `property`, будет возвращено только выбранное поле.

#### Пример (стандартный TLS секрет):
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: mysite-tls
spec:
  secretStoreRef:
    name: dm-store
  target:
    name: mysite-tls
    template:
      type: kubernetes.io/tls
      data:
        tls.crt: "{{ .bundle }}"
        tls.key: "{{ .key }}"
  dataFrom:
    - extract:
        key: rsa/name/mysite.com
        generator:
          providerType: "ZEROSSL"
          dnsNames: ["www"]
```

#### Пример (только одно поле):
```yaml
  dataFrom:
    - extract:
        key: rsa/name/mysite.com
        property: cert # Вернет только cert
```

---

## Форматы ключей (Key)

*   `rsa/name/<host>` — Поиск или создание RSA сертификата.
*   `ecdsa/name/<host>` — Поиск или создание ECDSA сертификата.
*   `rsa/id/<id>` — Только чтение по ID.

# Провайдер Domain Manager (DM)

Провайдер **Domain Manager (DM)** позволяет интегрировать систему управления SSL-сертификатами Clickadu в Kubernetes через External Secrets Operator (ESO).

## Возможности
*   **Получение существующих сертификатов**: Чтение по имени или ID.
*   **Автоматический выпуск**: Создание новых сертификатов прямо из ESO, если они отсутствуют в DM.
*   **Гибкая конфигурация**: Настройка провайдера (`providerName`), типа движка (`providerType`) и Common Name (`commonName`) через поле `generator`.

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

Если сертификат не найден в базе DM по доменному имени, провайдер попытается создать его, используя параметры из поля `generator`. **Внимание: выпуск сертификата возможен только при наличии заполненного поля `generator`.** Если сертификат не найден и поле `generator` не задано, провайдер вернет ошибку "Secret not found".

| Поле | JSON-тег | Тип | Описание |
|------|----------|-----|----------|
| `ProviderName` | `providerName` | `string` | Имя провайдера в DM (напр. `LE_PROD`, `ZERO_SSL`) |
| `ProviderType` | `providerType` | `string` | Тип движка (`acme`, `ca`) |
| `Sync` | `sync` | `bool` | Синхронизировать сертификат немедленно (по умолчанию `true`) |
| `Subject` | `subject` | `object` | Данные субъекта (содержит `commonName`) |
| `DNSNames` | `dnsNames` | `[]string` | Список доп. имен (SAN) |
| `IPAddresses` | `ipAddresses` | `[]string` | Список IP адресов |

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
          providerName: "google-http"
          providerType: "acme"
          subject:
            commonName: "example.com"
          dnsNames: ["www", "api"]
```

### 2. Автоматический выпуск всех полей (`dataFrom`)

В режиме `dataFrom` (через `extract`) провайдер по умолчанию возвращает карту со всеми доступными полями: `bundle`, `cert`, `ca` и `key`. Если указано поле `property`, будет возвращено только выбранное поле.

#### Пример:
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
      engineVersion: v2
      type: kubernetes.io/tls
      data:
        ca.crt: "{{ .ca }}"
        tls.crt: "{{ .bundle }}"
        tls.key: "{{ .key }}"
  dataFrom:
    - extract:
        key: rsa/name/mysite.com
        generator:
          providerType: "acme"
          subject:
            commonName: "mysite.com"
          dnsNames: ["www"]
```

### 3. Работа с несколькими сертификатами (`rewrite`)

Если в одном `ExternalSecret` нужно объединить несколько сертификатов, используйте механизм `rewrite`, чтобы избежать коллизии ключей (`ca`, `key` и т.д.).

#### Пример:
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: multi-cert-secret
spec:
  secretStoreRef:
    name: dm-store
  dataFrom:
    - extract:
        key: ecdsa/name/auth-server
      rewrite:
        - regexp:
            source: "(.*)"
            target: "auth_$1" # Ключи станут auth_ca, auth_bundle и т.д.
    - extract:
        key: ecdsa/name/vpn-server
      rewrite:
        - regexp:
            source: "(.*)"
            target: "vpn_$1" # Ключи станут vpn_ca, vpn_bundle и т.д.
  target:
    template:
      engineVersion: v2
      data:
        auth-crt: "{{ .auth_bundle }}"
        vpn-crt:  "{{ .vpn_bundle }}"
        combined-ca: "{{ .auth_ca }}\n{{ .vpn_ca }}"
```

---

## Форматы ключей (Key)

*   `rsa/name/<host>` — Поиск или создание RSA сертификата.
*   `ecdsa/name/<host>` — Поиск или создание ECDSA сертификата.
*   `rsa/id/<id>` — Только чтение по ID.
